package storage

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/dheeraj7000/control-plane/internal/events"
)

// EventStore is the Postgres-backed events.Store — the durable log
// that's the source of truth per the Milestone 1 architecture decision
// (NATS/events.Bus remains fan-out only; this is durability).
type EventStore struct {
	db *pgxpool.Pool
}

// NewEventStore builds an EventStore over db.
func NewEventStore(db *pgxpool.Pool) *EventStore {
	return &EventStore{db: db}
}

// Append implements events.Store. Sequence assignment (via
// execution_sequences) and the event insert happen in one transaction:
// see migrations/000005 for why the counter table's
// INSERT ... ON CONFLICT DO UPDATE pattern is what makes concurrent
// Appends for the same execution safe.
func (s *EventStore) Append(ctx context.Context, e events.Event) (events.Event, error) {
	if e.ExecutionID == "" {
		return events.Event{}, events.ErrEmptyExecutionID
	}
	if !e.Type.Valid() {
		return events.Event{}, events.ErrInvalidEventType
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return events.Event{}, fmt.Errorf("storage: begin append: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }() // no-op after a successful Commit

	var seq int64
	err = tx.QueryRow(ctx, `
		INSERT INTO execution_sequences (execution_id, next_seq) VALUES ($1, 1)
		ON CONFLICT (execution_id) DO UPDATE SET next_seq = execution_sequences.next_seq + 1
		RETURNING next_seq
	`, e.ExecutionID).Scan(&seq)
	if err != nil {
		return events.Event{}, fmt.Errorf("storage: assign sequence: %w", err)
	}
	e.Sequence = uint64(seq)

	dataJSON, err := json.Marshal(e.Data)
	if err != nil {
		return events.Event{}, fmt.Errorf("storage: marshal event data: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO events (id, execution_id, type, occurred_at, sequence, data)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, e.ID, e.ExecutionID, string(e.Type), e.OccurredAt, seq, dataJSON); err != nil {
		return events.Event{}, fmt.Errorf("storage: insert event: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return events.Event{}, fmt.Errorf("storage: commit append: %w", err)
	}
	return e, nil
}

// List implements events.Store.
func (s *EventStore) List(ctx context.Context, executionID string) ([]events.Event, error) {
	rows, err := s.db.Query(ctx, `
		SELECT id, type, occurred_at, sequence, data
		FROM events WHERE execution_id = $1 ORDER BY sequence ASC
	`, executionID)
	if err != nil {
		return nil, fmt.Errorf("storage: list events: %w", err)
	}
	defer rows.Close()

	out := []events.Event{} // never nil: an execution with no events yet is a valid, empty result, not an error
	for rows.Next() {
		var e events.Event
		var eventType string
		var seq int64
		var dataJSON []byte
		if err := rows.Scan(&e.ID, &eventType, &e.OccurredAt, &seq, &dataJSON); err != nil {
			return nil, fmt.Errorf("storage: scan event: %w", err)
		}
		e.ExecutionID = executionID
		e.Type = events.EventType(eventType)
		e.Sequence = uint64(seq)
		if err := json.Unmarshal(dataJSON, &e.Data); err != nil {
			return nil, fmt.Errorf("storage: unmarshal event data: %w", err)
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("storage: list events: %w", err)
	}
	return out, nil
}
