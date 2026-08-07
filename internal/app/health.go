package app

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// livenessResponse is intentionally minimal: liveness answers "is the
// process alive and able to handle a request", not "are my
// dependencies healthy" — that's readiness's job. Conflating the two
// causes orchestrators to restart a perfectly healthy process just
// because Postgres had a blip.
type livenessResponse struct {
	Status string `json:"status"`
}

type readinessResponse struct {
	Status string            `json:"status"`
	Checks map[string]string `json:"checks"`
}

func (a *App) handleLiveness(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, livenessResponse{Status: "ok"})
}

// handleReadiness pings every infra dependency with a short timeout and
// reports per-dependency status. It returns 503 if any check fails, so
// load balancers / orchestrators stop routing traffic here without
// killing the process.
func (a *App) handleReadiness(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	checks := map[string]string{}
	ready := true

	if err := a.DB.Ping(ctx); err != nil {
		checks["postgres"] = "error: " + err.Error()
		ready = false
	} else {
		checks["postgres"] = "ok"
	}

	if err := a.Redis.Ping(ctx).Err(); err != nil {
		checks["redis"] = "error: " + err.Error()
		ready = false
	} else {
		checks["redis"] = "ok"
	}

	if a.NATS == nil || !a.NATS.IsConnected() {
		checks["nats"] = "unavailable"
		ready = false
	} else {
		checks["nats"] = "ok"
	}

	status := http.StatusOK
	overall := "ready"
	if !ready {
		status = http.StatusServiceUnavailable
		overall = "not_ready"
	}

	writeJSON(w, status, readinessResponse{Status: overall, Checks: checks})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
