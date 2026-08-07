// Command server is the control plane's REST + WebSocket API process.
// It is intentionally thin: all wiring lives in internal/app so it can
// be reused by integration tests without spawning a real process.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/dheeraj7000/control-plane/internal/app"
	"github.com/dheeraj7000/control-plane/internal/config"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "control-plane server: "+err.Error())
		os.Exit(1)
	}
}

func run() error {
	ctx := context.Background()

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	a, err := app.New(ctx, cfg)
	if err != nil {
		return err
	}
	defer func() {
		closeCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()
		if closeErr := a.Close(closeCtx); closeErr != nil {
			a.Logger.Error("error during shutdown cleanup", slog.String("error", closeErr.Error()))
		}
	}()

	a.Logger.Info("starting control plane server",
		slog.String("env", cfg.Env),
		slog.Int("port", cfg.HTTPPort),
	)

	return a.Run(ctx)
}
