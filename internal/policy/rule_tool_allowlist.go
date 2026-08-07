package policy

import (
	"context"
	"fmt"
)

// ToolAllowlistRule denies a tool call from an agent whose configured
// allowlist doesn't include that tool, and allows it when it does.
// Agents with no configured allowlist at all are treated as "no
// opinion" rather than deny-by-default — this codebase has no
// dedicated Agent aggregate yet (see internal/budget's package doc for
// why), so allowedByAgent here is caller-supplied configuration
// standing in for what a real Agent registry would supply once one
// exists.
type ToolAllowlistRule struct {
	allowedByAgent map[string][]string
}

// NewToolAllowlistRule builds a ToolAllowlistRule from a map of agent
// ID to the tool names that agent may call.
func NewToolAllowlistRule(allowedByAgent map[string][]string) ToolAllowlistRule {
	cp := make(map[string][]string, len(allowedByAgent))
	for k, v := range allowedByAgent {
		cp[k] = append([]string(nil), v...)
	}
	return ToolAllowlistRule{allowedByAgent: cp}
}

// Name implements Rule.
func (ToolAllowlistRule) Name() string { return "tool-allowlist" }

// Evaluate implements Rule.
func (r ToolAllowlistRule) Evaluate(_ context.Context, in Input) (Decision, bool, error) {
	if in.Tool == "" || in.AgentID == "" {
		return Decision{}, false, nil
	}
	allowed, configured := r.allowedByAgent[in.AgentID]
	if !configured {
		return Decision{}, false, nil
	}
	for _, t := range allowed {
		if t == in.Tool {
			return Decision{Effect: EffectAllow, Reason: fmt.Sprintf("tool %s is allowlisted for agent %s", in.Tool, in.AgentID)}, true, nil
		}
	}
	return Decision{Effect: EffectDeny, Reason: fmt.Sprintf("tool %s is not allowlisted for agent %s", in.Tool, in.AgentID)}, true, nil
}
