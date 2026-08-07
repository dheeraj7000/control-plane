// Package events defines the Event envelope and the EventBus interface (Publish/Subscribe) that every subsystem uses to announce state changes (ExecutionStarted, StepCompleted, PolicyDenied, ...). No subsystem should call another directly when an event fits — see design principle #3.
//
// Scaffolded in Milestone 1 as an empty package to establish the
// repository's domain boundaries; behavior lands in Milestone 3 (Event bus and timeline engine).
package events
