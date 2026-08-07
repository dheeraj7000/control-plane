// Package agent owns the Agent aggregate named in the spec's Domain
// Model section: "Logical identity. Owns credentials, policies,
// budgets, allowed tools. Agents do not own state." — a concept the
// spec's own repository layout never gave a package to (see
// docs/architecture.md's Milestone 4 notes on that gap).
//
// This package is deliberately minimal: identity, an allowed-tools
// list that policy.ToolAllowlistRule can be configured from, and a
// bearer token for Gateway authentication. Full credential management
// (rotation, revocation, a real secrets-manager/Vault/KMS integration)
// remains the open question flagged since Milestone 1 — what's here is
// the minimum reasonable stance ahead of that bigger design landing:
// only a salted hash of the issued token is ever stored, never the
// plaintext.
package agent

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"github.com/dheeraj7000/control-plane/pkg/id"
)

// Sentinel errors.
var (
	ErrEmptyID   = errors.New("agent: id must not be empty")
	ErrEmptyName = errors.New("agent: name must not be empty")
)

// Agent is a logical identity that starts workflow executions and owns
// a set of allowed tools. It is a value type like Workflow — treat it
// as immutable and construct a fresh one (with RotateToken, if a
// caller needs to reissue credentials) rather than mutating fields.
type Agent struct {
	id           string
	name         string
	allowedTools []string
	tokenHash    string
	metadata     map[string]string
	createdAt    time.Time
}

// Option configures optional Agent fields in New, same pattern as
// workflow.Option.
type Option func(*Agent)

// WithMetadata attaches free-form key/value metadata.
func WithMetadata(m map[string]string) Option {
	return func(a *Agent) { a.metadata = copyStringMap(m) }
}

// New registers a new Agent and issues it a bearer token. The
// plaintext token is returned ONLY here and is not recoverable
// afterward — the same "copy it now, we won't show it again" model
// most API-key systems use. Only HashToken(token) is retained on the
// Agent; store the plaintext wherever the caller (Gateway's
// registration handler) hands it back to whoever is registering the
// agent.
func New(agentID, name string, allowedTools []string, opts ...Option) (Agent, string, error) {
	if agentID == "" {
		return Agent{}, "", ErrEmptyID
	}
	if name == "" {
		return Agent{}, "", ErrEmptyName
	}

	token := id.New("tok")
	a := Agent{
		id:           agentID,
		name:         name,
		allowedTools: append([]string(nil), allowedTools...),
		tokenHash:    HashToken(token),
		createdAt:    time.Now().UTC(),
	}
	for _, opt := range opts {
		opt(&a)
	}
	return a, token, nil
}

// HashToken returns the stored form of a plaintext bearer token.
// Exported so a Repository can index Agents by it and a caller (the
// Gateway's auth middleware) can look up an Agent by a presented token
// without this package needing its own "FindByPlaintext" method.
// Lookup-by-exact-hash-match (a map or indexed DB column) doesn't need
// a constant-time comparison the way a byte-by-byte secret compare
// would — it's the same shape as how GitHub/Stripe index hashed API
// keys.
func HashToken(plaintext string) string {
	sum := sha256.Sum256([]byte(plaintext))
	return hex.EncodeToString(sum[:])
}

// ID is this agent's unique identifier.
func (a Agent) ID() string { return a.id }

// Name is the agent's human-readable name.
func (a Agent) Name() string { return a.name }

// AllowedTools returns a copy of the tools this agent may invoke —
// the configuration policy.ToolAllowlistRule is built from.
func (a Agent) AllowedTools() []string { return append([]string(nil), a.allowedTools...) }

// TokenHash is the stored (hashed) form of this agent's bearer token,
// used as a Repository lookup key.
func (a Agent) TokenHash() string { return a.tokenHash }

// Metadata returns a copy; mutating the result does not affect a.
func (a Agent) Metadata() map[string]string { return copyStringMap(a.metadata) }

// CreatedAt is when this agent was registered.
func (a Agent) CreatedAt() time.Time { return a.createdAt }

func copyStringMap(m map[string]string) map[string]string {
	if m == nil {
		return nil
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
