// Package adapters normalizes provider-specific protocols (MCP, OpenAI, Anthropic, Ollama, ...) into the control plane's canonical execution events. Each provider gets its own subpackage implementing a common Adapter interface; adapters are plugins, never a hard dependency of the core.
//
// Scaffolded in Milestone 1 as an empty package to establish the
// repository's domain boundaries; behavior lands in Milestone 5 (Gateway and protocol adapters).
package adapters
