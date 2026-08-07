package agent

import (
	"context"
	"errors"
)

// Sentinel errors returned by Repository implementations.
var (
	ErrNotFound      = errors.New("agent: not found")
	ErrAlreadyExists = errors.New("agent: already exists")
)

// Repository persists Agents.
type Repository interface {
	Create(ctx context.Context, a Agent) error
	Get(ctx context.Context, id string) (Agent, error)
	// FindByTokenHash looks up the Agent whose token hashes to
	// tokenHash — the Gateway auth middleware's job is to compute
	// HashToken(presented) and call this, never to iterate agents
	// comparing plaintext tokens itself.
	FindByTokenHash(ctx context.Context, tokenHash string) (Agent, error)
	List(ctx context.Context) ([]Agent, error)
}
