package ai

import (
	"context"
	"encoding/json"
	"fmt"
)

// Rewriter provides AI-powered post rewriting into various formats.
type Rewriter struct {
	client *Client
}

// NewRewriter creates a new AI rewriter backed by the given client.
func NewRewriter(client *Client) *Rewriter {
	return &Rewriter{client: client}
}

// Rewrite transforms article content into multiple post formats suitable
// for X/Twitter publishing.
func (r *Rewriter) Rewrite(ctx context.Context, input RewriteInput) (*RewriteOutput, error) {
	inputJSON, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("marshal rewrite input: %w", err)
	}

	var result RewriteOutput
	err = r.client.CompleteJSON(ctx, systemRewriter, []Message{
		{Role: "user", Content: string(inputJSON)},
	}, &result)
	if err != nil {
		return nil, fmt.Errorf("rewrite: %w", err)
	}

	return &result, nil
}
