package gateway

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"
)

// RateLimiter is a Redis-backed fixed-window limiter, satisfying the
// "Redis-backed rate limiting" non-functional requirement. Fixed-window
// (INCR + EXPIRE on a key scoped to the current window) is the
// simplest correct approach and is what's implemented here; it allows
// short bursts right at a window boundary that a sliding-window or
// token-bucket limiter wouldn't — an acceptable trade for this
// milestone's scope, revisit if that burst behavior becomes a problem.
type RateLimiter struct {
	redis  *redis.Client
	limit  int
	window time.Duration
}

// NewRateLimiter builds a limiter allowing `limit` requests per
// `window` per key.
func NewRateLimiter(redisClient *redis.Client, limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{redis: redisClient, limit: limit, window: window}
}

// Allow reports whether key may make another request in the current
// window, incrementing its counter as a side effect. Fails open (returns
// true) on a Redis error — an unreachable rate limiter should degrade
// to "unlimited," not take the whole API down; readiness/health
// checks are where Redis outages should actually surface, not here.
func (l *RateLimiter) Allow(ctx context.Context, key string) (bool, error) {
	windowKey := fmt.Sprintf("ratelimit:%s:%d", key, time.Now().Unix()/int64(l.window.Seconds()))

	count, err := l.redis.Incr(ctx, windowKey).Result()
	if err != nil {
		return true, fmt.Errorf("gateway: rate limiter: %w", err)
	}
	if count == 1 {
		// Only the request that created this window's key needs to set
		// its expiry — every subsequent INCR in the same window is a
		// no-op on TTL.
		if err := l.redis.Expire(ctx, windowKey, l.window).Err(); err != nil {
			return true, fmt.Errorf("gateway: rate limiter: set expiry: %w", err)
		}
	}
	return count <= int64(l.limit), nil
}

// Middleware enforces the limiter keyed by the request's agent ID (set
// by AuthMiddleware, which must run first) or, failing that, remote
// address — an unauthenticated request is still rate-limited, just per
// IP instead of per agent.
func (l *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.RemoteAddr
		if a, ok := AgentFromContext(r.Context()); ok {
			key = a.ID()
		}

		allowed, err := l.Allow(r.Context(), key)
		if err != nil {
			// Logged by the caller's request-logging middleware via the
			// normal response path; fail open per Allow's doc comment.
			next.ServeHTTP(w, r)
			return
		}
		if !allowed {
			w.Header().Set("Retry-After", fmt.Sprintf("%.0f", l.window.Seconds()))
			writeError(w, http.StatusTooManyRequests, "rate limit exceeded")
			return
		}
		next.ServeHTTP(w, r)
	})
}
