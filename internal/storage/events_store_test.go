package storage_test

import (
	"context"
	"sync"
	"testing"

	"github.com/dheeraj7000/control-plane/internal/events"
	"github.com/dheeraj7000/control-plane/internal/storage"
)

func TestEventStore_AppendAssignsSequence(t *testing.T) {
	db := testDB(t)
	store := storage.NewEventStore(db)
	ctx := context.Background()
	execID := uniqueID(t, "exec")

	types := []events.EventType{events.ExecutionCreated, events.ExecutionStarted, events.ExecutionCompleted}
	for i, et := range types {
		e, err := events.New(execID, et, map[string]any{"i": float64(i)})
		if err != nil {
			t.Fatal(err)
		}
		stored, err := store.Append(ctx, e)
		if err != nil {
			t.Fatalf("Append() returned error: %v", err)
		}
		if stored.Sequence != uint64(i+1) {
			t.Errorf("Append() Sequence = %d, want %d", stored.Sequence, i+1)
		}
	}

	list, err := store.List(ctx, execID)
	if err != nil {
		t.Fatalf("List() returned error: %v", err)
	}
	if len(list) != 3 {
		t.Fatalf("List() len = %d, want 3", len(list))
	}
	for i, e := range list {
		if e.Type != types[i] {
			t.Errorf("list[%d].Type = %s, want %s", i, e.Type, types[i])
		}
	}
}

func TestEventStore_ListEmptyExecutionReturnsEmptyNotError(t *testing.T) {
	db := testDB(t)
	store := storage.NewEventStore(db)
	list, err := store.List(context.Background(), uniqueID(t, "ghost"))
	if err != nil {
		t.Fatalf("List() returned error: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("List() len = %d, want 0", len(list))
	}
}

func TestEventStore_AppendRejectsInvalid(t *testing.T) {
	db := testDB(t)
	store := storage.NewEventStore(db)
	if _, err := store.Append(context.Background(), events.Event{ExecutionID: "", Type: events.ExecutionCreated}); err == nil {
		t.Fatal("Append() with empty ExecutionID: expected an error")
	}
}

// TestEventStore_ConcurrentAppendsGetUniqueGaplessSequences is the real
// value-add of an integration test here over the in-memory store's
// mutex-based equivalent: it proves execution_sequences' INSERT ...
// ON CONFLICT DO UPDATE pattern (migrations/000005) is actually race-free
// against genuine concurrent Postgres connections, not just concurrent
// goroutines sharing one Go-level mutex.
func TestEventStore_ConcurrentAppendsGetUniqueGaplessSequences(t *testing.T) {
	db := testDB(t)
	store := storage.NewEventStore(db)
	ctx := context.Background()
	execID := uniqueID(t, "exec")

	const n = 50
	sequences := make([]uint64, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			e, err := events.New(execID, events.StepStarted, nil)
			if err != nil {
				t.Errorf("events.New() returned error: %v", err)
				return
			}
			stored, err := store.Append(ctx, e)
			if err != nil {
				t.Errorf("Append() returned error: %v", err)
				return
			}
			sequences[i] = stored.Sequence
		}(i)
	}
	wg.Wait()

	seen := make(map[uint64]bool, n)
	for _, seq := range sequences {
		if seq == 0 {
			t.Fatal("a goroutine failed to get a sequence assigned (see earlier errors)")
		}
		if seen[seq] {
			t.Fatalf("duplicate sequence %d assigned under concurrent Append", seq)
		}
		seen[seq] = true
	}
	for want := uint64(1); want <= n; want++ {
		if !seen[want] {
			t.Fatalf("sequence %d missing — gap in the counter under concurrent Append", want)
		}
	}
}
