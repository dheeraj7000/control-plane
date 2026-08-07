package events

import (
	"context"
	"sync"
)

// subscriberBufferSize bounds how many unconsumed events a subscriber
// can queue before Publish starts dropping for it. Sized for a
// dashboard tab actively watching one execution, not for a subscriber
// expected to keep up with a firehose of every execution's events.
const subscriberBufferSize = 32

// InMemoryBus is a thread-safe, single-process Bus implementation.
type InMemoryBus struct {
	mu     sync.Mutex
	nextID int
	subs   map[int]subscription
}

type subscription struct {
	filter Filter
	ch     chan Event
}

// NewInMemoryBus returns a Bus with no subscribers.
func NewInMemoryBus() *InMemoryBus {
	return &InMemoryBus{subs: make(map[int]subscription)}
}

// Publish implements Bus. It never blocks: a subscriber whose buffer
// is full simply misses this event. Bus is explicitly best-effort —
// see this package's doc comment — durability is Store's job.
func (b *InMemoryBus) Publish(_ context.Context, e Event) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	for _, sub := range b.subs {
		if sub.filter.ExecutionID != "" && sub.filter.ExecutionID != e.ExecutionID {
			continue
		}
		select {
		case sub.ch <- e.clone():
		default:
			// Slow subscriber: drop rather than block the publisher.
		}
	}
	return nil
}

// Subscribe implements Bus.
func (b *InMemoryBus) Subscribe(ctx context.Context, filter Filter) (<-chan Event, error) {
	ch := make(chan Event, subscriberBufferSize)

	b.mu.Lock()
	subID := b.nextID
	b.nextID++
	b.subs[subID] = subscription{filter: filter, ch: ch}
	b.mu.Unlock()

	go func() {
		<-ctx.Done()
		b.mu.Lock()
		delete(b.subs, subID)
		b.mu.Unlock()
		close(ch)
	}()

	return ch, nil
}
