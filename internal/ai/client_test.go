// SPDX-License-Identifier: AGPL-3.0-or-later

package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestConfigValidate(t *testing.T) {
	for _, bad := range []Config{{}, {BaseURL: "api.example.com/v1", Model: "m"}, {BaseURL: "https://x/v1"}} {
		if err := bad.Validate(); err == nil {
			t.Fatalf("config %+v validated", bad)
		}
	}
	if err := (Config{BaseURL: "https://api.deepseek.com/v1", Model: "deepseek-chat"}).Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestCompleteSendsToolsAndReadsToolCalls(t *testing.T) {
	var got chatRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer sk-test" {
			t.Errorf("authorization = %q", r.Header.Get("Authorization"))
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"choices":[{"message":{"content":"","tool_calls":[{"id":"c1","type":"function","function":{"name":"open_profile","arguments":"{\"profile\":\"p1\"}"}}]},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":11,"completion_tokens":3}}`))
	}))
	defer srv.Close()
	c, err := New(Config{BaseURL: srv.URL + "/v1", Model: "test-model", MaxTokens: 64}, "sk-test")
	if err != nil {
		t.Fatal(err)
	}
	resp, err := c.Complete(context.Background(), []Message{{Role: "user", Content: "open it"}}, []Tool{{
		Type: "function", Function: ToolFunction{Name: "open_profile", Parameters: json.RawMessage(`{"type":"object"}`)},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if got.ToolChoice != "auto" || len(got.Tools) != 1 || got.Model != "test-model" || got.MaxTokens != 64 {
		t.Fatalf("request = %+v", got)
	}
	if len(resp.ToolCalls) != 1 || resp.ToolCalls[0].Function.Name != "open_profile" {
		t.Fatalf("tool calls = %+v", resp.ToolCalls)
	}
	if resp.Usage.PromptTokens != 11 {
		t.Fatalf("usage = %+v", resp.Usage)
	}
}

func TestCompleteSurfacesProviderError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":{"message":"invalid api key","type":"auth"}}`))
	}))
	defer srv.Close()
	c, _ := New(Config{BaseURL: srv.URL, Model: "m"}, "bad")
	_, err := c.Complete(context.Background(), []Message{{Role: "user", Content: "hi"}}, nil)
	if err == nil || !strings.Contains(err.Error(), "invalid api key") {
		t.Fatalf("err = %v", err)
	}
}

func TestPingNeedsText(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`{"choices":[{"message":{"content":"  ready  "}}]}`))
	}))
	defer srv.Close()
	c, _ := New(Config{BaseURL: srv.URL, Model: "m"}, "")
	answer, err := c.Ping(context.Background())
	if err != nil || answer != "ready" {
		t.Fatalf("answer = %q, err = %v", answer, err)
	}
}
