package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/dheeraj7000/control-plane/internal/execution"
)

// ExecutionRepository is the Postgres-backed execution.Repository.
type ExecutionRepository struct {
	db *pgxpool.Pool
}

// NewExecutionRepository builds an ExecutionRepository over db.
func NewExecutionRepository(db *pgxpool.Pool) *ExecutionRepository {
	return &ExecutionRepository{db: db}
}

// Create implements execution.Repository.
func (r *ExecutionRepository) Create(ctx context.Context, e *execution.Execution) error {
	historyJSON, stepsJSON, err := marshalExecutionState(e)
	if err != nil {
		return err
	}

	_, err = r.db.Exec(ctx, `
		INSERT INTO executions (id, workflow_id, workflow_version, agent_id, state, created_at, updated_at, history, steps)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, e.ID(), e.WorkflowID(), e.WorkflowVersion(), e.AgentID(), string(e.State()), e.CreatedAt(), e.UpdatedAt(), historyJSON, stepsJSON)
	if err != nil {
		if isUniqueViolation(err) {
			return fmt.Errorf("%w: id=%s", execution.ErrAlreadyExists, e.ID())
		}
		return fmt.Errorf("storage: insert execution: %w", err)
	}
	return nil
}

// Get implements execution.Repository.
func (r *ExecutionRepository) Get(ctx context.Context, id string) (*execution.Execution, error) {
	row := r.db.QueryRow(ctx, `
		SELECT workflow_id, workflow_version, agent_id, state, created_at, updated_at, history, steps
		FROM executions WHERE id = $1
	`, id)

	var workflowID, agentID, state string
	var workflowVersion int
	var createdAt, updatedAt time.Time
	var historyJSON, stepsJSON []byte
	if err := row.Scan(&workflowID, &workflowVersion, &agentID, &state, &createdAt, &updatedAt, &historyJSON, &stepsJSON); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("%w: id=%s", execution.ErrNotFound, id)
		}
		return nil, fmt.Errorf("storage: query execution: %w", err)
	}
	return buildExecution(id, workflowID, workflowVersion, agentID, state, createdAt, updatedAt, historyJSON, stepsJSON)
}

// Update implements execution.Repository.
func (r *ExecutionRepository) Update(ctx context.Context, e *execution.Execution) error {
	historyJSON, stepsJSON, err := marshalExecutionState(e)
	if err != nil {
		return err
	}

	tag, err := r.db.Exec(ctx, `
		UPDATE executions
		SET workflow_id = $2, workflow_version = $3, agent_id = $4, state = $5,
		    updated_at = $6, history = $7, steps = $8
		WHERE id = $1
	`, e.ID(), e.WorkflowID(), e.WorkflowVersion(), e.AgentID(), string(e.State()), e.UpdatedAt(), historyJSON, stepsJSON)
	if err != nil {
		return fmt.Errorf("storage: update execution: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("%w: id=%s", execution.ErrNotFound, e.ID())
	}
	return nil
}

// List implements execution.Repository.
func (r *ExecutionRepository) List(ctx context.Context, filter execution.ListFilter) ([]*execution.Execution, error) {
	query := `SELECT id, workflow_id, workflow_version, agent_id, state, created_at, updated_at, history, steps FROM executions WHERE 1=1`
	var args []any
	if filter.WorkflowID != "" {
		args = append(args, filter.WorkflowID)
		query += fmt.Sprintf(" AND workflow_id = $%d", len(args))
	}
	if filter.State != "" {
		args = append(args, string(filter.State))
		query += fmt.Sprintf(" AND state = $%d", len(args))
	}

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("storage: list executions: %w", err)
	}
	defer rows.Close()

	var out []*execution.Execution
	for rows.Next() {
		var id, workflowID, agentID, state string
		var workflowVersion int
		var createdAt, updatedAt time.Time
		var historyJSON, stepsJSON []byte
		if err := rows.Scan(&id, &workflowID, &workflowVersion, &agentID, &state, &createdAt, &updatedAt, &historyJSON, &stepsJSON); err != nil {
			return nil, fmt.Errorf("storage: scan execution: %w", err)
		}
		e, err := buildExecution(id, workflowID, workflowVersion, agentID, state, createdAt, updatedAt, historyJSON, stepsJSON)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("storage: list executions: %w", err)
	}
	return out, nil
}

func marshalExecutionState(e *execution.Execution) (historyJSON, stepsJSON []byte, err error) {
	historyJSON, err = json.Marshal(e.History())
	if err != nil {
		return nil, nil, fmt.Errorf("storage: marshal history: %w", err)
	}
	stepsJSON, err = json.Marshal(e.StepRuns())
	if err != nil {
		return nil, nil, fmt.Errorf("storage: marshal steps: %w", err)
	}
	return historyJSON, stepsJSON, nil
}

func buildExecution(id, workflowID string, workflowVersion int, agentID, state string, createdAt, updatedAt time.Time, historyJSON, stepsJSON []byte) (*execution.Execution, error) {
	var history []execution.Transition
	if err := json.Unmarshal(historyJSON, &history); err != nil {
		return nil, fmt.Errorf("storage: unmarshal history: %w", err)
	}
	var steps map[string]execution.StepRun
	if err := json.Unmarshal(stepsJSON, &steps); err != nil {
		return nil, fmt.Errorf("storage: unmarshal steps: %w", err)
	}

	return execution.Restore(execution.RestoreParams{
		ID:              id,
		WorkflowID:      workflowID,
		WorkflowVersion: workflowVersion,
		AgentID:         agentID,
		State:           execution.State(state),
		CreatedAt:       createdAt,
		UpdatedAt:       updatedAt,
		History:         history,
		Steps:           steps,
	})
}
