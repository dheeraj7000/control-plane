// Package gateway is the ingress boundary of the control plane — per
// the Milestone 1 decision recorded in docs/architecture.md, it
// terminates inbound HTTP, authenticates the caller (internal/agent),
// applies rate limiting, and translates requests into Execution
// commands via internal/adapters. This is also where the spec's
// "Execution Manager" box finally gets built: Service is the
// orchestrator that composes internal/workflow, internal/execution,
// internal/events, internal/policy, internal/budget, and
// internal/adapters — every one of those packages was deliberately
// kept ignorant of the others across Milestones 2-4 specifically so
// this composition would be possible without touching any of them.
package gateway

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/dheeraj7000/control-plane/internal/adapters"
	"github.com/dheeraj7000/control-plane/internal/agent"
	"github.com/dheeraj7000/control-plane/internal/budget"
	"github.com/dheeraj7000/control-plane/internal/events"
	"github.com/dheeraj7000/control-plane/internal/execution"
	"github.com/dheeraj7000/control-plane/internal/policy"
	"github.com/dheeraj7000/control-plane/internal/timeline"
	"github.com/dheeraj7000/control-plane/internal/workflow"
	"github.com/dheeraj7000/control-plane/pkg/id"
)

// ServiceConfig is everything Service needs. All fields except
// DefaultExecutionBudget and Environment are required — NewService
// returns an error if any is nil.
type ServiceConfig struct {
	Workflows    workflow.Repository
	Executions   execution.Repository
	Agents       agent.Repository
	Budgets      budget.Repository
	Events       *events.Recorder
	PolicyEngine policy.Engine
	ToolAdapter  adapters.Adapter
	ModelAdapter adapters.ModelAdapter
	Logger       *slog.Logger

	// DefaultExecutionBudget seeds every new execution-scope Ledger.
	// The zero value (Limit{}) means unlimited — reasonable for a
	// milestone with no UI yet to configure real limits, but callers
	// wiring this into a real deployment should set one.
	DefaultExecutionBudget budget.Limit
	// Environment feeds policy.Input.Environment (e.g. "development",
	// "production") for TimeWindowRule/future environment-based rules.
	Environment string
}

// Service is the control plane's orchestrator.
type Service struct {
	cfg ServiceConfig
}

// NewService validates cfg and returns a Service.
func NewService(cfg ServiceConfig) (*Service, error) {
	switch {
	case cfg.Workflows == nil:
		return nil, fmt.Errorf("gateway: ServiceConfig.Workflows is required")
	case cfg.Executions == nil:
		return nil, fmt.Errorf("gateway: ServiceConfig.Executions is required")
	case cfg.Agents == nil:
		return nil, fmt.Errorf("gateway: ServiceConfig.Agents is required")
	case cfg.Budgets == nil:
		return nil, fmt.Errorf("gateway: ServiceConfig.Budgets is required")
	case cfg.Events == nil:
		return nil, fmt.Errorf("gateway: ServiceConfig.Events is required")
	case cfg.PolicyEngine == nil:
		return nil, fmt.Errorf("gateway: ServiceConfig.PolicyEngine is required")
	case cfg.ToolAdapter == nil:
		return nil, fmt.Errorf("gateway: ServiceConfig.ToolAdapter is required")
	case cfg.ModelAdapter == nil:
		return nil, fmt.Errorf("gateway: ServiceConfig.ModelAdapter is required")
	case cfg.Logger == nil:
		return nil, fmt.Errorf("gateway: ServiceConfig.Logger is required")
	}
	return &Service{cfg: cfg}, nil
}

// RegisterAgent registers a new Agent and returns it along with its
// plaintext bearer token — the only time that token is available, see
// agent.New.
func (s *Service) RegisterAgent(ctx context.Context, agentID, name string, allowedTools []string) (agent.Agent, string, error) {
	a, token, err := agent.New(agentID, name, allowedTools)
	if err != nil {
		return agent.Agent{}, "", err
	}
	if err := s.cfg.Agents.Create(ctx, a); err != nil {
		return agent.Agent{}, "", fmt.Errorf("gateway: persist agent: %w", err)
	}
	return a, token, nil
}

// GetAgent looks up an Agent by ID.
func (s *Service) GetAgent(ctx context.Context, agentID string) (agent.Agent, error) {
	return s.cfg.Agents.Get(ctx, agentID)
}

// ListAgents returns every registered Agent.
func (s *Service) ListAgents(ctx context.Context) ([]agent.Agent, error) {
	return s.cfg.Agents.List(ctx)
}

// RegisterWorkflow persists an already-constructed (and therefore
// already-validated, see workflow.New) Workflow.
func (s *Service) RegisterWorkflow(ctx context.Context, wf workflow.Workflow) error {
	if err := s.cfg.Workflows.Create(ctx, wf); err != nil {
		return fmt.Errorf("gateway: persist workflow: %w", err)
	}
	return nil
}

// GetWorkflow looks up an exact (id, version).
func (s *Service) GetWorkflow(ctx context.Context, workflowID string, version int) (workflow.Workflow, error) {
	return s.cfg.Workflows.Get(ctx, workflowID, version)
}

// GetLatestWorkflow looks up the highest version stored for workflowID.
func (s *Service) GetLatestWorkflow(ctx context.Context, workflowID string) (workflow.Workflow, error) {
	return s.cfg.Workflows.GetLatest(ctx, workflowID)
}

// ListWorkflows returns the latest version of every registered Workflow.
func (s *Service) ListWorkflows(ctx context.Context) ([]workflow.Workflow, error) {
	return s.cfg.Workflows.List(ctx)
}

// StartExecution creates an Execution against workflowID's latest
// version, attributes it to agentID (may be ""), durably records
// ExecutionCreated, and returns immediately — the execution then runs
// in the background. Callers observe progress via GetExecution,
// GetTimeline, GetEvents, or SubscribeEvents, not by StartExecution
// blocking until completion.
func (s *Service) StartExecution(ctx context.Context, workflowID, agentID string) (*execution.Execution, error) {
	wf, err := s.cfg.Workflows.GetLatest(ctx, workflowID)
	if err != nil {
		return nil, fmt.Errorf("gateway: lookup workflow %s: %w", workflowID, err)
	}

	var opts []execution.Option
	if agentID != "" {
		opts = append(opts, execution.WithAgentID(agentID))
	}
	exec, err := execution.New(id.New("exec"), wf, opts...)
	if err != nil {
		return nil, fmt.Errorf("gateway: create execution: %w", err)
	}

	if _, err := s.cfg.Events.Record(ctx, exec.ID(), events.ExecutionCreated, nil); err != nil {
		return nil, fmt.Errorf("gateway: record ExecutionCreated: %w", err)
	}
	if err := s.cfg.Executions.Create(ctx, exec); err != nil {
		return nil, fmt.Errorf("gateway: persist execution: %w", err)
	}

	// Run in the background, deliberately NOT using ctx (the inbound
	// HTTP request's context, which is cancelled the moment the
	// handler returns) — this work must outlive the request that
	// triggered it. context.Background() is correct here, not a bug.
	go s.run(context.Background(), exec.ID())

	return exec, nil
}

// GetExecution looks up an Execution by ID.
func (s *Service) GetExecution(ctx context.Context, executionID string) (*execution.Execution, error) {
	return s.cfg.Executions.Get(ctx, executionID)
}

// ListExecutions returns Executions matching filter.
func (s *Service) ListExecutions(ctx context.Context, filter execution.ListFilter) ([]*execution.Execution, error) {
	return s.cfg.Executions.List(ctx, filter)
}

// GetTimeline projects executionID's full event stream into a Timeline
// — see internal/timeline's package doc for why this doubles as
// "replay the execution timeline" in its safe, read-only sense.
func (s *Service) GetTimeline(ctx context.Context, executionID string) ([]timeline.Entry, error) {
	evts, err := s.cfg.Events.Store().List(ctx, executionID)
	if err != nil {
		return nil, err
	}
	return timeline.Build(evts), nil
}

// GetEvents returns executionID's raw event stream, Sequence-ordered.
func (s *Service) GetEvents(ctx context.Context, executionID string) ([]events.Event, error) {
	return s.cfg.Events.Store().List(ctx, executionID)
}

// SubscribeEvents returns a live channel of executionID's events —
// the WebSocket handler's data source.
func (s *Service) SubscribeEvents(ctx context.Context, executionID string) (<-chan events.Event, error) {
	return s.cfg.Events.Bus().Subscribe(ctx, events.Filter{ExecutionID: executionID})
}

// run drives exec's steps to completion. It is intentionally
// synchronous and sequential — TopologicalOrder() one step at a time,
// stopping at the first failure — not the concurrent,
// backoff-and-retry-aware dispatch a real Scheduler (internal/
// scheduler, still unbuilt) would do. This is the simplest driver that
// proves the full wiring (policy, budget, adapters, events) works
// end-to-end; making it concurrent/resilient is future work layered on
// top, not a rewrite of this method's shape.
func (s *Service) run(ctx context.Context, executionID string) {
	logger := s.cfg.Logger.With(slog.String("execution_id", executionID))

	exec, err := s.cfg.Executions.Get(ctx, executionID)
	if err != nil {
		logger.Error("run: load execution", slog.String("error", err.Error()))
		return
	}
	wf, err := s.cfg.Workflows.Get(ctx, exec.WorkflowID(), exec.WorkflowVersion())
	if err != nil {
		logger.Error("run: load workflow", slog.String("error", err.Error()))
		return
	}

	if err := exec.Transition(execution.StateQueued, "accepted"); err != nil {
		logger.Error("run: transition to queued", slog.String("error", err.Error()))
		return
	}
	if err := exec.Transition(execution.StateRunning, "dispatched"); err != nil {
		logger.Error("run: transition to running", slog.String("error", err.Error()))
		return
	}
	if _, err := s.cfg.Events.Record(ctx, executionID, events.ExecutionStarted, nil); err != nil {
		logger.Warn("run: record ExecutionStarted", slog.String("error", err.Error()))
	}
	if err := s.cfg.Executions.Update(ctx, exec); err != nil {
		logger.Error("run: persist running state", slog.String("error", err.Error()))
		return
	}

	var runErr error
	for _, stepID := range wf.TopologicalOrder() {
		if runErr = s.runStep(ctx, exec, wf, stepID); runErr != nil {
			break
		}
		if err := s.cfg.Executions.Update(ctx, exec); err != nil {
			logger.Error("run: persist after step", slog.String("step_id", stepID), slog.String("error", err.Error()))
		}
	}

	if runErr != nil {
		_ = exec.Transition(execution.StateFailed, runErr.Error())
		if _, err := s.cfg.Events.Record(ctx, executionID, events.ExecutionFailed, map[string]any{
			events.DataKeyReason: runErr.Error(),
		}); err != nil {
			logger.Warn("run: record ExecutionFailed", slog.String("error", err.Error()))
		}
	} else {
		_ = exec.Transition(execution.StateCompleted, "all steps completed")
		if _, err := s.cfg.Events.Record(ctx, executionID, events.ExecutionCompleted, nil); err != nil {
			logger.Warn("run: record ExecutionCompleted", slog.String("error", err.Error()))
		}
	}
	if err := s.cfg.Executions.Update(ctx, exec); err != nil {
		logger.Error("run: persist final state", slog.String("error", err.Error()))
	}
}

// runStep drives one step: StepStarted -> policy check -> (tool call |
// model call | no-op for step types with no adapter yet) ->
// StepCompleted/StepFailed. Returns a non-nil error only for failures
// that should stop the whole execution (matching run's
// stop-at-first-failure driver).
func (s *Service) runStep(ctx context.Context, exec *execution.Execution, wf workflow.Workflow, stepID string) error {
	logger := s.cfg.Logger.With(slog.String("execution_id", exec.ID()), slog.String("step_id", stepID))

	step, ok := wf.Step(stepID)
	if !ok {
		return fmt.Errorf("gateway: step %s not found in workflow %s", stepID, wf.ID())
	}

	if err := exec.StartStep(stepID); err != nil {
		return fmt.Errorf("gateway: start step %s: %w", stepID, err)
	}
	stepData := map[string]any{events.DataKeyStepID: stepID, events.DataKeyStepName: step.Name}
	if _, err := s.cfg.Events.Record(ctx, exec.ID(), events.StepStarted, stepData); err != nil {
		logger.Warn("record StepStarted", slog.String("error", err.Error()))
	}

	decision, err := s.evaluatePolicy(ctx, exec, wf, step)
	if err != nil {
		return s.failStep(ctx, exec, step, fmt.Errorf("policy evaluation: %w", err))
	}
	if !decision.Allowed() {
		return s.failStep(ctx, exec, step, fmt.Errorf("denied by policy: %s", decision.Reason))
	}

	switch step.Type {
	case workflow.StepTypeCallTool:
		if err := s.runToolStep(ctx, exec, step); err != nil {
			return s.failStep(ctx, exec, step, err)
		}
	case workflow.StepTypeModelCall:
		if err := s.runModelStep(ctx, exec, step); err != nil {
			return s.failStep(ctx, exec, step, err)
		}
	default:
		// Search, Summarize, Review, Wait, Approval: no adapter backs
		// these yet (they need the Tool Registry / a real approval
		// routing mechanism — both still-open items, see
		// docs/architecture.md). Complete immediately so a mixed-type
		// workflow can still run end-to-end in this milestone rather
		// than getting stuck on a step type nothing implements.
		logger.Info("step type has no adapter yet, completing as a no-op", slog.String("type", string(step.Type)))
	}

	if err := exec.CompleteStep(stepID); err != nil {
		return fmt.Errorf("gateway: complete step %s: %w", stepID, err)
	}
	if _, err := s.cfg.Events.Record(ctx, exec.ID(), events.StepCompleted, stepData); err != nil {
		logger.Warn("record StepCompleted", slog.String("error", err.Error()))
	}
	return nil
}

func (s *Service) failStep(ctx context.Context, exec *execution.Execution, step workflow.Step, cause error) error {
	_ = exec.FailStep(step.ID, cause.Error())
	if _, err := s.cfg.Events.Record(ctx, exec.ID(), events.StepFailed, map[string]any{
		events.DataKeyStepID:   step.ID,
		events.DataKeyStepName: step.Name,
		events.DataKeyReason:   cause.Error(),
	}); err != nil {
		s.cfg.Logger.Warn("record StepFailed", slog.String("error", err.Error()))
	}
	return cause
}

// evaluatePolicy builds a policy.Input from available context (the
// execution-scope budget ledger, the step's tool/model if any) and
// records PolicyEvaluated plus PolicyApproved/PolicyDenied, matching
// the spec's event chain (which names the immediate next event
// "ToolApproved" in its one worked example but "PolicyApproved" in its
// general event list — internal/events standardized on the general
// PolicyApproved/PolicyDenied pair in Milestone 3, since it applies to
// model-call steps too, not just tool calls).
func (s *Service) evaluatePolicy(ctx context.Context, exec *execution.Execution, wf workflow.Workflow, step workflow.Step) (policy.Decision, error) {
	in := policy.Input{
		AgentID:     exec.AgentID(),
		WorkflowID:  wf.ID(),
		ExecutionID: exec.ID(),
		Environment: s.cfg.Environment,
		Now:         time.Now().UTC(),
	}
	if tool, ok := step.Config["tool"].(string); ok {
		in.Tool = tool
	}
	if model, ok := step.Config["model"].(string); ok {
		in.Model = model
	}

	ledger, err := s.cfg.Budgets.GetOrCreate(ctx, budget.ScopeExecution, exec.ID(), "", s.cfg.DefaultExecutionBudget)
	if err == nil {
		in.BudgetLimit = ledger.Limit()
		in.BudgetUsage = ledger.Usage()
		in.BudgetExceeded = ledger.Exceeded()
	} else {
		s.cfg.Logger.Warn("evaluatePolicy: load budget ledger", slog.String("error", err.Error()))
	}

	decision, err := s.cfg.PolicyEngine.Evaluate(ctx, in)
	if err != nil {
		return policy.Decision{}, err
	}

	policyData := map[string]any{events.DataKeyPolicyName: decision.RuleName, events.DataKeyReason: decision.Reason}
	if _, err := s.cfg.Events.Record(ctx, exec.ID(), events.PolicyEvaluated, policyData); err != nil {
		s.cfg.Logger.Warn("record PolicyEvaluated", slog.String("error", err.Error()))
	}
	if decision.Allowed() {
		if _, err := s.cfg.Events.Record(ctx, exec.ID(), events.PolicyApproved, policyData); err != nil {
			s.cfg.Logger.Warn("record PolicyApproved", slog.String("error", err.Error()))
		}
	} else if _, err := s.cfg.Events.Record(ctx, exec.ID(), events.PolicyDenied, policyData); err != nil {
		s.cfg.Logger.Warn("record PolicyDenied", slog.String("error", err.Error()))
	}
	return decision, nil
}

func (s *Service) runToolStep(ctx context.Context, exec *execution.Execution, step workflow.Step) error {
	toolName, _ := step.Config["tool"].(string)
	args, _ := step.Config["args"].(map[string]any)

	toolData := map[string]any{events.DataKeyToolName: toolName}
	if _, err := s.cfg.Events.Record(ctx, exec.ID(), events.ToolRequested, toolData); err != nil {
		s.cfg.Logger.Warn("record ToolRequested", slog.String("error", err.Error()))
	}

	_, err := s.cfg.ToolAdapter.ExecuteTool(ctx, adapters.ToolCallRequest{Tool: toolName, Args: args})
	if err != nil {
		failData := map[string]any{events.DataKeyToolName: toolName, events.DataKeyReason: err.Error()}
		if _, recErr := s.cfg.Events.Record(ctx, exec.ID(), events.ToolFailed, failData); recErr != nil {
			s.cfg.Logger.Warn("record ToolFailed", slog.String("error", recErr.Error()))
		}
		return fmt.Errorf("tool %s: %w", toolName, err)
	}

	if _, err := s.cfg.Events.Record(ctx, exec.ID(), events.ToolExecuted, toolData); err != nil {
		s.cfg.Logger.Warn("record ToolExecuted", slog.String("error", err.Error()))
	}
	return nil
}

func (s *Service) runModelStep(ctx context.Context, exec *execution.Execution, step workflow.Step) error {
	model, _ := step.Config["model"].(string)
	prompt, _ := step.Config["prompt"].(string)

	result, err := s.cfg.ModelAdapter.CallModel(ctx, adapters.ModelCallRequest{
		Model:    model,
		Messages: []adapters.Message{{Role: "user", Content: prompt}},
	})
	if err != nil {
		return fmt.Errorf("model %s: %w", model, err)
	}

	if err := s.chargeBudget(ctx, exec.ID(), budget.Usage{
		InputTokens:  result.InputTokens,
		OutputTokens: result.OutputTokens,
	}); err != nil {
		s.cfg.Logger.Warn("charge budget", slog.String("error", err.Error()))
	}
	return nil
}

func (s *Service) chargeBudget(ctx context.Context, executionID string, delta budget.Usage) error {
	ledger, err := s.cfg.Budgets.GetOrCreate(ctx, budget.ScopeExecution, executionID, "", s.cfg.DefaultExecutionBudget)
	if err != nil {
		return fmt.Errorf("load ledger: %w", err)
	}
	if err := ledger.Charge(delta); err != nil {
		return fmt.Errorf("charge: %w", err)
	}
	if err := s.cfg.Budgets.Save(ctx, ledger); err != nil {
		return fmt.Errorf("save ledger: %w", err)
	}

	if _, err := s.cfg.Events.Record(ctx, executionID, events.BudgetUpdated, map[string]any{
		events.DataKeyTokenDelta: delta.InputTokens + delta.OutputTokens,
	}); err != nil {
		s.cfg.Logger.Warn("record BudgetUpdated", slog.String("error", err.Error()))
	}
	if ledger.Exceeded() {
		if _, err := s.cfg.Events.Record(ctx, executionID, events.BudgetExceeded, nil); err != nil {
			s.cfg.Logger.Warn("record BudgetExceeded", slog.String("error", err.Error()))
		}
	}
	return nil
}
