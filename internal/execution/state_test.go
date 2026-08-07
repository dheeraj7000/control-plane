package execution_test

import (
	"testing"

	"github.com/dheeraj7000/control-plane/internal/execution"
)

func TestIsValid(t *testing.T) {
	valid := []execution.State{
		execution.StateCreated, execution.StateQueued, execution.StateRunning,
		execution.StateWaiting, execution.StatePaused, execution.StateRetrying,
		execution.StateCompleted, execution.StateFailed, execution.StateCancelled,
	}
	for _, s := range valid {
		if !execution.IsValid(s) {
			t.Errorf("IsValid(%s) = false, want true", s)
		}
	}
	if execution.IsValid("bogus") {
		t.Error(`IsValid("bogus") = true, want false`)
	}
}

func TestIsTerminal(t *testing.T) {
	terminal := []execution.State{execution.StateCompleted, execution.StateFailed, execution.StateCancelled}
	for _, s := range terminal {
		if !execution.IsTerminal(s) {
			t.Errorf("IsTerminal(%s) = false, want true", s)
		}
	}

	nonTerminal := []execution.State{
		execution.StateCreated, execution.StateQueued, execution.StateRunning,
		execution.StateWaiting, execution.StatePaused, execution.StateRetrying,
	}
	for _, s := range nonTerminal {
		if execution.IsTerminal(s) {
			t.Errorf("IsTerminal(%s) = true, want false", s)
		}
	}
}

func TestCanTransition(t *testing.T) {
	tests := []struct {
		from, to execution.State
		want     bool
	}{
		{execution.StateCreated, execution.StateQueued, true},
		{execution.StateCreated, execution.StateCancelled, true},
		{execution.StateCreated, execution.StateRunning, false},
		{execution.StateCreated, execution.StateCompleted, false},
		{execution.StateQueued, execution.StateRunning, true},
		{execution.StateRunning, execution.StateWaiting, true},
		{execution.StateRunning, execution.StatePaused, true},
		{execution.StateRunning, execution.StateRetrying, true},
		{execution.StateRunning, execution.StateCompleted, true},
		{execution.StateRunning, execution.StateFailed, true},
		{execution.StateRunning, execution.StateCreated, false},
		{execution.StateWaiting, execution.StateRunning, true},
		{execution.StateWaiting, execution.StateQueued, false},
		{execution.StatePaused, execution.StateRunning, true},
		{execution.StateRetrying, execution.StateRunning, true},
		{execution.StateCompleted, execution.StateRunning, false},
		{execution.StateFailed, execution.StateRunning, false},
		{execution.StateCancelled, execution.StateRunning, false},
	}
	for _, tt := range tests {
		got := execution.CanTransition(tt.from, tt.to)
		if got != tt.want {
			t.Errorf("CanTransition(%s, %s) = %v, want %v", tt.from, tt.to, got, tt.want)
		}
	}
}
