package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/dheeraj7000/control-plane/internal/agent"
)

// AgentRepository is the Postgres-backed agent.Repository.
type AgentRepository struct {
	db *pgxpool.Pool
}

// NewAgentRepository builds an AgentRepository over db.
func NewAgentRepository(db *pgxpool.Pool) *AgentRepository {
	return &AgentRepository{db: db}
}

// Create implements agent.Repository.
func (r *AgentRepository) Create(ctx context.Context, a agent.Agent) error {
	toolsJSON, err := json.Marshal(a.AllowedTools())
	if err != nil {
		return fmt.Errorf("storage: marshal allowed_tools: %w", err)
	}
	metaJSON, err := json.Marshal(a.Metadata())
	if err != nil {
		return fmt.Errorf("storage: marshal metadata: %w", err)
	}

	_, err = r.db.Exec(ctx, `
		INSERT INTO agents (id, name, allowed_tools, token_hash, metadata, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, a.ID(), a.Name(), toolsJSON, a.TokenHash(), metaJSON, a.CreatedAt())
	if err != nil {
		if isUniqueViolation(err) {
			return fmt.Errorf("%w: id=%s", agent.ErrAlreadyExists, a.ID())
		}
		return fmt.Errorf("storage: insert agent: %w", err)
	}
	return nil
}

// Get implements agent.Repository.
func (r *AgentRepository) Get(ctx context.Context, id string) (agent.Agent, error) {
	row := r.db.QueryRow(ctx, `
		SELECT name, allowed_tools, token_hash, metadata, created_at FROM agents WHERE id = $1
	`, id)
	return scanAgent(row.Scan, id)
}

// FindByTokenHash implements agent.Repository.
func (r *AgentRepository) FindByTokenHash(ctx context.Context, tokenHash string) (agent.Agent, error) {
	row := r.db.QueryRow(ctx, `
		SELECT id, name, allowed_tools, metadata, created_at FROM agents WHERE token_hash = $1
	`, tokenHash)

	var id, name string
	var toolsJSON, metaJSON []byte
	var createdAt time.Time
	if err := row.Scan(&id, &name, &toolsJSON, &metaJSON, &createdAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return agent.Agent{}, agent.ErrNotFound
		}
		return agent.Agent{}, fmt.Errorf("storage: query agent: %w", err)
	}
	return buildAgent(id, name, toolsJSON, tokenHash, metaJSON, createdAt)
}

// List implements agent.Repository.
func (r *AgentRepository) List(ctx context.Context) ([]agent.Agent, error) {
	rows, err := r.db.Query(ctx, `SELECT id, name, allowed_tools, token_hash, metadata, created_at FROM agents`)
	if err != nil {
		return nil, fmt.Errorf("storage: list agents: %w", err)
	}
	defer rows.Close()

	var out []agent.Agent
	for rows.Next() {
		var id, name, tokenHash string
		var toolsJSON, metaJSON []byte
		var createdAt time.Time
		if err := rows.Scan(&id, &name, &toolsJSON, &tokenHash, &metaJSON, &createdAt); err != nil {
			return nil, fmt.Errorf("storage: scan agent: %w", err)
		}
		a, err := buildAgent(id, name, toolsJSON, tokenHash, metaJSON, createdAt)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("storage: list agents: %w", err)
	}
	return out, nil
}

func scanAgent(scan func(dest ...any) error, id string) (agent.Agent, error) {
	var name, tokenHash string
	var toolsJSON, metaJSON []byte
	var createdAt time.Time
	if err := scan(&name, &toolsJSON, &tokenHash, &metaJSON, &createdAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return agent.Agent{}, fmt.Errorf("%w: id=%s", agent.ErrNotFound, id)
		}
		return agent.Agent{}, fmt.Errorf("storage: query agent: %w", err)
	}
	return buildAgent(id, name, toolsJSON, tokenHash, metaJSON, createdAt)
}

func buildAgent(id, name string, toolsJSON []byte, tokenHash string, metaJSON []byte, createdAt time.Time) (agent.Agent, error) {
	var tools []string
	if err := json.Unmarshal(toolsJSON, &tools); err != nil {
		return agent.Agent{}, fmt.Errorf("storage: unmarshal allowed_tools: %w", err)
	}
	var metadata map[string]string
	if err := json.Unmarshal(metaJSON, &metadata); err != nil {
		return agent.Agent{}, fmt.Errorf("storage: unmarshal metadata: %w", err)
	}
	return agent.Restore(agent.RestoreParams{
		ID: id, Name: name, AllowedTools: tools, TokenHash: tokenHash,
		Metadata: metadata, CreatedAt: createdAt,
	})
}
