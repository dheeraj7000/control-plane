// Package scheduler decides when a queued Execution (or a retry, or a scheduled/cron-triggered Workflow) is dispatched to run, respecting concurrency limits and backoff policy.
//
// Scaffolded in Milestone 1 as an empty package to establish the
// repository's domain boundaries; behavior lands in Milestone 5+ (Gateway and orchestration).
package scheduler
