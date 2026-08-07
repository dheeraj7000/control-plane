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
