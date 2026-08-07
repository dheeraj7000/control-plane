package workflow

// StepType identifies the kind of work a Step performs. This is the
// closed set named in the spec; adding a new kind means adding a case
// here and to Valid(), not inventing a free-form string at the call
// site.
type StepType string

// The seven step types named in the spec.
const (
	StepTypeSearch    StepType = "search"
	StepTypeSummarize StepType = "summarize"
	StepTypeCallTool  StepType = "call_tool"
	StepTypeReview    StepType = "review"
	StepTypeWait      StepType = "wait"
	StepTypeApproval  StepType = "approval"
	StepTypeModelCall StepType = "model_call"
)

// Valid reports whether t is one of the known step types.
func (t StepType) Valid() bool {
	switch t {
	case StepTypeSearch, StepTypeSummarize, StepTypeCallTool, StepTypeReview, StepTypeWait, StepTypeApproval, StepTypeModelCall:
		return true
	default:
		return false
	}
}

// Step is a single node in a Workflow's dependency graph. Config is
// intentionally untyped (map[string]any) in this milestone — per-type
// config structs (e.g. a CallToolConfig with a ToolName field) are a
// natural follow-up once the Gateway/Adapter milestone defines what
// each step type actually needs, but adding that now would be
// guessing at a shape we don't have evidence for yet.
type Step struct {
	// ID must be unique within a Workflow. It's what DependsOn
	// references and what Execution step-run tracking keys on.
	ID string `json:"id"`
	// Name is a human-readable label; purely cosmetic.
	Name string   `json:"name,omitempty"`
	Type StepType `json:"type"`
	// DependsOn lists Step IDs that must complete before this step can
	// run. An empty slice means the step is a root and can run as soon
	// as the execution starts.
	DependsOn []string `json:"depends_on,omitempty"`
	// Config carries step-type-specific parameters, e.g. a tool name
	// for StepTypeCallTool or a model identifier for StepTypeModelCall.
	Config map[string]any `json:"config,omitempty"`
}
