// Package policy implements the native policy evaluator named in the
// spec: "implement a native policy evaluator first... expose an
// interface so OPA or Cedar adapters can be added later without
// changing callers." Engine is that interface; NativeEngine is the
// first (and for now, only) implementation.
//
// Rules cover the dimensions the spec names: agent, tool, provider,
// model, budget, and time have concrete Rule implementations in this
// milestone (rule_*.go). Workflow, execution, and environment are
// already present as Input fields for a future rule to match on —
// there's no dedicated rule for them yet simply because nothing in
// this codebase produces a meaningful policy for them today; adding
// one later is purely additive (implement Rule, register it). "User
// role" is explicitly deferred in the spec itself ("future").
//
// Policy decides; it doesn't act. A Decision is a recommendation
// (Allow/Deny + why) for whatever calls Evaluate — this package has no
// dependency on internal/execution or internal/events and doesn't
// record or emit anything itself. Wiring a Decision into an
// ExecutionCreated pause, a PolicyDenied event, or an HTTP 403 is a
// future orchestrator's job (Milestone 5's Execution Manager/Gateway),
// same boundary internal/execution draws around internal/budget and
// internal/events in earlier milestones.
package policy

import (
	"context"
	"errors"
	"time"

	"github.com/dheeraj7000/control-plane/internal/budget"
)

// Effect is a policy decision's outcome.
type Effect string

// The two possible outcomes of a Decision.
const (
	EffectAllow Effect = "allow"
	EffectDeny  Effect = "deny"
)

// Decision is the outcome of evaluating one Input against one or more
// Rules.
type Decision struct {
	Effect Effect
	// Reason is human-readable and intended to end up wherever a denial
	// needs explaining — a future PolicyDenied event's
	// events.DataKeyReason, an API error body, a dashboard tooltip.
	Reason string
	// RuleName is which Rule produced this Decision, filled in by the
	// Engine — individual Rule implementations leave it blank.
	RuleName string
}

// Allowed reports whether d permits the action.
func (d Decision) Allowed() bool { return d.Effect == EffectAllow }

// Input is everything a Rule might need to evaluate one decision point
// (e.g. "may this execution call this tool right now"). Fields are
// broad and mostly optional — a given Rule only reads what's relevant
// to it and returns ok=false (see Rule) when the fields it cares about
// aren't populated.
//
// Budget fields are plain values (budget.Limit / budget.Usage / bool),
// not a live *budget.Ledger — a Rule can read budget state but must
// not be able to call Ledger.Charge() on it. Enforcing "policy only
// reads budget, never mutates it" through the type system beats
// enforcing it by convention.
type Input struct {
	AgentID     string
	WorkflowID  string
	ExecutionID string
	Tool        string
	Provider    string
	Model       string

	BudgetLimit    budget.Limit
	BudgetUsage    budget.Usage
	BudgetExceeded bool

	// Environment matches internal/config's APP_ENV (development,
	// staging, production).
	Environment string
	// Now is caller-supplied rather than each Rule calling time.Now()
	// itself, so time-based Rules stay deterministic and testable.
	Now time.Time

	// Extra carries anything not yet promoted to a first-class field —
	// same "don't guess the shape before there's a real producer"
	// rationale as workflow.Step.Config and events.Event.Data.
	Extra map[string]any
}

// Rule is one native policy check. Evaluate returns ok=false to mean
// "this rule has no opinion about this Input" — evaluation continues
// to the next rule (or the Engine's default effect) rather than
// treating silence as an allow or a deny. This is what lets rules stay
// narrow: a rule that only cares about BudgetExceeded doesn't have to
// say anything about a tool call with no budget context.
type Rule interface {
	Name() string
	Evaluate(ctx context.Context, in Input) (Decision, bool, error)
}

// Engine is the swap point named in the spec: NativeEngine ships
// first; an OPA- or Cedar-backed implementation can be added later
// without changing callers, since it would implement this same
// interface.
type Engine interface {
	Evaluate(ctx context.Context, in Input) (Decision, error)
}

// ErrInvalidEffect is returned by NewNativeEngine for a default effect
// that isn't EffectAllow or EffectDeny.
var ErrInvalidEffect = errors.New("policy: invalid effect")
