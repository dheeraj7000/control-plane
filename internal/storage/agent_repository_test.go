package storage_test

import (
	"context"
	"errors"
	"testing"

	"github.com/dheeraj7000/control-plane/internal/agent"
	"github.com/dheeraj7000/control-plane/internal/storage"
)

func TestAgentRepository_CreateGetRoundTrip(t *testing.T) {
	db := testDB(t)
	repo := storage.NewAgentRepository(db)
	ctx := context.Background()
	id := uniqueID(t, "agent")

	a, token, err := agent.New(id, "Bot", []string{"github.search"}, agent.WithMetadata(map[string]string{"k": "v"}))
	if err != nil {
		t.Fatalf("agent.New() returned error: %v", err)
	}
	if err := repo.Create(ctx, a); err != nil {
		t.Fatalf("Create() returned error: %v", err)
	}

	got, err := repo.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get() returned error: %v", err)
	}
	if got.Name() != "Bot" || len(got.AllowedTools()) != 1 || got.AllowedTools()[0] != "github.search" {
		t.Errorf("Get() = %+v, mismatch", got)
	}
	if got.Metadata()["k"] != "v" {
		t.Errorf("Get().Metadata()[k] = %q, want v", got.Metadata()["k"])
	}

	found, err := repo.FindByTokenHash(ctx, agent.HashToken(token))
	if err != nil {
		t.Fatalf("FindByTokenHash() returned error: %v", err)
	}
	if found.ID() != id {
		t.Errorf("FindByTokenHash().ID() = %s, want %s", found.ID(), id)
	}
}

func TestAgentRepository_CreateDuplicateRejected(t *testing.T) {
	db := testDB(t)
	repo := storage.NewAgentRepository(db)
	ctx := context.Background()
	a, _, err := agent.New(uniqueID(t, "agent"), "Bot", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Create(ctx, a); err != nil {
		t.Fatalf("first Create() returned error: %v", err)
	}
	if err := repo.Create(ctx, a); !errors.Is(err, agent.ErrAlreadyExists) {
		t.Fatalf("second Create() error = %v, want ErrAlreadyExists", err)
	}
}

func TestAgentRepository_GetMissing(t *testing.T) {
	db := testDB(t)
	repo := storage.NewAgentRepository(db)
	if _, err := repo.Get(context.Background(), uniqueID(t, "ghost")); !errors.Is(err, agent.ErrNotFound) {
		t.Fatalf("Get() error = %v, want ErrNotFound", err)
	}
}

func TestAgentRepository_FindByTokenHash_Missing(t *testing.T) {
	db := testDB(t)
	repo := storage.NewAgentRepository(db)
	if _, err := repo.FindByTokenHash(context.Background(), "bogus-"+uniqueID(t, "hash")); !errors.Is(err, agent.ErrNotFound) {
		t.Fatalf("FindByTokenHash() error = %v, want ErrNotFound", err)
	}
}

func TestAgentRepository_List(t *testing.T) {
	db := testDB(t)
	repo := storage.NewAgentRepository(db)
	ctx := context.Background()
	a1, _, _ := agent.New(uniqueID(t, "agent-1"), "Bot 1", nil)
	a2, _, _ := agent.New(uniqueID(t, "agent-2"), "Bot 2", nil)
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
	found := map[string]bool{}
	for _, a := range list {
		found[a.ID()] = true
	}
	if !found[a1.ID()] || !found[a2.ID()] {
		t.Fatalf("List() missing one of the created agents: %v", list)
	}
}
