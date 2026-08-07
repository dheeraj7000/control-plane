package execution

// State is one of the Execution lifecycle states from the spec:
// Created, Queued, Running, Waiting, Paused, Retrying, Completed,
// Failed, Cancelled.
type State string

// The nine lifecycle states from the spec. See transitions below for
// which moves between them are legal.
const (
	StateCreated   State = "created"
	StateQueued    State = "queued"
	StateRunning   State = "running"
	StateWaiting   State = "waiting"
	StatePaused    State = "paused"
	StateRetrying  State = "retrying"
	StateCompleted State = "completed"
	StateFailed    State = "failed"
	StateCancelled State = "cancelled"
)

// transitions is the authoritative state graph. The spec's own diagram
// renders these nine states as a single straight line
// (Retrying -> Completed -> Failed -> Cancelled), which read literally
// would make every execution retry, then complete, then fail, then get
// cancelled — clearly just a linear list of the state names, not a
// real graph. This is the actual graph implied by the rest of the
// spec's language ("every transition must be validated", retries loop
// back to Running, Waiting covers approval/tool-response waits, Paused
// is an explicit human pause). See docs/architecture.md for the
// rendered diagram — keep it in sync with this map by hand; nothing
// generates one from the other.
var transitions = map[State][]State{
	StateCreated:   {StateQueued, StateCancelled},
	StateQueued:    {StateRunning, StateCancelled},
	StateRunning:   {StateWaiting, StatePaused, StateRetrying, StateCompleted, StateFailed, StateCancelled},
	StateWaiting:   {StateRunning, StateFailed, StateCancelled},
	StatePaused:    {StateRunning, StateCancelled},
	StateRetrying:  {StateRunning, StateFailed, StateCancelled},
	StateCompleted: {},
	StateFailed:    {},
	StateCancelled: {},
}

// IsValid reports whether s is one of the nine known states.
func IsValid(s State) bool {
	_, ok := transitions[s]
	return ok
}

// IsTerminal reports whether s has no outgoing transitions.
// Completed, Failed, and Cancelled are the only terminal states.
func IsTerminal(s State) bool {
	next, ok := transitions[s]
	return ok && len(next) == 0
}

// CanTransition reports whether the state graph allows from -> to
// directly. It does not consider `from` being unknown/invalid as
// distinct from simply having no allowed transitions.
func CanTransition(from, to State) bool {
	for _, s := range transitions[from] {
		if s == to {
			return true
		}
	}
	return false
}
