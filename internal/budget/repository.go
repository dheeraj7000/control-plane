package budget

import (
	"context"
	"errors"
)

// ErrNotFound is returned by Save when no matching Ledger was ever
// created via GetOrCreate.
var ErrNotFound = errors.New("budget: not found")

// Repository persists Ledgers. Unlike the Milestone 2 repositories,
// there's no plain Create/Get pair — GetOrCreate matches how ledgers
// actually come into being: nobody pre-registers "the daily ledger for
// 2026-08-07", it's implicitly created the first time an owner incurs
// a charge in a new period.
type Repository interface {
	// GetOrCreate returns the Ledger for (scope, ownerID, periodKey),
	// creating one seeded with defaultLimit if it doesn't exist yet.
	GetOrCreate(ctx context.Context, scope Scope, ownerID, periodKey string, defaultLimit Limit) (*Ledger, error)
	// Save persists a Ledger's current Usage. Returns ErrNotFound if
	// the Ledger wasn't previously returned by GetOrCreate.
	Save(ctx context.Context, l *Ledger) error
}
