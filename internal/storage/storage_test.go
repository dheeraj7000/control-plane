// Integration tests in this package require a real Postgres — set
// TEST_DATABASE_URL to run them (see .github/workflows/ci.yml for how
// CI provides one, and the Makefile's `test-integration` target for
// running them locally against `make dev-up`'s Postgres). They're
// skipped, not failed, when that's unset — the unit test suites in
// internal/workflow, internal/execution, internal/agent,
// internal/budget, and internal/events already cover the domain logic
// these repositories delegate to; what's unique to test here is SQL
// column mapping, JSON round-tripping, constraint-violation error
// mapping, and (for events) the atomic per-execution sequence counter
// under real concurrency.
package storage_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/dheeraj7000/control-plane/internal/storage"
)

// TestMain migrates and truncates once per test-binary run, not once
// per test — running this package's tests twice in a row against the
// same persistent database (as opposed to a fresh one CI provisions
// per run) would otherwise hit unique-constraint violations on the
// second run, since per-test uniqueness (see uniqueID) only guards
// against collisions *within* one run.
func TestMain(m *testing.M) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		os.Exit(m.Run()) // every test skips itself; nothing to set up
	}

	if err := storage.Migrate(url); err != nil {
		fmt.Fprintln(os.Stderr, "storage.Migrate() failed:", err)
		os.Exit(1)
	}

	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		fmt.Fprintln(os.Stderr, "pgxpool.New() failed:", err)
		os.Exit(1)
	}
	if _, err := pool.Exec(context.Background(),
		`TRUNCATE workflows, agents, executions, budget_ledgers, events, execution_sequences`,
	); err != nil {
		fmt.Fprintln(os.Stderr, "truncate test tables failed:", err)
		os.Exit(1)
	}
	pool.Close()

	os.Exit(m.Run())
}

func testDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping Postgres integration test")
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatalf("pgxpool.New() returned error: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// uniqueID returns an identifier scoped to the running test, so tests
// in this package don't need per-test cleanup between each other
// within a single run — every test operates on rows nothing else in
// that run could collide with. See TestMain for cross-run isolation.
func uniqueID(t *testing.T, prefix string) string {
	t.Helper()
	return prefix + "-" + t.Name()
}
