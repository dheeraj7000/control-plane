package mcp_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dheeraj7000/control-plane/internal/adapters"
	"github.com/dheeraj7000/control-plane/internal/adapters/mcp"
)

func TestName(t *testing.T) {
	c := mcp.New("http://example.invalid", nil)
	if c.Name() != "mcp" {
		t.Errorf("Name() = %q, want mcp", c.Name())
	}
}

func TestExecuteTool_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("server: decode request: %v", err)
		}
		if req["method"] != "tools/call" {
			t.Errorf("method = %v, want tools/call", req["method"])
		}
		params := req["params"].(map[string]any)
		if params["name"] != "github.search" {
			t.Errorf("params.name = %v, want github.search", params["name"])
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"jsonrpc": "2.0", "id": 1,
			"result": {"content": [{"type": "text", "text": "found 3 results"}], "isError": false}
		}`))
	}))
	defer srv.Close()

	c := mcp.New(srv.URL, srv.Client())
	result, err := c.ExecuteTool(context.Background(), adapters.ToolCallRequest{
		Tool: "github.search",
		Args: map[string]any{"query": "control plane"},
	})
	if err != nil {
		t.Fatalf("ExecuteTool() returned error: %v", err)
	}
	if result.Output["text"] != "found 3 results" {
		t.Errorf("Output[text] = %v, want 'found 3 results'", result.Output["text"])
	}
}

func TestExecuteTool_ToolLevelError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{
			"jsonrpc": "2.0", "id": 1,
			"result": {"content": [{"type": "text", "text": "tool not found"}], "isError": true}
		}`))
	}))
	defer srv.Close()

	c := mcp.New(srv.URL, srv.Client())
	_, err := c.ExecuteTool(context.Background(), adapters.ToolCallRequest{Tool: "ghost.tool"})
	if err == nil || !strings.Contains(err.Error(), "tool not found") {
		t.Fatalf("ExecuteTool() error = %v, want mentioning 'tool not found'", err)
	}
}

func TestExecuteTool_RPCLevelError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"jsonrpc": "2.0", "id": 1, "error": {"code": -32601, "message": "method not found"}}`))
	}))
	defer srv.Close()

	c := mcp.New(srv.URL, srv.Client())
	_, err := c.ExecuteTool(context.Background(), adapters.ToolCallRequest{Tool: "anything"})
	if err == nil || !strings.Contains(err.Error(), "method not found") {
		t.Fatalf("ExecuteTool() error = %v, want mentioning 'method not found'", err)
	}
}

func TestExecuteTool_NonOKStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := mcp.New(srv.URL, srv.Client())
	_, err := c.ExecuteTool(context.Background(), adapters.ToolCallRequest{Tool: "anything"})
	if err == nil {
		t.Fatal("ExecuteTool() expected an error for a 500 response")
	}
}

func TestExecuteTool_MalformedResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`not json`))
	}))
	defer srv.Close()

	c := mcp.New(srv.URL, srv.Client())
	_, err := c.ExecuteTool(context.Background(), adapters.ToolCallRequest{Tool: "anything"})
	if err == nil {
		t.Fatal("ExecuteTool() expected an error for a malformed response body")
	}
}
