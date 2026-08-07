package gateway_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/dheeraj7000/control-plane/internal/adapters"
	"github.com/dheeraj7000/control-plane/internal/agent"
	"github.com/dheeraj7000/control-plane/internal/budget"
	"github.com/dheeraj7000/control-plane/internal/events"
	"github.com/dheeraj7000/control-plane/internal/execution"
	"github.com/dheeraj7000/control-plane/internal/gateway"
	"github.com/dheeraj7000/control-plane/internal/policy"
	"github.com/dheeraj7000/control-plane/internal/workflow"
)

// fakeToolAdapter and fakeModelAdapter let Service tests exercise the
// full orchestration path without any real network call.

type fakeToolAdapter struct {
	result adapters.ToolCallResult
	err    error
	delay  time.Duration // artificial latency, for tests that need a window to observe in-flight state
	calls  []adapters.ToolCallRequest
}

func (f *fakeToolAdapter) Name() string { return "fake-tool" }
func (f *fakeToolAdapter) ExecuteTool(_ context.Context, req adapters.ToolCallRequest) (adapters.ToolCallResult, error) {
	if f.delay > 0 {
		time.Sleep(f.delay)
	}
	f.calls = append(f.calls, req)
	return f.result, f.err
}

type fakeModelAdapter struct {
	result adapters.ModelCallResult
	err    error
}

func (f *fakeModelAdapter) Name() string { return "fake-model" }
func (f *fakeModelAdapter) CallModel(_ context.Context, _ adapters.ModelCallRequest) (adapters.ModelCallResult, error) {
	return f.result, f.err
}

func silentLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func newTestService(t *testing.T, toolAdapter adapters.Adapter, modelAdapter adapters.ModelAdapter, engine policy.Engine) *gateway.Service {
	t.Helper()
	if engine == nil {
		var err error
		engine, err = policy.NewNativeEngine(policy.EffectAllow)
		if err != nil {
			t.Fatalf("policy.NewNativeEngine() returned error: %v", err)
		}
	}
	svc, err := gateway.NewService(gateway.ServiceConfig{
		Workflows:    workflow.NewInMemoryRepository(),
		Executions:   execution.NewInMemoryRepository(),
		Agents:       agent.NewInMemoryRepository(),
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

func waitForTerminal(t *testing.T, svc *gateway.Service, executionID string, timeout time.Duration) *execution.Execution {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		exec, err := svc.GetExecution(context.Background(), executionID)
		if err != nil {
			t.Fatalf("GetExecution() returned error: %v", err)
		}
		if execution.IsTerminal(exec.State()) {
			return exec
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("execution %s did not reach a terminal state within %v", executionID, timeout)
	return nil
}

func TestNewService_MissingDeps(t *testing.T) {
	full := gateway.ServiceConfig{
		Workflows:    workflow.NewInMemoryRepository(),
		Executions:   execution.NewInMemoryRepository(),
		Agents:       agent.NewInMemoryRepository(),
		Budgets:      budget.NewInMemoryRepository(),
		Events:       events.NewRecorder(events.NewInMemoryStore(), events.NewInMemoryBus()),
		PolicyEngine: mustAllowEngine(t),
		ToolAdapter:  &fakeToolAdapter{},
		ModelAdapter: &fakeModelAdapter{},
		Logger:       silentLogger(),
	}

	tests := []struct {
		name string
		zero func(*gateway.ServiceConfig)
	}{
		{"Workflows", func(c *gateway.ServiceConfig) { c.Workflows = nil }},
		{"Executions", func(c *gateway.ServiceConfig) { c.Executions = nil }},
		{"Agents", func(c *gateway.ServiceConfig) { c.Agents = nil }},
		{"Budgets", func(c *gateway.ServiceConfig) { c.Budgets = nil }},
		{"Events", func(c *gateway.ServiceConfig) { c.Events = nil }},
		{"PolicyEngine", func(c *gateway.ServiceConfig) { c.PolicyEngine = nil }},
		{"ToolAdapter", func(c *gateway.ServiceConfig) { c.ToolAdapter = nil }},
		{"ModelAdapter", func(c *gateway.ServiceConfig) { c.ModelAdapter = nil }},
		{"Logger", func(c *gateway.ServiceConfig) { c.Logger = nil }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := full
			tt.zero(&cfg)
			if _, err := gateway.NewService(cfg); err == nil {
				t.Fatalf("NewService() with nil %s: expected an error", tt.name)
			}
		})
	}
}

func mustAllowEngine(t *testing.T) policy.Engine {
	t.Helper()
	e, err := policy.NewNativeEngine(policy.EffectAllow)
	if err != nil {
		t.Fatalf("policy.NewNativeEngine() returned error: %v", err)
	}
	return e
}

func mixedWorkflow(t *testing.T) workflow.Workflow {
	t.Helper()
	wf, err := workflow.New("wf-1", "Mixed", 1, []workflow.Step{
		{ID: "call-tool", Type: workflow.StepTypeCallTool, Config: map[string]any{
			"tool": "github.search", "args": map[string]any{"query": "x"},
		}},
		{ID: "call-model", Type: workflow.StepTypeModelCall, DependsOn: []string{"call-tool"}, Config: map[string]any{
			"model": "gpt-5", "prompt": "summarize",
		}},
		{ID: "review", Type: workflow.StepTypeReview, DependsOn: []string{"call-model"}},
	})
	if err != nil {
		t.Fatalf("workflow.New() returned error: %v", err)
	}
	return wf
}

func TestStartExecution_RunsToCompletion(t *testing.T) {
	tool := &fakeToolAdapter{result: adapters.ToolCallResult{Output: map[string]any{"ok": true}}}
	model := &fakeModelAdapter{result: adapters.ModelCallResult{Content: "summary", InputTokens: 100, OutputTokens: 50}}
	svc := newTestService(t, tool, model, nil)
	ctx := context.Background()

	if err := svc.RegisterWorkflow(ctx, mixedWorkflow(t)); err != nil {
		t.Fatalf("RegisterWorkflow() returned error: %v", err)
	}

	exec, err := svc.StartExecution(ctx, "wf-1", "")
	if err != nil {
		t.Fatalf("StartExecution() returned error: %v", err)
	}

	final := waitForTerminal(t, svc, exec.ID(), 2*time.Second)
	if final.State() != execution.StateCompleted {
		t.Fatalf("final State() = %s, want %s", final.State(), execution.StateCompleted)
	}

	if len(tool.calls) != 1 || tool.calls[0].Tool != "github.search" {
		t.Errorf("tool adapter calls = %+v, want one call to github.search", tool.calls)
	}

	evts, err := svc.GetEvents(ctx, exec.ID())
	if err != nil {
		t.Fatalf("GetEvents() returned error: %v", err)
	}
	wantSeq := []events.EventType{
		events.ExecutionCreated, events.ExecutionStarted,
		events.StepStarted, events.PolicyEvaluated, events.PolicyApproved,
		events.ToolRequested, events.ToolExecuted, events.StepCompleted,
	}
	if len(evts) < len(wantSeq) {
		t.Fatalf("GetEvents() returned %d events, want at least %d", len(evts), len(wantSeq))
	}
	for i, want := range wantSeq {
		if evts[i].Type != want {
			t.Errorf("evts[%d].Type = %s, want %s", i, evts[i].Type, want)
		}
	}

	// Budget should have been charged for the model call.
	timelineEntries, err := svc.GetTimeline(ctx, exec.ID())
	if err != nil {
		t.Fatalf("GetTimeline() returned error: %v", err)
	}
	foundBudgetUpdate := false
	for _, e := range timelineEntries {
		if e.Type == events.BudgetUpdated {
			foundBudgetUpdate = true
			if e.Detail != "+150 Tokens" {
				t.Errorf("budget timeline Detail = %q, want '+150 Tokens'", e.Detail)
			}
		}
	}
	if !foundBudgetUpdate {
		t.Error("no BudgetUpdated entry found in timeline after a model call")
	}
}

func TestStartExecution_PolicyDeniesStep(t *testing.T) {
	toolRule := policy.NewToolAllowlistRule(map[string][]string{"agent-1": {"some.other.tool"}})
	engine, err := policy.NewNativeEngine(policy.EffectAllow, toolRule)
	if err != nil {
		t.Fatalf("NewNativeEngine() returned error: %v", err)
	}

	tool := &fakeToolAdapter{result: adapters.ToolCallResult{}}
	model := &fakeModelAdapter{}
	svc := newTestService(t, tool, model, engine)
	ctx := context.Background()

	wf, err := workflow.New("wf-1", "Single tool", 1, []workflow.Step{
		{ID: "call-tool", Type: workflow.StepTypeCallTool, Config: map[string]any{"tool": "github.search"}},
	})
	if err != nil {
		t.Fatalf("workflow.New() returned error: %v", err)
	}
	if err := svc.RegisterWorkflow(ctx, wf); err != nil {
		t.Fatal(err)
	}

	exec, err := svc.StartExecution(ctx, "wf-1", "agent-1")
	if err != nil {
		t.Fatalf("StartExecution() returned error: %v", err)
	}
	final := waitForTerminal(t, svc, exec.ID(), 2*time.Second)
	if final.State() != execution.StateFailed {
		t.Fatalf("final State() = %s, want %s (policy should have denied the tool call)", final.State(), execution.StateFailed)
	}
	if len(tool.calls) != 0 {
		t.Errorf("tool adapter was called %d times, want 0 (policy should have denied before the call)", len(tool.calls))
	}

	evts, err := svc.GetEvents(ctx, exec.ID())
	if err != nil {
		t.Fatal(err)
	}
	foundDenied := false
	for _, e := range evts {
		if e.Type == events.PolicyDenied {
			foundDenied = true
		}
	}
	if !foundDenied {
		t.Error("expected a PolicyDenied event")
	}
}

func TestStartExecution_ToolAdapterErrorFailsExecution(t *testing.T) {
	tool := &fakeToolAdapter{err: errors.New("upstream unreachable")}
	svc := newTestService(t, tool, &fakeModelAdapter{}, nil)
	ctx := context.Background()

	wf, err := workflow.New("wf-1", "Single tool", 1, []workflow.Step{
		{ID: "call-tool", Type: workflow.StepTypeCallTool, Config: map[string]any{"tool": "flaky.tool"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.RegisterWorkflow(ctx, wf); err != nil {
		t.Fatal(err)
	}

	exec, err := svc.StartExecution(ctx, "wf-1", "")
	if err != nil {
		t.Fatalf("StartExecution() returned error: %v", err)
	}
	final := waitForTerminal(t, svc, exec.ID(), 2*time.Second)
	if final.State() != execution.StateFailed {
		t.Fatalf("final State() = %s, want %s", final.State(), execution.StateFailed)
	}

	evts, err := svc.GetEvents(ctx, exec.ID())
	if err != nil {
		t.Fatal(err)
	}
	foundToolFailed := false
	for _, e := range evts {
		if e.Type == events.ToolFailed {
			foundToolFailed = true
		}
	}
	if !foundToolFailed {
		t.Error("expected a ToolFailed event")
	}
}

func TestStartExecution_UnknownWorkflow(t *testing.T) {
	svc := newTestService(t, &fakeToolAdapter{}, &fakeModelAdapter{}, nil)
	if _, err := svc.StartExecution(context.Background(), "ghost", ""); err == nil {
		t.Fatal("StartExecution() with an unregistered workflow: expected an error")
	}
}

func TestRegisterAgent(t *testing.T) {
	svc := newTestService(t, &fakeToolAdapter{}, &fakeModelAdapter{}, nil)
	ctx := context.Background()

	a, token, err := svc.RegisterAgent(ctx, "agent-1", "Bot", []string{"github.search"})
	if err != nil {
		t.Fatalf("RegisterAgent() returned error: %v", err)
	}
	if token == "" {
		t.Error("RegisterAgent() returned an empty token")
	}

	got, err := svc.GetAgent(ctx, a.ID())
	if err != nil {
		t.Fatalf("GetAgent() returned error: %v", err)
	}
	if got.ID() != "agent-1" {
		t.Errorf("GetAgent().ID() = %s, want agent-1", got.ID())
	}

	list, err := svc.ListAgents(ctx)
	if err != nil {
		t.Fatalf("ListAgents() returned error: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("ListAgents() len = %d, want 1", len(list))
	}
}

func TestSubscribeEvents_ReceivesLiveEvents(t *testing.T) {
	tool := &fakeToolAdapter{result: adapters.ToolCallResult{}}
	svc := newTestService(t, tool, &fakeModelAdapter{}, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	wf, err := workflow.New("wf-1", "Single tool", 1, []workflow.Step{
		{ID: "call-tool", Type: workflow.StepTypeCallTool, Config: map[string]any{"tool": "a.tool"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.RegisterWorkflow(ctx, wf); err != nil {
		t.Fatal(err)
	}

	exec, err := svc.StartExecution(ctx, "wf-1", "")
	if err != nil {
		t.Fatalf("StartExecution() returned error: %v", err)
	}

	ch, err := svc.SubscribeEvents(ctx, exec.ID())
	if err != nil {
		t.Fatalf("SubscribeEvents() returned error: %v", err)
	}

	received := 0
	timeout := time.After(2 * time.Second)
	for received < 3 {
		select {
		case e, ok := <-ch:
			if !ok {
				t.Fatal("event channel closed before receiving expected events")
			}
			if e.ExecutionID != exec.ID() {
				t.Errorf("received event for %s, want %s", e.ExecutionID, exec.ID())
			}
			received++
		case <-timeout:
			t.Fatalf("timed out after receiving %d events", received)
		}
	}
}

func TestListExecutions_Filters(t *testing.T) {
	svc := newTestService(t, &fakeToolAdapter{}, &fakeModelAdapter{}, nil)
	ctx := context.Background()

	wf, err := workflow.New("wf-1", "Empty-ish", 1, []workflow.Step{
		{ID: "noop", Type: workflow.StepTypeReview},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.RegisterWorkflow(ctx, wf); err != nil {
		t.Fatal(err)
	}

	exec, err := svc.StartExecution(ctx, "wf-1", "")
	if err != nil {
		t.Fatalf("StartExecution() returned error: %v", err)
	}
	waitForTerminal(t, svc, exec.ID(), 2*time.Second)

	list, err := svc.ListExecutions(ctx, execution.ListFilter{WorkflowID: "wf-1"})
	if err != nil {
		t.Fatalf("ListExecutions() returned error: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("ListExecutions() len = %d, want 1", len(list))
	}

	empty, err := svc.ListExecutions(ctx, execution.ListFilter{WorkflowID: "ghost"})
	if err != nil {
		t.Fatalf("ListExecutions() returned error: %v", err)
	}
	if len(empty) != 0 {
		t.Fatalf("ListExecutions(ghost) len = %d, want 0", len(empty))
	}
}

func TestChargeBudget_EmitsBudgetExceeded(t *testing.T) {
	model := &fakeModelAdapter{result: adapters.ModelCallResult{Content: "x", InputTokens: 1000, OutputTokens: 1000}}
	svc, err := gateway.NewService(gateway.ServiceConfig{
		Workflows:    workflow.NewInMemoryRepository(),
		Executions:   execution.NewInMemoryRepository(),
		Agents:       agent.NewInMemoryRepository(),
		Budgets:      budget.NewInMemoryRepository(),
		Events:       events.NewRecorder(events.NewInMemoryStore(), events.NewInMemoryBus()),
		PolicyEngine: mustAllowEngine(t),
		ToolAdapter:  &fakeToolAdapter{},
		ModelAdapter: model,
		Logger:       silentLogger(),
		DefaultExecutionBudget: budget.Limit{
			InputTokens: 100, // the model call above spends far more than this
		},
	})
	if err != nil {
		t.Fatalf("NewService() returned error: %v", err)
	}
	ctx := context.Background()

	wf, err := workflow.New("wf-1", "Model only", 1, []workflow.Step{
		{ID: "call-model", Type: workflow.StepTypeModelCall, Config: map[string]any{"model": "gpt-5", "prompt": "hi"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.RegisterWorkflow(ctx, wf); err != nil {
		t.Fatal(err)
	}

	exec, err := svc.StartExecution(ctx, "wf-1", "")
	if err != nil {
		t.Fatalf("StartExecution() returned error: %v", err)
	}
	waitForTerminal(t, svc, exec.ID(), 2*time.Second)

	evts, err := svc.GetEvents(ctx, exec.ID())
	if err != nil {
		t.Fatal(err)
	}
	foundExceeded := false
	for _, e := range evts {
		if e.Type == events.BudgetExceeded {
			foundExceeded = true
		}
	}
	if !foundExceeded {
		t.Error("expected a BudgetExceeded event after a charge past the configured limit")
	}
}

func TestRegisterWorkflow_DuplicateRejected(t *testing.T) {
	svc := newTestService(t, &fakeToolAdapter{}, &fakeModelAdapter{}, nil)
	ctx := context.Background()
	wf, err := workflow.New("wf-1", "Test", 1, []workflow.Step{{ID: "a", Type: workflow.StepTypeReview}})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.RegisterWorkflow(ctx, wf); err != nil {
		t.Fatalf("first RegisterWorkflow() returned error: %v", err)
	}
	if err := svc.RegisterWorkflow(ctx, wf); err == nil {
		t.Fatal("second RegisterWorkflow() with the same id/version: expected an error")
	}
}

func TestRegisterAgent_DuplicateRejected(t *testing.T) {
	svc := newTestService(t, &fakeToolAdapter{}, &fakeModelAdapter{}, nil)
	ctx := context.Background()
	if _, _, err := svc.RegisterAgent(ctx, "agent-1", "Bot", nil); err != nil {
		t.Fatalf("first RegisterAgent() returned error: %v", err)
	}
	if _, _, err := svc.RegisterAgent(ctx, "agent-1", "Bot 2", nil); err == nil {
		t.Fatal("second RegisterAgent() with the same id: expected an error")
	}
}

func TestGetWorkflowAndListWorkflows(t *testing.T) {
	svc := newTestService(t, &fakeToolAdapter{}, &fakeModelAdapter{}, nil)
	ctx := context.Background()
	wf, err := workflow.New("wf-1", "Test", 1, []workflow.Step{{ID: "a", Type: workflow.StepTypeReview}})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.RegisterWorkflow(ctx, wf); err != nil {
		t.Fatal(err)
	}

	got, err := svc.GetWorkflow(ctx, "wf-1", 1)
	if err != nil {
		t.Fatalf("GetWorkflow() returned error: %v", err)
	}
	if got.ID() != "wf-1" {
		t.Errorf("GetWorkflow().ID() = %s, want wf-1", got.ID())
	}

	latest, err := svc.GetLatestWorkflow(ctx, "wf-1")
	if err != nil {
		t.Fatalf("GetLatestWorkflow() returned error: %v", err)
	}
	if latest.Version() != 1 {
		t.Errorf("GetLatestWorkflow().Version() = %d, want 1", latest.Version())
	}

	list, err := svc.ListWorkflows(ctx)
	if err != nil {
		t.Fatalf("ListWorkflows() returned error: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("ListWorkflows() len = %d, want 1", len(list))
	}
}
