package agent_test

import (
	"context"
	"errors"
	"testing"

	"github.com/dheeraj7000/control-plane/internal/agent"
)

func TestInMemoryRepository_CreateAndGet(t *testing.T) {
	ctx := context.Background()
	repo := agent.NewInMemoryRepository()
	a, _, err := agent.New("agent-1", "Bot", nil)
	if err != nil {
		t.Fatalf("agent.New() returned error: %v", err)
	}

	if err := repo.Create(ctx, a); err != nil {
		t.Fatalf("Create() returned error: %v", err)
	}
	got, err := repo.Get(ctx, "agent-1")
	if err != nil {
		t.Fatalf("Get() returned error: %v", err)
	}
	if got.ID() != "agent-1" {
		t.Errorf("Get().ID() = %s, want agent-1", got.ID())
	}
}

func TestInMemoryRepository_CreateDuplicateRejected(t *testing.T) {
	ctx := context.Background()
	repo := agent.NewInMemoryRepository()
	a, _, err := agent.New("agent-1", "Bot", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Create(ctx, a); err != nil {
		t.Fatal(err)
	}
	if err := repo.Create(ctx, a); !errors.Is(err, agent.ErrAlreadyExists) {
		t.Fatalf("second Create() error = %v, want ErrAlreadyExists", err)
	}
}

func TestInMemoryRepository_GetMissing(t *testing.T) {
	ctx := context.Background()
	repo := agent.NewInMemoryRepository()
	if _, err := repo.Get(ctx, "ghost"); !errors.Is(err, agent.ErrNotFound) {
		t.Fatalf("Get() error = %v, want ErrNotFound", err)
	}
}

func TestInMemoryRepository_FindByTokenHash(t *testing.T) {
	ctx := context.Background()
	repo := agent.NewInMemoryRepository()
	a, token, err := agent.New("agent-1", "Bot", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Create(ctx, a); err != nil {
		t.Fatal(err)
	}

	found, err := repo.FindByTokenHash(ctx, agent.HashToken(token))
	if err != nil {
		t.Fatalf("FindByTokenHash() returned error: %v", err)
	}
	if found.ID() != "agent-1" {
		t.Errorf("FindByTokenHash().ID() = %s, want agent-1", found.ID())
	}
}

func TestInMemoryRepository_FindByTokenHash_Missing(t *testing.T) {
	ctx := context.Background()
	repo := agent.NewInMemoryRepository()
	if _, err := repo.FindByTokenHash(ctx, "bogus-hash"); !errors.Is(err, agent.ErrNotFound) {
		t.Fatalf("FindByTokenHash() error = %v, want ErrNotFound", err)
	}
}

func TestInMemoryRepository_List(t *testing.T) {
	ctx := context.Background()
	repo := agent.NewInMemoryRepository()
	a1, _, _ := agent.New("agent-1", "Bot 1", nil)
	a2, _, _ := agent.New("agent-2", "Bot 2", nil)
	if err := repo.Create(ctx, a1); err != nil {
		t.Fatal(err)
	}
	if err := repo.Create(ctx, a2); err != nil {
		t.Fatal(err)
	}

	list, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List() returned error: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("List() len = %d, want 2", len(list))
	}
}
