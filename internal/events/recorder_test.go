package events_test

import (
	"context"
	"testing"
	"time"

	"github.com/dheeraj7000/control-plane/internal/events"
)

func TestRecorder_Record(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	store := events.NewInMemoryStore()
	bus := events.NewInMemoryBus()
	rec := events.NewRecorder(store, bus)

	ch, err := bus.Subscribe(ctx, events.Filter{ExecutionID: "exec-1"})
	if err != nil {
		t.Fatalf("Subscribe() returned error: %v", err)
	}

	stored, err := rec.Record(ctx, "exec-1", events.StepCompleted, map[string]any{
		events.DataKeyStepID: "fetch",
	})
	if err != nil {
		t.Fatalf("Record() returned error: %v", err)
	}
	if stored.Sequence != 1 {
		t.Errorf("Record() Sequence = %d, want 1", stored.Sequence)
	}

	// The durable copy must be retrievable via the store...
	list, err := store.List(ctx, "exec-1")
	if err != nil {
		t.Fatalf("List() returned error: %v", err)
	}
	if len(list) != 1 || list[0].ID != stored.ID {
		t.Fatalf("store.List() = %v, want the recorded event", list)
	}

	// ...and delivered live via the bus, with the same Sequence.
	select {
	case got := <-ch:
		if got.ID != stored.ID || got.Sequence != stored.Sequence {
			t.Errorf("bus delivered %+v, want matching %+v", got, stored)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for the recorded event on the bus")
	}
}

func TestRecorder_Record_InvalidInputNotStored(t *testing.T) {
	ctx := context.Background()
	store := events.NewInMemoryStore()
	bus := events.NewInMemoryBus()
	rec := events.NewRecorder(store, bus)

	if _, err := rec.Record(ctx, "", events.StepCompleted, nil); err == nil {
		t.Fatal("Record() with empty execution id should return an error")
	}

	list, err := store.List(ctx, "")
	if err != nil {
		t.Fatalf("List() returned error: %v", err)
	}
	if len(list) != 0 {
		t.Fatal("invalid Record() call should not have reached the store")
	}
}
