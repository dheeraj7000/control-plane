// Package adapters defines the two protocol-facing interfaces named in
// the spec — one for tool calls (e.g. MCP), one for model calls (e.g.
// OpenAI) — that normalize a provider's wire protocol into a plain Go
// request/result pair. Each provider gets its own subpackage
// implementing one or both interfaces (internal/adapters/mcp,
// internal/adapters/openai).
//
// Adapters do protocol work and nothing else. Recording
// ToolRequested/ToolExecuted/ToolFailed (or the equivalent for model
// calls) around an adapter call is internal/gateway's job — the same
// boundary internal/execution keeps around internal/events and
// internal/policy keeps around internal/budget: a low-level package
// does its one job and returns a plain result or error; a higher
// orchestration layer decides what that means for state, events, and
// policy.
package adapters

import "context"

// ToolCallRequest is a provider-agnostic tool invocation.
type ToolCallRequest struct {
	Tool string
	Args map[string]any
}

// ToolCallResult is what a tool call produced.
type ToolCallResult struct {
	Output map[string]any
}

// Adapter executes tool calls against one provider's protocol (e.g. MCP).
type Adapter interface {
	Name() string
	ExecuteTool(ctx context.Context, req ToolCallRequest) (ToolCallResult, error)
}

// Message is one turn in a model conversation.
type Message struct {
	Role    string
	Content string
}

// ModelCallRequest is a provider-agnostic model/chat completion request.
type ModelCallRequest struct {
	Model    string
	Messages []Message
}

// ModelCallResult is what a model call produced, including token usage
// for internal/budget to charge.
type ModelCallResult struct {
	Content      string
	InputTokens  int64
	OutputTokens int64
}

// ModelAdapter executes model/chat completions against one provider's
// protocol (e.g. OpenAI).
type ModelAdapter interface {
	Name() string
	CallModel(ctx context.Context, req ModelCallRequest) (ModelCallResult, error)
}
