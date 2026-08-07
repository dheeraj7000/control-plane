// Package gateway is the ingress boundary of the control plane: it terminates inbound protocol traffic (MCP, provider-native HTTP), authenticates/authorizes the caller, applies rate limiting, and translates requests into internal/adapters calls and Execution commands. It is deliberately kept separate from internal/adapters — gateway is 'what comes in and who's allowed', adapters is 'how we speak a given provider's protocol'.
//
// Scaffolded in Milestone 1 as an empty package to establish the
// repository's domain boundaries; behavior lands in Milestone 5 (Gateway and protocol adapters).
package gateway
