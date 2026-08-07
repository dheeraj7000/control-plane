// Package id generates unique, prefixed identifiers for domain
// aggregates (e.g. "wf_1b2c3d...", "exec_9f8e7d..."). It lives under
// pkg/ because ID generation has no dependency on control-plane domain
// types and is safe to reuse from the CLI or SDK.
//
// Execution and Workflow constructors deliberately take an id string
// rather than generating one internally — that keeps domain
// constructors pure and trivially testable with fixed IDs. Callers
// (the future API layer, Milestone 5) use New here to produce the ID
// they pass in.
package id

import "github.com/google/uuid"

// New returns a prefix-scoped, globally unique identifier. Prefixes
// make IDs self-describing in logs, URLs, and support tickets — "what
// kind of thing is exec_..." is answerable at a glance, the way
// Stripe's ch_/cus_/pi_ prefixes are.
func New(prefix string) string {
	return prefix + "_" + uuid.NewString()
}
