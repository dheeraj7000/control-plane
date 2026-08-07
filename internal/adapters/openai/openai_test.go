package openai_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dheeraj7000/control-plane/internal/adapters"
	"github.com/dheeraj7000/control-plane/internal/adapters/openai"
)

func TestName(t *testing.T) {
	c := openai.New("", "key", nil)
	if c.Name() != "openai" {
		t.Errorf("Name() = %q, want openai", c.Name())
	}
}

func TestCallModel_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Errorf("Authorization header = %q, want 'Bearer test-key'", got)
		}
		if r.URL.Path != "/chat/completions" {
			t.Errorf("path = %s, want /chat/completions", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"choices": [{"message": {"role": "assistant", "content": "hello there"}}],
			"usage": {"prompt_tokens": 12, "completion_tokens": 5}
		}`))
	}))
	defer srv.Close()

	c := openai.New(srv.URL, "test-key", srv.Client())
	result, err := c.CallModel(context.Background(), adapters.ModelCallRequest{
		Model:    "gpt-5",
		Messages: []adapters.Message{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("CallModel() returned error: %v", err)
	}
	if result.Content != "hello there" {
		t.Errorf("Content = %q, want 'hello there'", result.Content)
	}
	if result.InputTokens != 12 || result.OutputTokens != 5 {
		t.Errorf("tokens = %d/%d, want 12/5", result.InputTokens, result.OutputTokens)
	}
}

func TestCallModel_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error": {"message": "invalid api key"}}`))
	}))
	defer srv.Close()

	c := openai.New(srv.URL, "bad-key", srv.Client())
	_, err := c.CallModel(context.Background(), adapters.ModelCallRequest{Model: "gpt-5"})
	if err == nil || !strings.Contains(err.Error(), "invalid api key") {
		t.Fatalf("CallModel() error = %v, want mentioning 'invalid api key'", err)
	}
}

func TestCallModel_NoChoices(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"choices": []}`))
	}))
	defer srv.Close()

	c := openai.New(srv.URL, "key", srv.Client())
	_, err := c.CallModel(context.Background(), adapters.ModelCallRequest{Model: "gpt-5"})
	if err == nil || !strings.Contains(err.Error(), "no choices") {
		t.Fatalf("CallModel() error = %v, want mentioning 'no choices'", err)
	}
}

func TestCallModel_UnexpectedStatusNoErrorBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := openai.New(srv.URL, "key", srv.Client())
	_, err := c.CallModel(context.Background(), adapters.ModelCallRequest{Model: "gpt-5"})
	if err == nil || !strings.Contains(err.Error(), "500") {
		t.Fatalf("CallModel() error = %v, want mentioning status 500", err)
	}
}

func TestNew_DefaultsBaseURL(t *testing.T) {
	// Can't easily assert the private baseURL field from outside the
	// package; this just documents/locks in that New("", ...) doesn't
	// panic and produces a usable client (a request against it would
	// go to the real API, which is exactly the default we want).
	c := openai.New("", "key", nil)
	if c == nil {
		t.Fatal("New() returned nil")
	}
}
