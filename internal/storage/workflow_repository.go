package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/dheeraj7000/control-plane/internal/workflow"
)

// WorkflowRepository is the Postgres-backed workflow.Repository.
type WorkflowRepository struct {
	db *pgxpool.Pool
}

// NewWorkflowRepository builds a WorkflowRepository over db.
func NewWorkflowRepository(db *pgxpool.Pool) *WorkflowRepository {
	return &WorkflowRepository{db: db}
}

// Create implements workflow.Repository.
func (r *WorkflowRepository) Create(ctx context.Context, wf workflow.Workflow) error {
	stepsJSON, err := json.Marshal(wf.Steps())
	if err != nil {
		return fmt.Errorf("storage: marshal steps: %w", err)
	}
	metaJSON, err := json.Marshal(wf.Metadata())
	if err != nil {
		return fmt.Errorf("storage: marshal metadata: %w", err)
	}

	_, err = r.db.Exec(ctx, `
		INSERT INTO workflows (id, version, name, description, steps, metadata, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, wf.ID(), wf.Version(), wf.Name(), wf.Description(), stepsJSON, metaJSON, wf.CreatedAt())
	if err != nil {
		if isUniqueViolation(err) {
			return fmt.Errorf("%w: id=%s version=%d", workflow.ErrAlreadyExists, wf.ID(), wf.Version())
		}
		return fmt.Errorf("storage: insert workflow: %w", err)
	}
	return nil
}

// Get implements workflow.Repository.
func (r *WorkflowRepository) Get(ctx context.Context, id string, version int) (workflow.Workflow, error) {
	row := r.db.QueryRow(ctx, `
		SELECT name, description, steps, metadata, created_at
		FROM workflows WHERE id = $1 AND version = $2
	`, id, version)

	var name, description string
	var stepsJSON, metaJSON []byte
	var createdAt time.Time
	if err := row.Scan(&name, &description, &stepsJSON, &metaJSON, &createdAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return workflow.Workflow{}, fmt.Errorf("%w: id=%s version=%d", workflow.ErrNotFound, id, version)
		}
		return workflow.Workflow{}, fmt.Errorf("storage: query workflow: %w", err)
	}
	return buildWorkflow(id, version, name, description, stepsJSON, metaJSON, createdAt)
}

// GetLatest implements workflow.Repository.
func (r *WorkflowRepository) GetLatest(ctx context.Context, id string) (workflow.Workflow, error) {
	row := r.db.QueryRow(ctx, `
		SELECT version, name, description, steps, metadata, created_at
		FROM workflows WHERE id = $1 ORDER BY version DESC LIMIT 1
	`, id)

	var version int
	var name, description string
	var stepsJSON, metaJSON []byte
	var createdAt time.Time
	if err := row.Scan(&version, &name, &description, &stepsJSON, &metaJSON, &createdAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return workflow.Workflow{}, fmt.Errorf("%w: id=%s", workflow.ErrNotFound, id)
		}
		return workflow.Workflow{}, fmt.Errorf("storage: query workflow: %w", err)
	}
	return buildWorkflow(id, version, name, description, stepsJSON, metaJSON, createdAt)
}

// List implements workflow.Repository. DISTINCT ON (id) ... ORDER BY
// id, version DESC is the standard Postgres idiom for "latest row per
// group".
func (r *WorkflowRepository) List(ctx context.Context) ([]workflow.Workflow, error) {
	rows, err := r.db.Query(ctx, `
		SELECT DISTINCT ON (id) id, version, name, description, steps, metadata, created_at
		FROM workflows
		ORDER BY id, version DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("storage: list workflows: %w", err)
	}
	defer rows.Close()

	var out []workflow.Workflow
	for rows.Next() {
		var id, name, description string
		var version int
		var stepsJSON, metaJSON []byte
		var createdAt time.Time
		if err := rows.Scan(&id, &version, &name, &description, &stepsJSON, &metaJSON, &createdAt); err != nil {
			return nil, fmt.Errorf("storage: scan workflow: %w", err)
		}
		wf, err := buildWorkflow(id, version, name, description, stepsJSON, metaJSON, createdAt)
		if err != nil {
			return nil, err
		}
		out = append(out, wf)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("storage: list workflows: %w", err)
	}
	return out, nil
}

func buildWorkflow(id string, version int, name, description string, stepsJSON, metaJSON []byte, createdAt time.Time) (workflow.Workflow, error) {
	var steps []workflow.Step
	if err := json.Unmarshal(stepsJSON, &steps); err != nil {
		return workflow.Workflow{}, fmt.Errorf("storage: unmarshal steps: %w", err)
	}
	var metadata map[string]string
	if err := json.Unmarshal(metaJSON, &metadata); err != nil {
		return workflow.Workflow{}, fmt.Errorf("storage: unmarshal metadata: %w", err)
	}

	var opts []workflow.Option
	if description != "" {
		opts = append(opts, workflow.WithDescription(description))
	}
	if len(metadata) > 0 {
		opts = append(opts, workflow.WithMetadata(metadata))
	}
	return workflow.Restore(id, name, version, steps, createdAt, opts...)
}
