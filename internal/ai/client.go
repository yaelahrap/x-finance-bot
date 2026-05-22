// Package ai provides AI integration for editorial review, relevance scoring,
// risk filtering, and post rewriting.
//
// Two providers are supported:
//   - "anthropic": Anthropic Messages API (https://api.anthropic.com)
//   - "openai":    OpenAI-compatible Chat Completions API (e.g. 9Router, vLLM, OpenRouter)
package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	// ProviderAnthropic selects the native Anthropic Messages API.
	ProviderAnthropic = "anthropic"
	// ProviderOpenAI selects an OpenAI-compatible Chat Completions endpoint.
	ProviderOpenAI = "openai"

	defaultAnthropicBaseURL = "https://api.anthropic.com"
	anthropicAPIVersion     = "2023-06-01"
	maxTokens               = 4096
)

// Client is a multi-provider AI client supporting Anthropic and
// OpenAI-compatible endpoints.
type Client struct {
	provider   string
	baseURL    string
	apiKey     string
	model      string
	httpClient *http.Client
}

// Config holds the AI client configuration.
type Config struct {
	// Provider selects the API protocol: ProviderAnthropic or ProviderOpenAI.
	Provider string
	// BaseURL is the API root. For Anthropic, defaults to https://api.anthropic.com.
	// For OpenAI-compatible providers, must be set (e.g. https://api.raflyrama.dev/v1).
	BaseURL string
	// APIKey is the bearer token for the chosen provider.
	APIKey string
	// Model is the model identifier (provider-specific).
	Model string
	// HTTPClient is optional; a 60s-timeout client is used when nil.
	HTTPClient *http.Client
}

// NewClient creates a new AI client. Provider defaults to "anthropic" when empty.
func NewClient(cfg Config) *Client {
	provider := cfg.Provider
	if provider == "" {
		provider = ProviderAnthropic
	}

	baseURL := strings.TrimRight(cfg.BaseURL, "/")
	if baseURL == "" && provider == ProviderAnthropic {
		baseURL = defaultAnthropicBaseURL
	}

	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 60 * time.Second}
	}

	return &Client{
		provider:   provider,
		baseURL:    baseURL,
		apiKey:     cfg.APIKey,
		model:      cfg.Model,
		httpClient: httpClient,
	}
}

// Message represents a single message in a chat completion request.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Complete sends a chat completion request and returns the assistant's text response.
func (c *Client) Complete(ctx context.Context, system string, messages []Message) (string, error) {
	switch c.provider {
	case ProviderOpenAI:
		return c.completeOpenAI(ctx, system, messages)
	case ProviderAnthropic:
		return c.completeAnthropic(ctx, system, messages)
	default:
		return "", fmt.Errorf("unsupported AI provider: %q", c.provider)
	}
}

// CompleteJSON sends a request and unmarshals the JSON response into target.
// Markdown code fences are stripped before parsing.
func (c *Client) CompleteJSON(ctx context.Context, system string, messages []Message, target interface{}) error {
	text, err := c.Complete(ctx, system, messages)
	if err != nil {
		return err
	}

	text = stripJSONFences(text)

	if err := json.Unmarshal([]byte(text), target); err != nil {
		return fmt.Errorf("unmarshal JSON response: %w (raw: %.200s)", err, text)
	}
	return nil
}

// --- Anthropic provider ---

type anthropicRequest struct {
	Model     string    `json:"model"`
	MaxTokens int       `json:"max_tokens"`
	System    string    `json:"system,omitempty"`
	Messages  []Message `json:"messages"`
}

type anthropicResponse struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Role    string `json:"role"`
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	StopReason string `json:"stop_reason"`
	Usage      struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

func (c *Client) completeAnthropic(ctx context.Context, system string, messages []Message) (string, error) {
	reqBody := anthropicRequest{
		Model:     c.model,
		MaxTokens: maxTokens,
		System:    system,
		Messages:  messages,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	endpoint := c.baseURL + "/v1/messages"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", anthropicAPIVersion)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("anthropic request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1*1024*1024))
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("anthropic API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var apiResp anthropicResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return "", fmt.Errorf("unmarshal response: %w", err)
	}

	if len(apiResp.Content) == 0 {
		return "", fmt.Errorf("empty response from anthropic")
	}

	return apiResp.Content[0].Text, nil
}

// --- OpenAI-compatible provider ---

type openAIRequest struct {
	Model     string    `json:"model"`
	Messages  []Message `json:"messages"`
	MaxTokens int       `json:"max_tokens,omitempty"`
	Stream    bool      `json:"stream"`
}

type openAIResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index   int     `json:"index"`
		Message Message `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error,omitempty"`
}

func (c *Client) completeOpenAI(ctx context.Context, system string, messages []Message) (string, error) {
	// OpenAI puts the system prompt as the first message with role="system".
	allMessages := make([]Message, 0, len(messages)+1)
	if system != "" {
		allMessages = append(allMessages, Message{Role: "system", Content: system})
	}
	allMessages = append(allMessages, messages...)

	reqBody := openAIRequest{
		Model:     c.model,
		Messages:  allMessages,
		MaxTokens: maxTokens,
		Stream:    false,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	endpoint := c.baseURL + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("openai request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1*1024*1024))
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("openai API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	// Some OpenAI-compatible providers (e.g. 9Router) return SSE-style
	// responses even when stream=false is requested. Detect and decode.
	if isSSE(respBody) {
		return decodeSSE(respBody)
	}

	var apiResp openAIResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return "", fmt.Errorf("unmarshal response: %w", err)
	}

	if apiResp.Error != nil {
		return "", fmt.Errorf("openai API error: %s", apiResp.Error.Message)
	}

	if len(apiResp.Choices) == 0 {
		return "", fmt.Errorf("empty response from openai")
	}

	return apiResp.Choices[0].Message.Content, nil
}

// --- Helpers ---

// isSSE reports whether the response body looks like Server-Sent Events
// (lines beginning with "data: "), which some OpenAI-compatible providers
// return even when stream=false is requested.
func isSSE(body []byte) bool {
	trimmed := bytes.TrimLeft(body, " \t\r\n")
	return bytes.HasPrefix(trimmed, []byte("data:"))
}

// decodeSSE reassembles the assistant's full text content from a stream of
// OpenAI-style SSE chunks. Each "data: {...}" line carries a delta with the
// next content fragment; we concatenate them into the final string.
func decodeSSE(body []byte) (string, error) {
	type sseChoice struct {
		Index int `json:"index"`
		Delta struct {
			Role    string `json:"role,omitempty"`
			Content string `json:"content,omitempty"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason,omitempty"`
	}
	type sseChunk struct {
		Choices []sseChoice `json:"choices"`
	}

	var sb strings.Builder
	for _, line := range bytes.Split(body, []byte("\n")) {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		if !bytes.HasPrefix(line, []byte("data:")) {
			continue
		}
		payload := bytes.TrimSpace(line[len("data:"):])
		if len(payload) == 0 || bytes.Equal(payload, []byte("[DONE]")) {
			continue
		}

		var chunk sseChunk
		if err := json.Unmarshal(payload, &chunk); err != nil {
			continue
		}
		for _, ch := range chunk.Choices {
			sb.WriteString(ch.Delta.Content)
		}
	}

	out := sb.String()
	if out == "" {
		return "", fmt.Errorf("empty SSE response from provider")
	}
	return out, nil
}

// stripJSONFences removes markdown code fences that models sometimes wrap JSON in.
func stripJSONFences(s string) string {
	s = strings.TrimSpace(s)

	if strings.HasPrefix(s, "```json") {
		s = strings.TrimPrefix(s, "```json")
		if idx := strings.LastIndex(s, "```"); idx >= 0 {
			s = s[:idx]
		}
	} else if strings.HasPrefix(s, "```") {
		s = strings.TrimPrefix(s, "```")
		if idx := strings.LastIndex(s, "```"); idx >= 0 {
			s = s[:idx]
		}
	}

	return strings.TrimSpace(s)
}
