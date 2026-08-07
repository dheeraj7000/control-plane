package execution

import "time"

// StepStatus tracks a single step's progress within one Execution. This
// is deliberately a lighter-weight model than the Execution-level State
// graph: it records what happened, but doesn't yet enforce a full
// transition graph of its own — the only rule enforced here is "a
// terminal step can't be modified further". Ordering which step runs
// next based on the Workflow's dependency graph is the Scheduler /
// Execution Manager's job (Milestone 5+), not this package's.
type StepStatus string

// The six statuses a StepRun can be in.
const (
	StepPending   StepStatus = "pending"
	StepRunning   StepStatus = "running"
	StepWaiting   StepStatus = "waiting"
	StepCompleted StepStatus = "completed"
	StepFailed    StepStatus = "failed"
	StepSkipped   StepStatus = "skipped"
)

// IsTerminal reports whether a step in this status can still change.
func (s StepStatus) IsTerminal() bool {
	switch s {
	case StepCompleted, StepFailed, StepSkipped:
		return true
	default:
		return false
	}
}

// isValid reports whether s is one of the six known statuses. Used by
// Restore to catch corrupted persisted data; the live StartStep/
// CompleteStep/etc. path never produces anything else, so this isn't
// needed there.
func (s StepStatus) isValid() bool {
	switch s {
	case StepPending, StepRunning, StepWaiting, StepCompleted, StepFailed, StepSkipped:
		return true
	default:
		return false
	}
}

// StepRun is the per-execution runtime record for one Workflow Step.
type StepRun struct {
	StepID    string
	Status    StepStatus
	Attempt   int
	StartedAt *time.Time
	EndedAt   *time.Time
	Error     string
}

func (s StepRun) clone() StepRun {
	out := s
	if s.StartedAt != nil {
		t := *s.StartedAt
		out.StartedAt = &t
	}
	if s.EndedAt != nil {
		t := *s.EndedAt
		out.EndedAt = &t
	}
	return out
}
