package policy_test

import (
	"context"
	"errors"
	"testing"

	"github.com/dheeraj7000/control-plane/internal/policy"
)

// fakeRule is a minimal, directly-configurable Rule for exercising the
// Engine's combination logic in isolation from any real rule's own
// matching behavior.
type fakeRule struct {
	name    string
	decide  bool
	effect  policy.Effect
	failErr error
}

func (r fakeRule) Name() string { return r.name }

func (r fakeRule) Evaluate(_ context.Context, _ policy.Input) (policy.Decision, bool, error) {
	if r.failErr != nil {
		return policy.Decision{}, false, r.failErr
	}
	if !r.decide {
		return policy.Decision{}, false, nil
	}
	return policy.Decision{Effect: r.effect, Reason: "fake"}, true, nil
}

func TestNewNativeEngine_InvalidDefaultEffect(t *testing.T) {
	if _, err := policy.NewNativeEngine("bogus"); !errors.Is(err, policy.ErrInvalidEffect) {
		t.Fatalf("NewNativeEngine() error = %v, want ErrInvalidEffect", err)
	}
}

func TestEvaluate_NoRulesUsesDefault(t *testing.T) {
	engine, err := policy.NewNativeEngine(policy.EffectDeny)
	if err != nil {
		t.Fatalf("NewNativeEngine() returned error: %v", err)
	}
	d, err := engine.Evaluate(context.Background(), policy.Input{})
	if err != nil {
		t.Fatalf("Evaluate() returned error: %v", err)
	}
	if d.Effect != policy.EffectDeny {
		t.Errorf("Effect = %s, want %s", d.Effect, policy.EffectDeny)
	}
}

func TestEvaluate_NoOpinionRulesFallThroughToDefault(t *testing.T) {
	engine, err := policy.NewNativeEngine(policy.EffectAllow,
		fakeRule{name: "silent-1", decide: false},
		fakeRule{name: "silent-2", decide: false},
	)
	if err != nil {
		t.Fatalf("NewNativeEngine() returned error: %v", err)
	}
	d, err := engine.Evaluate(context.Background(), policy.Input{})
	if err != nil {
		t.Fatalf("Evaluate() returned error: %v", err)
	}
	if d.Effect != policy.EffectAllow {
		t.Errorf("Effect = %s, want %s", d.Effect, policy.EffectAllow)
	}
}

func TestEvaluate_ExplicitAllow(t *testing.T) {
	engine, err := policy.NewNativeEngine(policy.EffectDeny,
		fakeRule{name: "allower", decide: true, effect: policy.EffectAllow},
	)
	if err != nil {
		t.Fatalf("NewNativeEngine() returned error: %v", err)
	}
	d, err := engine.Evaluate(context.Background(), policy.Input{})
	if err != nil {
		t.Fatalf("Evaluate() returned error: %v", err)
	}
	if !d.Allowed() {
		t.Errorf("Decision = %+v, want Allowed()", d)
	}
	if d.RuleName != "allower" {
		t.Errorf("RuleName = %q, want allower", d.RuleName)
	}
}

func TestEvaluate_DenyOverridesAllowRegardlessOfOrder(t *testing.T) {
	// Allow registered first, deny second — deny must still win.
	engine, err := policy.NewNativeEngine(policy.EffectAllow,
		fakeRule{name: "allower", decide: true, effect: policy.EffectAllow},
		fakeRule{name: "denier", decide: true, effect: policy.EffectDeny},
	)
	if err != nil {
		t.Fatalf("NewNativeEngine() returned error: %v", err)
	}
	d, err := engine.Evaluate(context.Background(), policy.Input{})
	if err != nil {
		t.Fatalf("Evaluate() returned error: %v", err)
	}
	if d.Allowed() {
		t.Fatalf("Decision = %+v, want denied (deny overrides)", d)
	}
	if d.RuleName != "denier" {
		t.Errorf("RuleName = %q, want denier", d.RuleName)
	}

	// Reverse the registration order — result must be identical.
	engine2, err := policy.NewNativeEngine(policy.EffectAllow,
		fakeRule{name: "denier", decide: true, effect: policy.EffectDeny},
		fakeRule{name: "allower", decide: true, effect: policy.EffectAllow},
	)
	if err != nil {
		t.Fatalf("NewNativeEngine() returned error: %v", err)
	}
	d2, err := engine2.Evaluate(context.Background(), policy.Input{})
	if err != nil {
		t.Fatalf("Evaluate() returned error: %v", err)
	}
	if d2.Allowed() {
		t.Fatalf("Decision = %+v, want denied regardless of rule order", d2)
	}
}

func TestEvaluate_RuleErrorPropagates(t *testing.T) {
	boom := errors.New("boom")
	engine, err := policy.NewNativeEngine(policy.EffectAllow,
		fakeRule{name: "broken", failErr: boom},
	)
	if err != nil {
		t.Fatalf("NewNativeEngine() returned error: %v", err)
	}
	_, err = engine.Evaluate(context.Background(), policy.Input{})
	if !errors.Is(err, boom) {
		t.Fatalf("Evaluate() error = %v, want wrapping %v", err, boom)
	}
}

func TestDecision_Allowed(t *testing.T) {
	if !(policy.Decision{Effect: policy.EffectAllow}).Allowed() {
		t.Error("Allowed() = false for EffectAllow")
	}
	if (policy.Decision{Effect: policy.EffectDeny}).Allowed() {
		t.Error("Allowed() = true for EffectDeny")
	}
}
