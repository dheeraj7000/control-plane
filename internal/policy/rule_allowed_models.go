package policy

import (
	"context"
	"fmt"
)

// AllowedModelsRule denies any model not in a caller-supplied
// allowlist, and allows one that is. It has no opinion when Input.Model
// is empty (a decision point unrelated to a model call).
type AllowedModelsRule struct {
	allowed map[string]struct{}
}

// NewAllowedModelsRule builds an AllowedModelsRule from the set of
// permitted model identifiers (e.g. "gpt-5", "claude-sonnet-5").
func NewAllowedModelsRule(models []string) AllowedModelsRule {
	set := make(map[string]struct{}, len(models))
	for _, m := range models {
		set[m] = struct{}{}
	}
	return AllowedModelsRule{allowed: set}
}

// Name implements Rule.
func (AllowedModelsRule) Name() string { return "allowed-models" }

// Evaluate implements Rule.
func (r AllowedModelsRule) Evaluate(_ context.Context, in Input) (Decision, bool, error) {
	if in.Model == "" {
		return Decision{}, false, nil
	}
	if _, ok := r.allowed[in.Model]; ok {
		return Decision{Effect: EffectAllow, Reason: fmt.Sprintf("model %s is allowed", in.Model)}, true, nil
	}
	return Decision{Effect: EffectDeny, Reason: fmt.Sprintf("model %s is not in the allowed list", in.Model)}, true, nil
}
