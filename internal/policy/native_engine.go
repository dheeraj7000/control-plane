package policy

import (
	"context"
	"fmt"
)

// NativeEngine evaluates every registered Rule and combines their
// opinions with a deny-overrides model, the standard safety-biased
// choice for a policy engine (the same principle AWS IAM's explicit
// deny and Kubernetes admission webhooks use): if any Rule denies,
// that denial wins immediately regardless of what other rules said or
// what order they ran in. If no Rule denies but at least one allows,
// the action is allowed. If no Rule has any opinion at all, the
// configured defaultEffect applies.
//
// This is deliberately not "first matching rule wins" — that would
// make behavior depend on registration order, which is a common source
// of subtle policy bugs (an early permissive rule accidentally
// shadowing a later, stricter one).
type NativeEngine struct {
	rules         []Rule
	defaultEffect Effect
}

// NewNativeEngine builds an Engine over rules. defaultEffect applies
// when no rule expresses an opinion; EffectDeny is the safer choice for
// most deployments (fail closed) but EffectAllow may suit a permissive
// development environment — the caller decides, this package doesn't
// pick a default for you.
func NewNativeEngine(defaultEffect Effect, rules ...Rule) (*NativeEngine, error) {
	if defaultEffect != EffectAllow && defaultEffect != EffectDeny {
		return nil, fmt.Errorf("%w: %s", ErrInvalidEffect, defaultEffect)
	}
	return &NativeEngine{
		rules:         append([]Rule(nil), rules...),
		defaultEffect: defaultEffect,
	}, nil
}

// Evaluate implements Engine.
func (e *NativeEngine) Evaluate(ctx context.Context, in Input) (Decision, error) {
	sawAllow := false
	var allowDecision Decision

	for _, r := range e.rules {
		d, ok, err := r.Evaluate(ctx, in)
		if err != nil {
			return Decision{}, fmt.Errorf("policy: rule %s: %w", r.Name(), err)
		}
		if !ok {
			continue
		}
		d.RuleName = r.Name()

		if d.Effect == EffectDeny {
			return d, nil // deny overrides: short-circuit immediately
		}
		if !sawAllow {
			sawAllow = true
			allowDecision = d
		}
	}

	if sawAllow {
		return allowDecision, nil
	}
	return Decision{
		Effect: e.defaultEffect,
		Reason: "no rule matched; default effect applied",
	}, nil
}
