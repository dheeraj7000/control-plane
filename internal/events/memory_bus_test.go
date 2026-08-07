package events_test

import (
	"context"
	"testing"
	"time"

	"github.com/dheeraj7000/control-plane/internal/events"
)

func TestInMemoryBus_PublishSubscribe(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bus := events.NewInMemoryBus()
	ch, err := bus.Subscribe(ctx, events.Filter{})
	if err != nil {
		t.Fatalf("Subscribe() returned error: %v", err)
	}

	e, _ := events.New("exec-1", events.ExecutionStarted, nil)
	if err := bus.Publish(ctx, e); err != nil {
		t.Fatalf("Publish() returned error: %v", err)
	}

	select {
	case got := <-ch:
		if got.ExecutionID != "exec-1" || got.Type != events.ExecutionStarted {
			t.Errorf("received event = %+v, unexpected", got)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for published event")
	}
}

func TestInMemoryBus_FilterByExecutionID(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bus := events.NewInMemoryBus()
	chA, err := bus.Subscribe(ctx, events.Filter{ExecutionID: "exec-a"})
	if err != nil {
		t.Fatalf("Subscribe() returned error: %v", err)
	}
	chAll, err := bus.Subscribe(ctx, events.Filter{})
	if err != nil {
		t.Fatalf("Subscribe() returned error: %v", err)
	}

	eA, _ := events.New("exec-a", events.ExecutionStarted, nil)
	eB, _ := events.New("exec-b", events.ExecutionStarted, nil)
	if err := bus.Publish(ctx, eA); err != nil {
		t.Fatal(err)
	}
	if err := bus.Publish(ctx, eB); err != nil {
		t.Fatal(err)
	}

	// chA should see exactly exec-a's event.
	select {
	case got := <-chA:
		if got.ExecutionID != "exec-a" {
			t.Errorf("chA received %s, want exec-a", got.ExecutionID)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting on chA")
	}
	select {
	case got := <-chA:
		t.Fatalf("chA received unexpected second event: %+v", got)
	case <-time.After(50 * time.Millisecond):
	}

	// chAll should see both, in publish order.
	for _, want := range []string{"exec-a", "exec-b"} {
		select {
		case got := <-chAll:
			if got.ExecutionID != want {
				t.Errorf("chAll received %s, want %s", got.ExecutionID, want)
			}
		case <-time.After(time.Second):
			t.Fatalf("timed out waiting for %s on chAll", want)
		}
	}
}

func TestInMemoryBus_ClosesChannelOnContextCancel(t *testing.T) {
	bus := events.NewInMemoryBus()
	ctx, cancel := context.WithCancel(context.Background())
	ch, err := bus.Subscribe(ctx, events.Filter{})
	if err != nil {
		t.Fatalf("Subscribe() returned error: %v", err)
	}

	cancel()

	select {
	case _, ok := <-ch:
		if ok {
			t.Fatal("expected channel to be closed, got a value instead")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for channel to close after context cancel")
	}
}

func TestInMemoryBus_SlowSubscriberDoesNotBlockPublish(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bus := events.NewInMemoryBus()
	// Subscribe but never drain — Publish must not block regardless.
	if _, err := bus.Subscribe(ctx, events.Filter{}); err != nil {
		t.Fatalf("Subscribe() returned error: %v", err)
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 1000; i++ {
			e, _ := events.New("exec-1", events.ExecutionStarted, nil)
			if err := bus.Publish(ctx, e); err != nil {
				t.Errorf("Publish() returned error: %v", err)
				return
			}
		}
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Publish() blocked on a slow/unread subscriber")
	}
}
