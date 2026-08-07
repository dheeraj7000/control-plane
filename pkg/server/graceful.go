// Package server provides a reusable graceful-shutdown wrapper around
// net/http.Server. It lives under pkg/ (not internal/) because it has
// no dependency on control-plane domain types and is safe to reuse from
// the CLI, SDK, or a future extraction into a standalone module.
package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os/signal"
	"syscall"
	"time"
)

// RunGraceful starts srv, blocks until the process receives SIGINT or
// SIGTERM (or ctx is cancelled), then drains in-flight requests for up
// to shutdownTimeout before forcing close.
func RunGraceful(ctx context.Context, srv *http.Server, logger *slog.Logger, shutdownTimeout time.Duration) error {
	notifyCtx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	serveErr := make(chan error, 1)
	go func() {
		logger.Info("http server listening", slog.String("addr", srv.Addr))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr <- err
			return
		}
		serveErr <- nil
	}()

	select {
	case err := <-serveErr:
		if err != nil {
			return fmt.Errorf("server: listen and serve: %w", err)
		}
		return nil
	case <-notifyCtx.Done():
		logger.Info("shutdown signal received, draining connections",
			slog.Duration("timeout", shutdownTimeout))
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("server: graceful shutdown: %w", err)
	}
	logger.Info("http server shut down cleanly")
	return nil
}
