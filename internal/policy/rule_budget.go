package policy

import "context"

// BudgetRule denies when Input.BudgetExceeded is true. It never
// expresses an explicit allow — it's a pure veto, matching its single
// responsibility — and has no opinion at all when BudgetExceeded is
// false, letting other rules (or the Engine's default) decide.
type BudgetRule struct{}

// NewBudgetRule constructs a BudgetRule. It has no configuration.
func NewBudgetRule() BudgetRule { return BudgetRule{} }

// Name implements Rule.
func (BudgetRule) Name() string { return "budget-exceeded" }

// Evaluate implements Rule.
func (BudgetRule) Evaluate(_ context.Context, in Input) (Decision, bool, error) {
	if in.BudgetExceeded {
		return Decision{Effect: EffectDeny, Reason: "budget exceeded"}, true, nil
	}
	return Decision{}, false, nil
}
