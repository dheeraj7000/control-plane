// Package budget tracks input/output tokens and estimated cost against
// caps at three scopes named in the spec: per-execution, daily, and
// monthly. Policy rules reference this package's Limit/Usage directly
// (see internal/policy) — Budget's job is accurate tracking, not
// deciding anything; whether an exceeded budget should deny a tool
// call is Policy's call, not Budget's.
//
// Scoping note: a daily or monthly Limit is naturally owned by
// whoever's spend it caps — the spec's "Agent" domain concept (owns
// credentials, policies, budgets, allowed tools). There is no
// dedicated internal/agent package in this codebase yet — the spec
// names Agent in its Domain Model section but its own repository
// layout never gives it one, and building a full Agent aggregate
// (registration, credentials, permissions) is out of scope for this
// milestone; it belongs with Milestone 5's Gateway, where agents
// actually authenticate and credential-storage decisions (an open
// question since Milestone 1) get made. Until then, OwnerID here is an
// opaque string the caller assigns meaning to — an execution ID for
// ScopeExecution, or an agent identifier of the caller's choosing for
// ScopeDaily/ScopeMonthly.
package budget

import (
	"errors"
	"fmt"
	"time"
)

// Scope identifies which budget dimension a Ledger caps.
type Scope string

// The three scopes named in the spec.
const (
	ScopeExecution Scope = "execution"
	ScopeDaily     Scope = "daily"
	ScopeMonthly   Scope = "monthly"
)

// Valid reports whether s is one of the three known scopes.
func (s Scope) Valid() bool {
	switch s {
	case ScopeExecution, ScopeDaily, ScopeMonthly:
		return true
	default:
		return false
	}
}

// Cost is a monetary amount in micro-USD (1,000,000 = $1.00). Integer
// arithmetic here avoids the floating-point drift that accumulating
// many small per-token charges in float64 dollars would eventually
// produce.
type Cost int64

// USD converts c to a floating-point dollar amount for display.
func (c Cost) USD() float64 { return float64(c) / 1_000_000 }

// Limit caps consumption for one (Scope, OwnerID, PeriodKey) Ledger.
// A zero field means "no cap on that dimension" — a Limit that only
// caps Cost but not token counts is valid.
type Limit struct {
	InputTokens  int64
	OutputTokens int64
	Cost         Cost
}

// Usage is the running total consumed so far against a Limit.
type Usage struct {
	InputTokens  int64
	OutputTokens int64
	Cost         Cost
}

// Sentinel errors.
var (
	ErrEmptyOwnerID   = errors.New("budget: owner id must not be empty")
	ErrInvalidScope   = errors.New("budget: invalid scope")
	ErrNegativeCharge = errors.New("budget: charge amounts must not be negative")
)

// Ledger tracks Usage against a Limit for one (Scope, OwnerID,
// PeriodKey) triple. PeriodKey buckets Daily/Monthly ledgers by
// calendar period (see PeriodKey below); it's always "" for
// ScopeExecution, since OwnerID (the execution ID) is already unique.
type Ledger struct {
	scope     Scope
	ownerID   string
	periodKey string
	limit     Limit
	usage     Usage
}

// New constructs a Ledger with zero usage. periodKey should come from
// PeriodKey (or be "" for ScopeExecution) — it's accepted as a plain
// string rather than a time.Time so a Repository can look up an
// existing ledger by the same key it was created with, without
// recomputing "which day is this" and risking a mismatch across a
// midnight boundary between create and lookup.
func New(scope Scope, ownerID, periodKey string, limit Limit) (*Ledger, error) {
	if ownerID == "" {
		return nil, ErrEmptyOwnerID
	}
	if !scope.Valid() {
		return nil, fmt.Errorf("%w: %s", ErrInvalidScope, scope)
	}
	return &Ledger{scope: scope, ownerID: ownerID, periodKey: periodKey, limit: limit}, nil
}

// PeriodKey computes the calendar bucket for scope at time t, in UTC.
// ScopeExecution doesn't bucket by time and always returns "".
func PeriodKey(scope Scope, t time.Time) string {
	switch scope {
	case ScopeDaily:
		return t.UTC().Format("2006-01-02")
	case ScopeMonthly:
		return t.UTC().Format("2006-01")
	default:
		return ""
	}
}

// Scope is which budget dimension this Ledger caps.
func (l *Ledger) Scope() Scope { return l.scope }

// OwnerID is the opaque identifier this Ledger's spend is attributed to.
func (l *Ledger) OwnerID() string { return l.ownerID }

// PeriodKey is the calendar bucket this Ledger belongs to (see PeriodKey func).
func (l *Ledger) PeriodKey() string { return l.periodKey }

// Limit is the cap this Ledger enforces.
func (l *Ledger) Limit() Limit { return l.limit }

// Usage is the running total consumed so far.
func (l *Ledger) Usage() Usage { return l.usage }

// Charge adds delta to the running Usage. Amounts must be
// non-negative — Budget only ever accumulates; correcting a mistaken
// charge is a reconciliation concern for whatever calls this, not
// something modeled as a negative charge here.
func (l *Ledger) Charge(delta Usage) error {
	if delta.InputTokens < 0 || delta.OutputTokens < 0 || delta.Cost < 0 {
		return ErrNegativeCharge
	}
	l.usage.InputTokens += delta.InputTokens
	l.usage.OutputTokens += delta.OutputTokens
	l.usage.Cost += delta.Cost
	return nil
}

// Exceeded reports whether any capped dimension (a non-zero Limit
// field) has been met or surpassed by Usage. An uncapped dimension
// (Limit field == 0) never contributes to Exceeded.
func (l *Ledger) Exceeded() bool {
	return capExceeded(l.limit.InputTokens, l.usage.InputTokens) ||
		capExceeded(l.limit.OutputTokens, l.usage.OutputTokens) ||
		capExceeded(int64(l.limit.Cost), int64(l.usage.Cost))
}

func capExceeded(limit, used int64) bool {
	return limit > 0 && used >= limit
}

// Clone returns a deep copy. Limit and Usage are plain value structs
// with no nested reference types, so a shallow struct copy already is
// a full deep copy — unlike execution.Execution.Clone, no field-by-field
// reconstruction is needed.
func (l *Ledger) Clone() *Ledger {
	clone := *l
	return &clone
}
