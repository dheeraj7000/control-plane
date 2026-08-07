package gateway_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/dheeraj7000/control-plane/internal/gateway"
)

func newTestRedis(t *testing.T) *redis.Client {
	t.Helper()
	mr := miniredis.RunT(t) // RunT registers cleanup automatically
	return redis.NewClient(&redis.Options{Addr: mr.Addr()})
}

func TestRateLimiter_AllowsWithinLimit(t *testing.T) {
	client := newTestRedis(t)
	limiter := gateway.NewRateLimiter(client, 3, time.Minute)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		ok, err := limiter.Allow(ctx, "agent-1")
		if err != nil {
			t.Fatalf("Allow() returned error: %v", err)
		}
		if !ok {
			t.Fatalf("Allow() call %d = false, want true (within limit)", i+1)
		}
	}
}

func TestRateLimiter_DeniesOverLimit(t *testing.T) {
	client := newTestRedis(t)
	limiter := gateway.NewRateLimiter(client, 2, time.Minute)
	ctx := context.Background()

	for i := 0; i < 2; i++ {
		if _, err := limiter.Allow(ctx, "agent-1"); err != nil {
			t.Fatal(err)
		}
	}
	ok, err := limiter.Allow(ctx, "agent-1")
	if err != nil {
		t.Fatalf("Allow() returned error: %v", err)
	}
	if ok {
		t.Fatal("Allow() = true on the 3rd call with a limit of 2, want false")
	}
}

func TestRateLimiter_KeysAreIndependent(t *testing.T) {
	client := newTestRedis(t)
	limiter := gateway.NewRateLimiter(client, 1, time.Minute)
	ctx := context.Background()

	if ok, err := limiter.Allow(ctx, "agent-a"); err != nil || !ok {
		t.Fatalf("Allow(agent-a) = %v, %v, want true, nil", ok, err)
	}
	if ok, err := limiter.Allow(ctx, "agent-b"); err != nil || !ok {
		t.Fatalf("Allow(agent-b) = %v, %v, want true, nil (independent key)", ok, err)
	}
}

func TestRateLimiter_Middleware_TooManyRequests(t *testing.T) {
	client := newTestRedis(t)
	limiter := gateway.NewRateLimiter(client, 1, time.Minute)

	handler := limiter.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	req.RemoteAddr = "1.2.3.4:5678"

	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req)
	if rec1.Code != http.StatusOK {
		t.Fatalf("first request status = %d, want 200", rec1.Code)
	}

	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req)
	if rec2.Code != http.StatusTooManyRequests {
		t.Fatalf("second request status = %d, want 429", rec2.Code)
	}
}

func TestRateLimiter_FailsOpenOnRedisError(t *testing.T) {
	// A client pointed at nothing listening simulates Redis being down.
	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"})
	limiter := gateway.NewRateLimiter(client, 1, time.Minute)

	handler := limiter.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status with Redis unreachable = %d, want 200 (fail open)", rec.Code)
	}
}
