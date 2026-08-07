// Package openai implements adapters.ModelAdapter over the OpenAI
// Chat Completions REST API, using net/http directly rather than an
// SDK dependency — the request/response shapes this milestone needs
// are small enough that pulling in a full client library isn't worth
// it. Only the single-turn, non-streaming, non-tool-calling subset of
// the API is implemented; that's what internal/gateway's Model Call
// step handling needs today, and extending this client (streaming,
// function calling, multi-turn history beyond what Messages already
// supports) is additive later without changing the adapters.ModelAdapter
// interface it implements.
package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/dheeraj7000/control-plane/internal/adapters"
)

const defaultBaseURL = "https://api.openai.com/v1"

// Client is an adapters.ModelAdapter backed by the OpenAI (or an
// OpenAI-compatible) Chat Completions endpoint.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// New builds a Client. An empty baseURL defaults to the real OpenAI
// API; tests (and OpenAI-compatible self-hosted gateways) can override
// it. A nil httpClient defaults to http.DefaultClient.
func New(baseURL, apiKey string, httpClient *http.Client) *Client {
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Client{baseURL: baseURL, apiKey: apiKey, httpClient: httpClient}
}

// Name implements adapters.ModelAdapter.
func (c *Client) Name() string { return "openai" }

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
}

type chatResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int64 `json:"prompt_tokens"`
		CompletionTokens int64 `json:"completion_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// CallModel implements adapters.ModelAdapter.
func (c *Client) CallModel(ctx context.Context, req adapters.ModelCallRequest) (adapters.ModelCallResult, error) {
	msgs := make([]chatMessage, len(req.Messages))
	for i, m := range req.Messages {
		msgs[i] = chatMessage{Role: m.Role, Content: m.Content}
	}

	body, err := json.Marshal(chatRequest{Model: req.Model, Messages: msgs})
	if err != nil {
		return adapters.ModelCallResult{}, fmt.Errorf("openai: encode request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return adapters.ModelCallResult{}, fmt.Errorf("openai: build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return adapters.ModelCallResult{}, fmt.Errorf("openai: request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var chatResp chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return adapters.ModelCallResult{}, fmt.Errorf("openai: decode response: %w", err)
	}

	if chatResp.Error != nil {
		return adapters.ModelCallResult{}, fmt.Errorf("openai: %s", chatResp.Error.Message)
	}
	if resp.StatusCode != http.StatusOK {
		return adapters.ModelCallResult{}, fmt.Errorf("openai: unexpected status %d", resp.StatusCode)
	}
	if len(chatResp.Choices) == 0 {
		return adapters.ModelCallResult{}, fmt.Errorf("openai: response had no choices")
	}

	return adapters.ModelCallResult{
		Content:      chatResp.Choices[0].Message.Content,
		InputTokens:  chatResp.Usage.PromptTokens,
		OutputTokens: chatResp.Usage.CompletionTokens,
	}, nil
}

var _ adapters.ModelAdapter = (*Client)(nil)
