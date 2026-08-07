// Package mcp implements adapters.Adapter over MCP's Streamable HTTP
// transport: JSON-RPC 2.0 requests POSTed to a single endpoint, for the
// tools/call method specifically.
//
// What's simplified relative to the full MCP spec, deliberately, to
// keep this milestone's scope to "does the adapter translation
// boundary work end to end against a real wire protocol": no
// initialize/capability-negotiation handshake, no tools/list discovery
// (the caller already names the tool), no SSE/streaming responses, no
// session management. A production MCP client needs all of those; this
// client implements just enough of the wire protocol to make one
// tools/call request and parse its result — which is sufficient to
// prove adapters.Adapter's contract against a real (if minimal) MCP
// server, tested against an httptest server that speaks this same
// subset. Hardening this into a full client is future work and doesn't
// change the adapters.Adapter interface this package implements.
package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/dheeraj7000/control-plane/internal/adapters"
)

// Client is an adapters.Adapter backed by one MCP server's Streamable
// HTTP endpoint.
type Client struct {
	endpoint   string
	httpClient *http.Client
}

// New builds a Client against endpoint. A nil httpClient defaults to
// http.DefaultClient.
func New(endpoint string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Client{endpoint: endpoint, httpClient: httpClient}
}

// Name implements adapters.Adapter.
func (c *Client) Name() string { return "mcp" }

type rpcRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int    `json:"id"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type toolCallParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments,omitempty"`
}

type toolCallContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type toolCallResult struct {
	Content []toolCallContent `json:"content"`
	IsError bool              `json:"isError"`
}

// ExecuteTool implements adapters.Adapter by issuing one tools/call
// JSON-RPC request.
func (c *Client) ExecuteTool(ctx context.Context, req adapters.ToolCallRequest) (adapters.ToolCallResult, error) {
	body, err := json.Marshal(rpcRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params:  toolCallParams{Name: req.Tool, Arguments: req.Args},
	})
	if err != nil {
		return adapters.ToolCallResult{}, fmt.Errorf("mcp: encode request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return adapters.ToolCallResult{}, fmt.Errorf("mcp: build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return adapters.ToolCallResult{}, fmt.Errorf("mcp: request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return adapters.ToolCallResult{}, fmt.Errorf("mcp: unexpected status %d", resp.StatusCode)
	}

	var rpcResp rpcResponse
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return adapters.ToolCallResult{}, fmt.Errorf("mcp: decode response: %w", err)
	}
	if rpcResp.Error != nil {
		return adapters.ToolCallResult{}, fmt.Errorf("mcp: %s (code %d)", rpcResp.Error.Message, rpcResp.Error.Code)
	}

	var result toolCallResult
	if err := json.Unmarshal(rpcResp.Result, &result); err != nil {
		return adapters.ToolCallResult{}, fmt.Errorf("mcp: decode result: %w", err)
	}
	if result.IsError {
		text := ""
		if len(result.Content) > 0 {
			text = result.Content[0].Text
		}
		return adapters.ToolCallResult{}, fmt.Errorf("mcp: tool %s returned an error: %s", req.Tool, text)
	}

	output := make(map[string]any, len(result.Content))
	if len(result.Content) > 0 {
		output["text"] = result.Content[0].Text
	}
	return adapters.ToolCallResult{Output: output}, nil
}

var _ adapters.Adapter = (*Client)(nil)
