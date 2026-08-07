// Package execution owns the Execution aggregate: the runtime instance of a Workflow, its state machine (Created/Queued/Running/Waiting/Paused/Retrying/Completed/Failed/Cancelled), and every transition rule between those states. Execution is the primary abstraction of the control plane — events, policies, budgets, traces, and approvals all belong to an Execution, never the other way around.
//
// Scaffolded in Milestone 1 as an empty package to establish the
// repository's domain boundaries; behavior lands in Milestone 2 (Core domain model & state machine).
package execution
