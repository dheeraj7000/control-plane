// Package auth will own authentication and authorization for the control plane itself (as opposed to Policy, which governs what an Execution/Agent may do). Scaffolded now, behind an interface, so multi-tenant/SSO support can be added later without touching call sites.
//
// Scaffolded in Milestone 1 as an empty package to establish the
// repository's domain boundaries; behavior lands in Post-Phase-1 (hardening / commercial evolution).
package auth
