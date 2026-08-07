// Package budget tracks input/output tokens, estimated cost, and budget consumption at the execution, daily, and monthly scope. The Policy engine depends on Budget state (e.g. 'deny if execution budget exceeded'), so Budget's interface must exist before Policy's evaluator is wired up.
//
// Scaffolded in Milestone 1 as an empty package to establish the
// repository's domain boundaries; behavior lands in Milestone 4, alongside Policy engine.
package budget
