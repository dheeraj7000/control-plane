package agent_test

import (
	"errors"
	"testing"

	"github.com/dheeraj7000/control-plane/internal/agent"
)

func TestNew_Valid(t *testing.T) {
	a, token, err := agent.New("agent-1", "Research Bot", []string{"github.search"},
		agent.WithMetadata(map[string]string{"team": "platform"}))
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}
	if a.ID() != "agent-1" || a.Name() != "Research Bot" {
		t.Errorf("identity = %s/%s, want agent-1/Research Bot", a.ID(), a.Name())
	}
	if token == "" {
		t.Error("New() returned an empty token")
	}
	if len(a.AllowedTools()) != 1 || a.AllowedTools()[0] != "github.search" {
		t.Errorf("AllowedTools() = %v, want [github.search]", a.AllowedTools())
	}
	if a.Metadata()["team"] != "platform" {
		t.Errorf("Metadata()[team] = %q, want platform", a.Metadata()["team"])
	}
}

func TestNew_TokenHashMatchesHashToken(t *testing.T) {
	a, token, err := agent.New("agent-1", "Bot", nil)
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}
	if a.TokenHash() != agent.HashToken(token) {
		t.Error("stored TokenHash() does not match HashToken(the returned plaintext token)")
	}
}

func TestNew_TokenNotGuessableFromHash(t *testing.T) {
	a, _, err := agent.New("agent-1", "Bot", nil)
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}
	if agent.HashToken("wrong-token") == a.TokenHash() {
		t.Fatal("an unrelated token hashed to the same value — test setup is broken")
	}
}

func TestNew_Errors(t *testing.T) {
	if _, _, err := agent.New("", "name", nil); !errors.Is(err, agent.ErrEmptyID) {
		t.Fatalf("New() error = %v, want ErrEmptyID", err)
	}
	if _, _, err := agent.New("id", "", nil); !errors.Is(err, agent.ErrEmptyName) {
		t.Fatalf("New() error = %v, want ErrEmptyName", err)
	}
}

func TestRestore_RoundTrip(t *testing.T) {
	original, _, err := agent.New("agent-1", "Bot", []string{"tool-a"}, agent.WithMetadata(map[string]string{"k": "v"}))
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}

	restored, err := agent.Restore(agent.RestoreParams{
		ID:           original.ID(),
		Name:         original.Name(),
		AllowedTools: original.AllowedTools(),
		TokenHash:    original.TokenHash(),
		Metadata:     original.Metadata(),
		CreatedAt:    original.CreatedAt(),
	})
	if err != nil {
		t.Fatalf("Restore() returned error: %v", err)
	}
	if restored.TokenHash() != original.TokenHash() {
		t.Error("Restore() did not preserve the original TokenHash")
	}
	if !restored.CreatedAt().Equal(original.CreatedAt()) {
		t.Error("Restore() did not preserve the original CreatedAt")
	}
	if restored.Metadata()["k"] != "v" {
		t.Error("Restore() did not preserve Metadata")
	}
}

func TestRestore_Errors(t *testing.T) {
	if _, err := agent.Restore(agent.RestoreParams{ID: ""}); !errors.Is(err, agent.ErrEmptyID) {
		t.Fatalf("Restore() error = %v, want ErrEmptyID", err)
	}
	if _, err := agent.Restore(agent.RestoreParams{ID: "agent-1", Name: ""}); !errors.Is(err, agent.ErrEmptyName) {
		t.Fatalf("Restore() error = %v, want ErrEmptyName", err)
	}
}

func TestAllowedTools_ReturnsIndependentSlice(t *testing.T) {
	a, _, err := agent.New("agent-1", "Bot", []string{"tool-a"})
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}
	got := a.AllowedTools()
	got[0] = "mutated"
	if a.AllowedTools()[0] != "tool-a" {
		t.Fatal("mutating the returned slice affected the Agent's internal state")
	}
}
