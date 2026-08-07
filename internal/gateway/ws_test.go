package gateway_test

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	"github.com/dheeraj7000/control-plane/internal/adapters"
	"github.com/dheeraj7000/control-plane/internal/events"
	"github.com/dheeraj7000/control-plane/internal/workflow"
)

// TestWS_StreamsExecutionEvents uses a deliberately slow tool adapter
// so the test has a real window to dial the WebSocket before the
// execution finishes — events.Bus has no history for a late subscriber
// (a documented, deliberate Milestone 3 design choice: Bus is live-only,
// Store is where replay comes from), so a WS client that connects after
// an already-fast execution completes would legitimately see nothing.
func TestWS_StreamsExecutionEvents(t *testing.T) {
	agents := newAgentRepoForWS(t)
	tool := &fakeToolAdapter{
		result: adapters.ToolCallResult{},
		delay:  150 * time.Millisecond,
	}
	svc := newTestServiceWithAgentsAndAdapters(t, agents, tool, &fakeModelAdapter{})

	wf, err := workflow.New("wf-1", "Test", 1, []workflow.Step{
		{ID: "a", Type: workflow.StepTypeCallTool, Config: map[string]any{"tool": "slow.tool"}},
	})
	if err != nil {
		t.Fatalf("workflow.New() returned error: %v", err)
	}
	if err := svc.RegisterWorkflow(context.Background(), wf); err != nil {
		t.Fatalf("RegisterWorkflow() returned error: %v", err)
	}

	r := newMountedRouter(svc, agents)
	server := httptest.NewServer(r)
	defer server.Close()

	_, token, err := svc.RegisterAgent(context.Background(), "agent-1", "Bot", nil)
	if err != nil {
		t.Fatalf("RegisterAgent() returned error: %v", err)
	}

	exec, err := svc.StartExecution(context.Background(), "wf-1", "agent-1")
	if err != nil {
		t.Fatalf("StartExecution() returned error: %v", err)
	}

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/executions/" + exec.ID() + "/ws"
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, resp, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{
		HTTPHeader: map[string][]string{"Authorization": {"Bearer " + token}},
	})
	if resp != nil && resp.Body != nil {
		defer func() { _ = resp.Body.Close() }()
	}
	if err != nil {
		t.Fatalf("websocket.Dial() returned error: %v", err)
	}
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()

	var receivedCompleted bool
	var count int
	for !receivedCompleted {
		var evt events.Event
		if err := wsjson.Read(ctx, conn, &evt); err != nil {
			t.Fatalf("wsjson.Read() returned error after %d messages: %v", count, err)
		}
		count++
		if evt.ExecutionID != exec.ID() {
			t.Errorf("event ExecutionID = %s, want %s", evt.ExecutionID, exec.ID())
		}
		if evt.Type == events.ExecutionCompleted {
			receivedCompleted = true
		}
	}
	if count == 0 {
		t.Fatal("received zero events over the WebSocket")
	}
}

// TestWS_AcceptsTokenViaQueryParam locks in AuthMiddlewareWS's
// exception: unlike every other route, this one must also accept the
// token as `?token=`, since browsers can't set an Authorization header
// on a WebSocket handshake.
func TestWS_AcceptsTokenViaQueryParam(t *testing.T) {
	agents := newAgentRepoForWS(t)
	tool := &fakeToolAdapter{result: adapters.ToolCallResult{}, delay: 150 * time.Millisecond}
	svc := newTestServiceWithAgentsAndAdapters(t, agents, tool, &fakeModelAdapter{})

	wf, err := workflow.New("wf-1", "Test", 1, []workflow.Step{
		{ID: "a", Type: workflow.StepTypeCallTool, Config: map[string]any{"tool": "slow.tool"}},
	})
	if err != nil {
		t.Fatalf("workflow.New() returned error: %v", err)
	}
	if err := svc.RegisterWorkflow(context.Background(), wf); err != nil {
		t.Fatalf("RegisterWorkflow() returned error: %v", err)
	}

	r := newMountedRouter(svc, agents)
	server := httptest.NewServer(r)
	defer server.Close()

	_, token, err := svc.RegisterAgent(context.Background(), "agent-1", "Bot", nil)
	if err != nil {
		t.Fatalf("RegisterAgent() returned error: %v", err)
	}

	exec, err := svc.StartExecution(context.Background(), "wf-1", "agent-1")
	if err != nil {
		t.Fatalf("StartExecution() returned error: %v", err)
	}

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/executions/" + exec.ID() + "/ws?token=" + token
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, resp, err := websocket.Dial(ctx, wsURL, nil)
	if resp != nil && resp.Body != nil {
		defer func() { _ = resp.Body.Close() }()
	}
	if err != nil {
		t.Fatalf("websocket.Dial() returned error: %v", err)
	}
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()

	var evt events.Event
	if err := wsjson.Read(ctx, conn, &evt); err != nil {
		t.Fatalf("wsjson.Read() returned error: %v", err)
	}
	if evt.ExecutionID != exec.ID() {
		t.Errorf("event ExecutionID = %s, want %s", evt.ExecutionID, exec.ID())
	}
}

// TestWS_RejectsMissingToken confirms the query-token exception isn't
// an "anyone can connect" hole — a request with neither the header nor
// the query param still gets rejected before the Upgrade completes.
func TestWS_RejectsMissingToken(t *testing.T) {
	agents := newAgentRepoForWS(t)
	svc := newTestServiceWithAgentsAndAdapters(t, agents, &fakeToolAdapter{}, &fakeModelAdapter{})
	r := newMountedRouter(svc, agents)
	server := httptest.NewServer(r)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/executions/ghost/ws"
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, resp, err := websocket.Dial(ctx, wsURL, nil)
	if err == nil {
		t.Fatal("websocket.Dial() succeeded, want rejection for missing token")
	}
	if resp != nil {
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != 401 {
			t.Errorf("status = %d, want 401", resp.StatusCode)
		}
	}
}

// TestWS_RejectsDisallowedOrigin and TestWS_AcceptsAllowedOrigin lock
// in the fix for a real bug found during Milestone 6's dashboard
// verification: coder/websocket.Accept enforces its own same-origin
// check independent of (and not covered by) the CORS middleware every
// HTTP route gets, and no Go-based test caught it before a real
// browser did, because net/http/httptest clients never send an Origin
// header the way a browser's WebSocket API does. These tests set that
// header explicitly to close the gap.
func TestWS_RejectsDisallowedOrigin(t *testing.T) {
	agents := newAgentRepoForWS(t)
	svc := newTestServiceWithAgentsAndAdapters(t, agents, &fakeToolAdapter{}, &fakeModelAdapter{})
	r := newMountedRouterWithOrigins(svc, agents, []string{"http://dashboard.example.com"})
	server := httptest.NewServer(r)
	defer server.Close()

	_, token, err := svc.RegisterAgent(context.Background(), "agent-1", "Bot", nil)
	if err != nil {
		t.Fatalf("RegisterAgent() returned error: %v", err)
	}

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/executions/ghost/ws?token=" + token
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, resp, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{
		HTTPHeader: map[string][]string{"Origin": {"http://evil.example.com"}},
	})
	if err == nil {
		t.Fatal("websocket.Dial() succeeded, want rejection for a disallowed Origin")
	}
	if resp != nil {
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != 403 {
			t.Errorf("status = %d, want 403", resp.StatusCode)
		}
	}
}

func TestWS_AcceptsAllowedOrigin(t *testing.T) {
	agents := newAgentRepoForWS(t)
	svc := newTestServiceWithAgentsAndAdapters(t, agents, &fakeToolAdapter{}, &fakeModelAdapter{})
	r := newMountedRouterWithOrigins(svc, agents, []string{"http://dashboard.example.com"})
	server := httptest.NewServer(r)
	defer server.Close()

	_, token, err := svc.RegisterAgent(context.Background(), "agent-1", "Bot", nil)
	if err != nil {
		t.Fatalf("RegisterAgent() returned error: %v", err)
	}

	// "ghost" doesn't need to be a real execution — handleEventsWS
	// subscribes to whatever ID is in the path without checking it
	// exists first (SubscribeEvents just filters a live feed by ID);
	// this test is only exercising the Origin check, not delivery.
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/executions/ghost/ws?token=" + token
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, resp, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{
		HTTPHeader: map[string][]string{"Origin": {"http://dashboard.example.com"}},
	})
	if resp != nil && resp.Body != nil {
		defer func() { _ = resp.Body.Close() }()
	}
	if err != nil {
		t.Fatalf("websocket.Dial() returned error: %v, want a successful handshake for an allowed Origin", err)
	}
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()
}
