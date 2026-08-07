package events

import "context"

// Filter narrows a Subscribe call. A zero-value field means "don't
// filter on this dimension" — an empty ExecutionID subscribes to every
// execution's events.
type Filter struct {
	ExecutionID string
}

// Bus is ephemeral pub/sub fan-out for live event delivery (e.g. to a
// future WebSocket endpoint watching one execution). It is explicitly
// NOT durable — see this package's doc comment — implementations may
// drop events under backpressure rather than block a publisher.
//
// The in-memory implementation below is single-process. A NATS-backed
// implementation is a later milestone's job, once something actually
// needs cross-instance fan-out (multiple server replicas serving
// WebSocket subscribers) — the interface doesn't change either way.
type Bus interface {
	Publish(ctx context.Context, e Event) error
	// Subscribe returns a channel of events matching filter. The
	// channel is closed and the subscription torn down when ctx is
	// done — there's no separate Unsubscribe method, cancel the
	// context you subscribed with instead.
	Subscribe(ctx context.Context, filter Filter) (<-chan Event, error)
}
