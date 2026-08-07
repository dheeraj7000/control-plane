package gateway

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/dheeraj7000/control-plane/internal/agent"
	"github.com/dheeraj7000/control-plane/internal/execution"
	"github.com/dheeraj7000/control-plane/internal/workflow"
)

// Mount registers every Gateway route on r. Agent registration is
// deliberately NOT behind authRepo's AuthMiddleware — bootstrapping the
// very first agent would otherwise be circular. This is a known,
// flagged gap (see docs/architecture.md): a real deployment needs
// internal/auth (control-plane operator authentication, still
// unbuilt) gating who may register agents at all, not open
// registration. Everything else requires a valid agent bearer token.
//
// wsAllowedOrigins is passed straight through to handleEventsWS — see
// its doc comment for why the WebSocket route needs its own Origin
// allowlist distinct from the CORS middleware every other route gets.
// Passing the same value as CORSAllowedOrigins is the expected/only
// use today, but Mount takes it as an explicit parameter rather than
// reaching for a package-level config to keep this package's only
// dependency on configuration explicit at the call site.
func Mount(r chi.Router, svc *Service, agents agent.Repository, limiter *RateLimiter, wsAllowedOrigins []string) {
	r.Post("/agents", handleRegisterAgent(svc))

	r.Group(func(r chi.Router) {
		r.Use(AuthMiddleware(agents))
		if limiter != nil {
			r.Use(limiter.Middleware)
		}

		r.Get("/agents/{id}", handleGetAgent(svc))
		r.Get("/agents", handleListAgents(svc))

		r.Post("/workflows", handleRegisterWorkflow(svc))
		r.Get("/workflows/{id}", handleGetWorkflow(svc))
		r.Get("/workflows", handleListWorkflows(svc))

		r.Post("/executions", handleStartExecution(svc))
		r.Get("/executions/{id}", handleGetExecution(svc))
		r.Get("/executions", handleListExecutions(svc))
		r.Get("/executions/{id}/timeline", handleGetTimeline(svc))
		r.Get("/executions/{id}/events", handleGetEvents(svc))
		r.Get("/executions/{id}/budget", handleGetExecutionBudget(svc))
	})

	// The WebSocket route gets its own auth middleware (query-param
	// token fallback) instead of joining the group above — see
	// AuthMiddlewareWS's doc comment for why.
	r.Group(func(r chi.Router) {
		r.Use(AuthMiddlewareWS(agents))
		r.Get("/executions/{id}/ws", handleEventsWS(svc, wsAllowedOrigins))
	})
}

// --- agents ---

type registerAgentRequest struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	AllowedTools []string `json:"allowed_tools,omitempty"`
}

type registerAgentResponse struct {
	Agent agentView `json:"agent"`
	Token string    `json:"token"`
}

type agentView struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	AllowedTools []string `json:"allowed_tools,omitempty"`
}

func toAgentView(a agent.Agent) agentView {
	return agentView{ID: a.ID(), Name: a.Name(), AllowedTools: a.AllowedTools()}
}

func handleRegisterAgent(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req registerAgentRequest
		if !decodeJSON(w, r, &req) {
			return
		}
		a, token, err := svc.RegisterAgent(r.Context(), req.ID, req.Name, req.AllowedTools)
		if err != nil {
			writeServiceError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, registerAgentResponse{Agent: toAgentView(a), Token: token})
	}
}

func handleGetAgent(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		a, err := svc.GetAgent(r.Context(), chi.URLParam(r, "id"))
		if err != nil {
			writeServiceError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, toAgentView(a))
	}
}

func handleListAgents(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		list, err := svc.ListAgents(r.Context())
		if err != nil {
			writeServiceError(w, err)
			return
		}
		views := make([]agentView, len(list))
		for i, a := range list {
			views[i] = toAgentView(a)
		}
		writeJSON(w, http.StatusOK, views)
	}
}

// --- workflows ---

func handleRegisterWorkflow(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var wf workflow.Workflow
		if !decodeJSON(w, r, &wf) {
			return
		}
		if err := svc.RegisterWorkflow(r.Context(), wf); err != nil {
			writeServiceError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, wf)
	}
}

func handleGetWorkflow(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		wf, err := svc.GetLatestWorkflow(r.Context(), chi.URLParam(r, "id"))
		if err != nil {
			writeServiceError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, wf)
	}
}

func handleListWorkflows(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		list, err := svc.ListWorkflows(r.Context())
		if err != nil {
			writeServiceError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, list)
	}
}

// --- executions ---

type startExecutionRequest struct {
	WorkflowID string `json:"workflow_id"`
}

func handleStartExecution(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req startExecutionRequest
		if !decodeJSON(w, r, &req) {
			return
		}
		agentID := ""
		if a, ok := AgentFromContext(r.Context()); ok {
			agentID = a.ID()
		}
		exec, err := svc.StartExecution(r.Context(), req.WorkflowID, agentID)
		if err != nil {
			writeServiceError(w, err)
			return
		}
		writeJSON(w, http.StatusAccepted, execView(exec))
	}
}

func handleGetExecution(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		exec, err := svc.GetExecution(r.Context(), chi.URLParam(r, "id"))
		if err != nil {
			writeServiceError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, execView(exec))
	}
}

func handleListExecutions(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		filter := execution.ListFilter{
			WorkflowID: r.URL.Query().Get("workflow_id"),
			State:      execution.State(r.URL.Query().Get("state")),
		}
		list, err := svc.ListExecutions(r.Context(), filter)
		if err != nil {
			writeServiceError(w, err)
			return
		}
		views := make([]executionView, len(list))
		for i, e := range list {
			views[i] = execView(e)
		}
		writeJSON(w, http.StatusOK, views)
	}
}

func handleGetTimeline(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		entries, err := svc.GetTimeline(r.Context(), chi.URLParam(r, "id"))
		if err != nil {
			writeServiceError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, entries)
	}
}

func handleGetEvents(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		evts, err := svc.GetEvents(r.Context(), chi.URLParam(r, "id"))
		if err != nil {
			writeServiceError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, evts)
	}
}

// budgetView is the wire shape for GET .../executions/{id}/budget —
// hand-picked the same way executionView/agentView are, rather than
// exposing budget.Ledger's internals directly.
type budgetView struct {
	Scope             string  `json:"scope"`
	OwnerID           string  `json:"owner_id"`
	InputTokens       int64   `json:"input_tokens"`
	OutputTokens      int64   `json:"output_tokens"`
	CostUSD           float64 `json:"cost_usd"`
	LimitInputTokens  int64   `json:"limit_input_tokens,omitempty"`
	LimitOutputTokens int64   `json:"limit_output_tokens,omitempty"`
	LimitCostUSD      float64 `json:"limit_cost_usd,omitempty"`
	Exceeded          bool    `json:"exceeded"`
}

func handleGetExecutionBudget(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ledger, err := svc.GetExecutionBudget(r.Context(), chi.URLParam(r, "id"))
		if err != nil {
			writeServiceError(w, err)
			return
		}
		usage, limit := ledger.Usage(), ledger.Limit()
		writeJSON(w, http.StatusOK, budgetView{
			Scope:             string(ledger.Scope()),
			OwnerID:           ledger.OwnerID(),
			InputTokens:       usage.InputTokens,
			OutputTokens:      usage.OutputTokens,
			CostUSD:           usage.Cost.USD(),
			LimitInputTokens:  limit.InputTokens,
			LimitOutputTokens: limit.OutputTokens,
			LimitCostUSD:      limit.Cost.USD(),
			Exceeded:          ledger.Exceeded(),
		})
	}
}

type executionView struct {
	ID              string `json:"id"`
	WorkflowID      string `json:"workflow_id"`
	WorkflowVersion int    `json:"workflow_version"`
	AgentID         string `json:"agent_id,omitempty"`
	State           string `json:"state"`
}

func execView(e *execution.Execution) executionView {
	return executionView{
		ID:              e.ID(),
		WorkflowID:      e.WorkflowID(),
		WorkflowVersion: e.WorkflowVersion(),
		AgentID:         e.AgentID(),
		State:           string(e.State()),
	}
}

// --- shared helpers ---

func decodeJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		writeError(w, http.StatusBadRequest, "malformed request body: "+err.Error())
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

type errorResponse struct {
	Error string `json:"error"`
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, errorResponse{Error: message})
}

// writeServiceError maps a Service error to an HTTP status. This is a
// coarse mapping (the repository sentinel errors this package knows
// about all being tested with errors.Is) rather than a rich
// problem-details scheme — sufficient for this milestone, worth
// revisiting if API consumers need more structured error bodies.
func writeServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, workflow.ErrNotFound), errors.Is(err, execution.ErrNotFound), errors.Is(err, agent.ErrNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, workflow.ErrAlreadyExists), errors.Is(err, agent.ErrAlreadyExists):
		writeError(w, http.StatusConflict, err.Error())
	default:
		writeError(w, http.StatusBadRequest, err.Error())
	}
}
