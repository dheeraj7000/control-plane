package gateway

import (
	"context"
	"net/http"
	"strings"

	"github.com/dheeraj7000/control-plane/internal/agent"
)

type contextKey int

const agentContextKey contextKey = iota

// AgentFromContext returns the Agent authenticated by AuthMiddleware
// for this request, if any.
func AgentFromContext(ctx context.Context) (agent.Agent, bool) {
	a, ok := ctx.Value(agentContextKey).(agent.Agent)
	return a, ok
}

// AuthMiddleware authenticates requests via a bearer token, looking up
// the presented token's hash in repo (see agent.HashToken — the token
// itself is never compared or logged). Requests without a valid token
// are rejected with 401; RegisterAgent's own endpoint must be mounted
// outside this middleware, or authentication would be circular.
func AuthMiddleware(repo agent.Repository) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			token, ok := strings.CutPrefix(authHeader, "Bearer ")
			if !ok || token == "" {
				writeError(w, http.StatusUnauthorized, "missing or malformed Authorization header")
				return
			}

			a, err := repo.FindByTokenHash(r.Context(), agent.HashToken(token))
			if err != nil {
				writeError(w, http.StatusUnauthorized, "invalid token")
				return
			}

			ctx := context.WithValue(r.Context(), agentContextKey, a)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
