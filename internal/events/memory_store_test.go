package events_test

import (
	"context"
	"errors"
	"testing"

	"github.com/dheeraj7000/control-plane/internal/events"
)

func TestInMemoryStore_AppendAssignsSequence(t *testing.T) {
	ctx := context.Background()
	store := events.NewInMemoryStore()

	types := []events.EventType{events.ExecutionCreated, events.ExecutionStarted, events.ExecutionCompleted}
	for i, et := range types {
		e, err := events.New("exec-1", et, nil)
		if err != nil {
			t.Fatalf("New() returned error: %v", err)
		}
		stored, err := store.Append(ctx, e)
		if err != nil {
			t.Fatalf("Append() returned error: %v", err)
		}
		if want := uint64(i + 1); stored.Sequence != want {
			t.Errorf("Append() Sequence = %d, want %d", stored.Sequence, want)
		}
	}
}

func TestInMemoryStore_SequenceIsPerExecution(t *testing.T) {
	ctx := context.Background()
	store := events.NewInMemoryStore()

	e1, _ := events.New("exec-a", events.ExecutionCreated, nil)
	e2, _ := events.New("exec-b", events.ExecutionCreated, nil)

	s1, err := store.Append(ctx, e1)
	if err != nil {
		t.Fatalf("Append() returned error: %v", err)
	}
	s2, err := store.Append(ctx, e2)
	if err != nil {
		t.Fatalf("Append() returned error: %v", err)
	}
	if s1.Sequence != 1 || s2.Sequence != 1 {
		t.Errorf("two different executions' first events got sequences %d, %d, want 1, 1", s1.Sequence, s2.Sequence)
	}
}

func TestInMemoryStore_AppendRejectsInvalid(t *testing.T) {
	ctx := context.Background()
	store := events.NewInMemoryStore()

	_, err := store.Append(ctx, events.Event{ExecutionID: "", Type: events.ExecutionCreated})
	if !errors.Is(err, events.ErrEmptyExecutionID) {
		t.Fatalf("Append() error = %v, want ErrEmptyExecutionID", err)
	}

	_, err = store.Append(ctx, events.Event{ExecutionID: "exec-1", Type: "bogus"})
	if !errors.Is(err, events.ErrInvalidEventType) {
		t.Fatalf("Append() error = %v, want ErrInvalidEventType", err)
	}
}

func TestInMemoryStore_ListOrdersBySequence(t *testing.T) {
	ctx := context.Background()
	store := events.NewInMemoryStore()

	for _, et := range []events.EventType{events.ExecutionCreated, events.ExecutionStarted, events.ExecutionCompleted} {
		e, _ := events.New("exec-1", et, nil)
		if _, err := store.Append(ctx, e); err != nil {
			t.Fatalf("Append() returned error: %v", err)
		}
	}

	list, err := store.List(ctx, "exec-1")
	if err != nil {
		t.Fatalf("List() returned error: %v", err)
	}
	if len(list) != 3 {
		t.Fatalf("List() len = %d, want 3", len(list))
	}
	for i, e := range list {
		if e.Sequence != uint64(i+1) {
			t.Errorf("list[%d].Sequence = %d, want %d", i, e.Sequence, i+1)
		}
	}
}

func TestInMemoryStore_ListEmptyExecutionReturnsEmptyNotError(t *testing.T) {
	ctx := context.Background()
	store := events.NewInMemoryStore()
	list, err := store.List(ctx, "ghost")
	if err != nil {
		t.Fatalf("List() returned error: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("List() len = %d, want 0", len(list))
	}
}

func TestInMemoryStore_ListReturnsIndependentCopy(t *testing.T) {
	ctx := context.Background()
	store := events.NewInMemoryStore()
	e, _ := events.New("exec-1", events.BudgetUpdated, map[string]any{events.DataKeyTokenDelta: 100})
	if _, err := store.Append(ctx, e); err != nil {
		t.Fatalf("Append() returned error: %v", err)
	}

	list, err := store.List(ctx, "exec-1")
	if err != nil {
		t.Fatalf("List() returned error: %v", err)
	}
	list[0].Data[events.DataKeyTokenDelta] = 999999

	again, err := store.List(ctx, "exec-1")
	if err != nil {
		t.Fatalf("List() returned error: %v", err)
	}
	if again[0].Data[events.DataKeyTokenDelta] != 100 {
		t.Fatal("mutating a List() result affected the stored event")
	}
}
