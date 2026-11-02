// SPDX-License-Identifier: AGPL-3.0-or-later

// Package ai is the model side of the in-app agent: one OpenAI-compatible
// client, so the same code talks to OpenAI, DeepSeek, a gateway such as
// 9router, or a model served on this machine. It knows nothing about profiles
// or tools; the agent loop owns those and passes a tool schema in.
package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Config is one endpoint. BaseURL is the API root, e.g.
// "https://api.deepseek.com/v1"; the client appends /chat/completions.
type Config struct {
	BaseURL   string `json:"baseUrl"`
	Model     string `json:"model"`
	MaxTokens int    `json:"maxTokens,omitempty"`
}

// Validate rejects a config the first request would fail on anyway, so the
// settings page can say what is missing instead of showing a 404.
func (c Config) Validate() error {
	if strings.TrimSpace(c.BaseURL) == "" {
		return errors.New("base URL is required")
	}
	if !strings.HasPrefix(c.BaseURL, "http://") && !strings.HasPrefix(c.BaseURL, "https://") {
		return errors.New("base URL must start with http:// or https://")
	}
	if strings.TrimSpace(c.Model) == "" {
		return errors.New("model is required")
	}
	return nil
}

// Message is one turn. ToolCalls is set on an assistant turn that asks for
// tools; ToolCallID, Name and Content are set on the tool result turn.
type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	Name       string     `json:"name,omitempty"`
}

// ToolCall is one tool invocation the model asked for.
type ToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

// Tool is one entry of the tool catalog. Parameters is a JSON Schema object.
type Tool struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

type ToolFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

// Response is the part of the completion the agent loop reads.
type Response struct {
	Content   string
	ToolCalls []ToolCall
	Usage     Usage
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
}

// Client talks to one endpoint. It is safe for concurrent use: every call
// builds its own request and the http.Client is shared.
type Client struct {
	cfg  Config
	key  string
	http *http.Client
	// endpoint is Config.BaseURL without a trailing slash; tests set it.
	base string
}

// New validates the config and returns a client. key may be empty for a local
// model that needs none.
func New(cfg Config, key string) (*Client, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &Client{cfg: cfg, key: key, base: strings.TrimRight(cfg.BaseURL, "/"),
		http: &http.Client{Timeout: 3 * time.Minute}}, nil
}

// chatRequest is the wire shape. Tool choice is left "auto" so a model that
// cannot call tools still answers in text.
type chatRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Tools       []Tool    `json:"tools,omitempty"`
	ToolChoice  string    `json:"tool_choice,omitempty"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	Temperature float64   `json:"temperature,omitempty"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content   string     `json:"content"`
			ToolCalls []ToolCall `json:"tool_calls"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage Usage     `json:"usage"`
	Error *apiError `json:"error"`
}

type apiError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
}

// Complete sends the conversation and returns the assistant turn. A provider
// error becomes an error whose text is the provider's own message, because the
// settings page shows it to the user.
func (c *Client) Complete(ctx context.Context, messages []Message, tools []Tool) (Response, error) {
	req := chatRequest{Model: c.cfg.Model, Messages: messages, Tools: tools, MaxTokens: c.cfg.MaxTokens}
	if len(tools) > 0 {
		req.ToolChoice = "auto"
	}
	body, err := json.Marshal(req)
	if err != nil {
		return Response{}, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return Response{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if c.key != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.key)
	}
	resp, err := c.http.Do(httpReq)
	if err != nil {
		return Response{}, fmt.Errorf("model request: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return Response{}, err
	}
	var parsed chatResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return Response{}, fmt.Errorf("model answered %s with a body this client cannot read", resp.Status)
	}
	if parsed.Error != nil && parsed.Error.Message != "" {
		return Response{}, fmt.Errorf("model: %s", parsed.Error.Message)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return Response{}, fmt.Errorf("model answered %s", resp.Status)
	}
	if len(parsed.Choices) == 0 {
		return Response{}, errors.New("model returned no choice")
	}
	choice := parsed.Choices[0]
	return Response{Content: choice.Message.Content, ToolCalls: choice.Message.ToolCalls, Usage: parsed.Usage}, nil
}

// Ping asks the endpoint for one short answer. It is the settings page's
// "Test connection": it proves the URL, the key and the model name together.
func (c *Client) Ping(ctx context.Context) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	resp, err := c.Complete(ctx, []Message{{Role: "user", Content: "Reply with the single word: ready"}}, nil)
	if err != nil {
		return "", err
	}
	answer := strings.TrimSpace(resp.Content)
	if answer == "" {
		return "", errors.New("model answered with no text")
	}
	return answer, nil
}
