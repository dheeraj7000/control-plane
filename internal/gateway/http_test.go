package gateway_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/dheeraj7000/control-plane/internal/adapters"
	"github.com/dheeraj7000/control-plane/internal/agent"
	"github.com/dheeraj7000/control-plane/internal/budget"
	"github.com/dheeraj7000/control-plane/internal/events"
	"github.com/dheeraj7000/control-plane/internal/execution"
	"github.com/dheeraj7000/control-plane/internal/gateway"
	"github.com/dheeraj7000/control-plane/internal/policy"
	"github.com/dheeraj7000/control-plane/internal/workflow"
)

// testServer bundles a full Mount()-ed router with its Agent
// repository, so tests can register an agent and use its token.
type testServer struct {
	router chi.Router
	agents agent.Repository
	svc    *gateway.Service
}

func newTestServer(t *testing.T) *testServer {
	t.Helper()
	agents := newAgentRepoForWS(t)
	svc := newTestServiceWithAgents(t, agents)
	return &testServer{router: newMountedRouter(svc, agents), agents: agents, svc: svc}
}

// newAgentRepoForWS is just agent.NewInMemoryRepository() under a name
// that reads sensibly from ws_test.go too — both files share these
// helpers since they're testing the same Mount()-ed router from
// different angles (plain HTTP vs. WebSocket).
func newAgentRepoForWS(t *testing.T) agent.Repository {
	t.Helper()
	return agent.NewInMemoryRepository()
}

func newMountedRouter(svc *gateway.Service, agents agent.Repository) chi.Router {
	return newMountedRouterWithOrigins(svc, agents, []string{"*"})
}

func newMountedRouterWithOrigins(svc *gateway.Service, agents agent.Repository, wsAllowedOrigins []string) chi.Router {
	r := chi.NewRouter()
	gateway.Mount(r, svc, agents, nil, wsAllowedOrigins)
	return r
}

func newTestServiceWithAgents(t *testing.T, agents agent.Repository) *gateway.Service {
	t.Helper()
	return newTestServiceWithAgentsAndAdapters(t, agents, &fakeToolAdapter{}, &fakeModelAdapter{})
}

func newTestServiceWithAgentsAndAdapters(t *testing.T, agents agent.Repository, toolAdapter adapters.Adapter, modelAdapter adapters.ModelAdapter) *gateway.Service {
	t.Helper()
	engine, err := policy.NewNativeEngine(policy.EffectAllow)
	if err != nil {
		t.Fatalf("policy.NewNativeEngine() returned error: %v", err)
	}
	svc, err := gateway.NewService(gateway.ServiceConfig{
		Workflows:    workflow.NewInMemoryRepository(),
		Executions:   execution.NewInMemoryRepository(),
		Agents:       agents,
		Budgets:      budget.NewInMemoryRepository(),
		Events:       events.NewRecorder(events.NewInMemoryStore(), events.NewInMemoryBus()),
		PolicyEngine: engine,
		ToolAdapter:  toolAdapter,
		ModelAdapter: modelAdapter,
		Logger:       silentLogger(),
	})
	if err != nil {
		t.Fatalf("NewService() returned error: %v", err)
	}
	return svc
}

func (ts *testServer) registerAgent(t *testing.T, id, name string) string {
	t.Helper()
	_, token, err := ts.svc.RegisterAgent(context.Background(), id, name, nil)
	if err != nil {
		t.Fatalf("RegisterAgent() returned error: %v", err)
	}
	return token
}

func (ts *testServer) do(t *testing.T, method, path, token string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("json.Marshal() returned error: %v", err)
		}
		reader = bytes.NewReader(b)
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequestWithContext(context.Background(), method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	ts.router.ServeHTTP(rec, req)
	return rec
}

func TestHTTP_RegisterAgent_NoAuthRequired(t *testing.T) {
	ts := newTestServer(t)
	rec := ts.do(t, http.MethodPost, "/agents", "", map[string]any{"id": "agent-1", "name": "Bot"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201, body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["token"] == "" || resp["token"] == nil {
		t.Error("response missing a non-empty token")
	}
}

func TestHTTP_ProtectedRoute_RequiresAuth(t *testing.T) {
	ts := newTestServer(t)
	rec := ts.do(t, http.MethodGet, "/workflows", "", nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestHTTP_ProtectedRoute_RejectsBadToken(t *testing.T) {
	ts := newTestServer(t)
	rec := ts.do(t, http.MethodGet, "/workflows", "not-a-real-token", nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestHTTP_WorkflowLifecycle(t *testing.T) {
	ts := newTestServer(t)
	token := ts.registerAgent(t, "agent-1", "Bot")

	wfBody := map[string]any{
		"id": "wf-1", "name": "Test", "version": 1,
		"steps": []map[string]any{{"id": "a", "type": "review"}},
	}
	rec := ts.do(t, http.MethodPost, "/workflows", token, wfBody)
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST /workflows status = %d, want 201, body=%s", rec.Code, rec.Body.String())
	}

	rec = ts.do(t, http.MethodGet, "/workflows/wf-1", token, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /workflows/wf-1 status = %d, want 200", rec.Code)
	}

	rec = ts.do(t, http.MethodGet, "/workflows", token, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /workflows status = %d, want 200", rec.Code)
	}
	var list []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("workflow list len = %d, want 1", len(list))
	}
}

func TestHTTP_ExecutionLifecycle(t *testing.T) {
	ts := newTestServer(t)
	token := ts.registerAgent(t, "agent-1", "Bot")

	wfBody := map[string]any{
		"id": "wf-1", "name": "Test", "version": 1,
		"steps": []map[string]any{{"id": "a", "type": "review"}},
	}
	if rec := ts.do(t, http.MethodPost, "/workflows", token, wfBody); rec.Code != http.StatusCreated {
		t.Fatalf("POST /workflows status = %d, body=%s", rec.Code, rec.Body.String())
	}

	rec := ts.do(t, http.MethodPost, "/executions", token, map[string]any{"workflow_id": "wf-1"})
	if rec.Code != http.StatusAccepted {
		t.Fatalf("POST /executions status = %d, want 202, body=%s", rec.Code, rec.Body.String())
	}
	var execResp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &execResp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	execID, _ := execResp["id"].(string)
	if execID == "" {
		t.Fatalf("response missing execution id: %s", rec.Body.String())
	}
	if execResp["agent_id"] != "agent-1" {
		t.Errorf("agent_id = %v, want agent-1 (from the authenticated request)", execResp["agent_id"])
	}

	rec = ts.do(t, http.MethodGet, "/executions/"+execID, token, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /executions/{id} status = %d, want 200", rec.Code)
	}

	rec = ts.do(t, http.MethodGet, "/executions/"+execID+"/events", token, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET .../events status = %d, want 200", rec.Code)
	}

	rec = ts.do(t, http.MethodGet, "/executions/"+execID+"/timeline", token, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET .../timeline status = %d, want 200", rec.Code)
	}

	rec = ts.do(t, http.MethodGet, "/executions/"+execID+"/budget", token, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET .../budget status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	var budgetResp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &budgetResp); err != nil {
		t.Fatalf("decode budget response: %v", err)
	}
	if budgetResp["scope"] != "execution" || budgetResp["owner_id"] != execID {
		t.Errorf("budget response = %v, want scope=execution owner_id=%s", budgetResp, execID)
	}
}

func TestHTTP_GetExecutionBudget_UnknownExecutionNotFound(t *testing.T) {
	ts := newTestServer(t)
	token := ts.registerAgent(t, "agent-1", "Bot")
	rec := ts.do(t, http.MethodGet, "/executions/ghost/budget", token, nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404, body=%s", rec.Code, rec.Body.String())
	}
}

func TestHTTP_GetExecution_NotFound(t *testing.T) {
	ts := newTestServer(t)
	token := ts.registerAgent(t, "agent-1", "Bot")
	rec := ts.do(t, http.MethodGet, "/executions/ghost", token, nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestHTTP_RegisterWorkflow_MalformedBody(t *testing.T) {
	ts := newTestServer(t)
	token := ts.registerAgent(t, "agent-1", "Bot")

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/workflows", bytes.NewReader([]byte("not json")))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	ts.router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestHTTP_RegisterWorkflow_DuplicateConflict(t *testing.T) {
	ts := newTestServer(t)
	token := ts.registerAgent(t, "agent-1", "Bot")
	wfBody := map[string]any{
		"id": "wf-1", "name": "Test", "version": 1,
		"steps": []map[string]any{{"id": "a", "type": "review"}},
	}
	if rec := ts.do(t, http.MethodPost, "/workflows", token, wfBody); rec.Code != http.StatusCreated {
		t.Fatalf("first POST status = %d, body=%s", rec.Code, rec.Body.String())
	}
	rec := ts.do(t, http.MethodPost, "/workflows", token, wfBody)
	if rec.Code != http.StatusConflict {
		t.Fatalf("second POST status = %d, want 409, body=%s", rec.Code, rec.Body.String())
	}
}
