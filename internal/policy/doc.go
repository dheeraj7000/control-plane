// Package policy defines the PolicyEngine interface (Evaluate(ctx, PolicyInput) (Decision, error)) and ships a native evaluator first. Rules can reference agent, workflow, execution, tool, provider, model, budget, time, and environment. The interface is designed so OPA or Cedar backends can be swapped in later without changing callers.
//
// Scaffolded in Milestone 1 as an empty package to establish the
// repository's domain boundaries; behavior lands in Milestone 4 (Policy engine).
package policy
