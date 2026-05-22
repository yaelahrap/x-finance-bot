// Package ai provides the Anthropic Claude integration for editorial review,
// relevance scoring, risk filtering, and post rewriting.
package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	anthropicAPIURL     = "https://api.anthropic.com/v1/messages"
	anthropicAPIVersion = "2023-06-01"
	maxTokens           = 4096
)

// Client is the Anthropic Claude API client.
type Client struct {
	apiKey     string
	model      string
	httpClient *http.Client
}

// NewClient creates a new Anthropic API client.
func NewClient(apiKey, model string) *Client {
	if model == "" {
		model = "claude-sonnet-4-5"
	}
	return &Client{
		apiKey: apiKey,
		model:  model,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// Message represents a message in the conversation.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Request is the Anthropic Messages API request body.
type Request struct {
	Model     string    `json:"model"`
	MaxTokens int      `json:"max_tokens"`
	System    string    `json:"system,omitempty"`
	Messages  []Message `json:"messages"`
}

// Response is the Anthropic Messages API response.
type Response struct {
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

// Complete sends a message to Claude and returns the text response.
func (c *Client) Complete(ctx context.Context, system string, messages []Message) (string, error) {
	reqBody := Request{
		Model:     c.model,
		MaxTokens: maxTokens,
		System:    system,
		Messages:  messages,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, anthropicAPIURL, bytes.NewReader(bodyBytes))
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

	var apiResp Response
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return "", fmt.Errorf("unmarshal response: %w", err)
	}

	if len(apiResp.Content) == 0 {
		return "", fmt.Errorf("empty response from anthropic")
	}

	return apiResp.Content[0].Text, nil
}

// CompleteJSON sends a message to Claude and unmarshals the JSON response
// into the provided target. Claude is instructed to return JSON only.
func (c *Client) CompleteJSON(ctx context.Context, system string, messages []Message, target interface{}) error {
	text, err := c.Complete(ctx, system, messages)
	if err != nil {
		return err
	}

	// Strip potential markdown code fences
	text = stripJSONFences(text)

	if err := json.Unmarshal([]byte(text), target); err != nil {
		return fmt.Errorf("unmarshal JSON response: %w (raw: %.200s)", err, text)
	}
	return nil
}

// stripJSONFences removes markdown code fences that Claude sometimes wraps JSON in.
func stripJSONFences(s string) string {
	// Trim whitespace
	s = trimSpace(s)

	// Remove ```json ... ``` wrapper
	if len(s) > 7 && s[:7] == "```json" {
		s = s[7:]
		if idx := lastIndex(s, "```"); idx >= 0 {
			s = s[:idx]
		}
	} else if len(s) > 3 && s[:3] == "```" {
		s = s[3:]
		if idx := lastIndex(s, "```"); idx >= 0 {
			s = s[:idx]
		}
	}

	return trimSpace(s)
}

func trimSpace(s string) string {
	start := 0
	for start < len(s) && (s[start] == ' ' || s[start] == '\n' || s[start] == '\r' || s[start] == '\t') {
		start++
	}
	end := len(s)
	for end > start && (s[end-1] == ' ' || s[end-1] == '\n' || s[end-1] == '\r' || s[end-1] == '\t') {
		end--
	}
	return s[start:end]
}

func lastIndex(s, substr string) int {
	for i := len(s) - len(substr); i >= 0; i-- {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
