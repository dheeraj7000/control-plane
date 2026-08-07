// Package workflow owns the Workflow aggregate: the immutable template (steps, dependencies, metadata) that an Execution is instantiated from, plus the Step types (Search, Summarize, Call Tool, Review, Wait, Approval, Model Call).
//
// Scaffolded in Milestone 1 as an empty package to establish the
// repository's domain boundaries; behavior lands in Milestone 2 (Core domain model & state machine).
package workflow
