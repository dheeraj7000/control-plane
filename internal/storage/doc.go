// Package storage holds Postgres-backed repository implementations for the domain aggregates (Execution, Workflow, ...). Earlier milestones develop against in-memory implementations of the same repository interfaces, defined alongside their owning domain package, so Milestone 7 only swaps the implementation, not the contract.
//
// Scaffolded in Milestone 1 as an empty package to establish the
// repository's domain boundaries; behavior lands in Milestone 7 (Persistence and production hardening).
package storage
