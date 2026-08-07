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
	return authMiddleware(repo, false)
}

// AuthMiddlewareWS is AuthMiddleware's exception, used only for `GET
// .../executions/{id}/ws` (see Mount): it accepts the token via a
// `?token=` query parameter in addition to the Authorization header.
// Browsers' native WebSocket API cannot set arbitrary headers during
// the handshake, so the header-only scheme every other route uses
// would make this endpoint impossible to call from the dashboard at
// all. This is a deliberate, narrow trade-off (a query parameter can
// end up in server access logs or a Referer header in ways an
// Authorization header doesn't) accepted for exactly one endpoint
// that structurally cannot avoid it, not a general relaxation of this
// API's auth scheme — every other route still requires the header.
func AuthMiddlewareWS(repo agent.Repository) func(http.Handler) http.Handler {
	return authMiddleware(repo, true)
}

func authMiddleware(repo agent.Repository, allowQueryToken bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
			if (!ok || token == "") && allowQueryToken {
				token, ok = r.URL.Query().Get("token"), r.URL.Query().Has("token")
			}
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
