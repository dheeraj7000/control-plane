package storage

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/dheeraj7000/control-plane/internal/budget"
)

// BudgetRepository is the Postgres-backed budget.Repository.
type BudgetRepository struct {
	db *pgxpool.Pool
}

// NewBudgetRepository builds a BudgetRepository over db.
func NewBudgetRepository(db *pgxpool.Pool) *BudgetRepository {
	return &BudgetRepository{db: db}
}

// GetOrCreate implements budget.Repository. Uses INSERT ... ON CONFLICT
// DO NOTHING followed by a SELECT rather than a single upsert, since a
// freshly-created row must keep defaultLimit while an existing row's
// limit must NOT be overwritten by whatever default the caller happens
// to pass this time.
func (r *BudgetRepository) GetOrCreate(ctx context.Context, scope budget.Scope, ownerID, periodKey string, defaultLimit budget.Limit) (*budget.Ledger, error) {
	_, err := r.db.Exec(ctx, `
		INSERT INTO budget_ledgers (scope, owner_id, period_key, input_tokens_limit, output_tokens_limit, cost_limit_micros)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (scope, owner_id, period_key) DO NOTHING
	`, string(scope), ownerID, periodKey, defaultLimit.InputTokens, defaultLimit.OutputTokens, int64(defaultLimit.Cost))
	if err != nil {
		return nil, fmt.Errorf("storage: create budget ledger: %w", err)
	}

	row := r.db.QueryRow(ctx, `
		SELECT input_tokens_limit, output_tokens_limit, cost_limit_micros,
		       input_tokens_used, output_tokens_used, cost_used_micros
		FROM budget_ledgers WHERE scope = $1 AND owner_id = $2 AND period_key = $3
	`, string(scope), ownerID, periodKey)

	var inLimit, outLimit, costLimit, inUsed, outUsed, costUsed int64
	if err := row.Scan(&inLimit, &outLimit, &costLimit, &inUsed, &outUsed, &costUsed); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("storage: budget ledger vanished after insert: scope=%s owner=%s period=%s", scope, ownerID, periodKey)
		}
		return nil, fmt.Errorf("storage: query budget ledger: %w", err)
	}

	l, err := budget.New(scope, ownerID, periodKey, budget.Limit{
		InputTokens: inLimit, OutputTokens: outLimit, Cost: budget.Cost(costLimit),
	})
	if err != nil {
		return nil, err
	}
	// New() always starts at zero Usage; Charge() with the persisted
	// totals reconstructs it — see budget package doc on why Ledger
	// doesn't need its own Restore (Charge already does the job for a
	// value type with no hidden/generated fields, unlike Workflow/
	// Execution/Agent).
	if err := l.Charge(budget.Usage{InputTokens: inUsed, OutputTokens: outUsed, Cost: budget.Cost(costUsed)}); err != nil {
		return nil, fmt.Errorf("storage: reconstruct budget usage: %w", err)
	}
	return l, nil
}

// Save implements budget.Repository.
func (r *BudgetRepository) Save(ctx context.Context, l *budget.Ledger) error {
	usage := l.Usage()
	tag, err := r.db.Exec(ctx, `
		UPDATE budget_ledgers
		SET input_tokens_used = $4, output_tokens_used = $5, cost_used_micros = $6
		WHERE scope = $1 AND owner_id = $2 AND period_key = $3
	`, string(l.Scope()), l.OwnerID(), l.PeriodKey(), usage.InputTokens, usage.OutputTokens, int64(usage.Cost))
	if err != nil {
		return fmt.Errorf("storage: save budget ledger: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("%w: scope=%s owner=%s period=%s", budget.ErrNotFound, l.Scope(), l.OwnerID(), l.PeriodKey())
	}
	return nil
}
