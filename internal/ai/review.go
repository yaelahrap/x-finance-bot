package ai

import (
	"context"
	"encoding/json"
	"fmt"
)

// Reviewer provides AI-powered editorial review and scoring.
type Reviewer struct {
	client *Client
}

// NewReviewer creates a new AI reviewer backed by the given client.
func NewReviewer(client *Client) *Reviewer {
	return &Reviewer{client: client}
}

// Review performs a full editorial review of the given input, returning
// scores, risk assessment, and suggested post content.
func (r *Reviewer) Review(ctx context.Context, input ReviewInput) (*ReviewOutput, error) {
	inputJSON, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("marshal review input: %w", err)
	}

	var result ReviewOutput
	err = r.client.CompleteJSON(ctx, systemEditor, []Message{
		{Role: "user", Content: string(inputJSON)},
	}, &result)
	if err != nil {
		return nil, fmt.Errorf("review: %w", err)
	}

	return &result, nil
}

// Score performs relevance scoring only (lighter than full review).
func (r *Reviewer) Score(ctx context.Context, input ScoreInput) (*ScoreOutput, error) {
	inputJSON, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("marshal score input: %w", err)
	}

	var result ScoreOutput
	err = r.client.CompleteJSON(ctx, systemScorer, []Message{
		{Role: "user", Content: string(inputJSON)},
	}, &result)
	if err != nil {
		return nil, fmt.Errorf("score: %w", err)
	}

	return &result, nil
}

// AssessRisk performs risk-only assessment for quick filtering.
func (r *Reviewer) AssessRisk(ctx context.Context, input RiskInput) (*RiskOutput, error) {
	inputJSON, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("marshal risk input: %w", err)
	}

	var result RiskOutput
	err = r.client.CompleteJSON(ctx, systemRiskFilter, []Message{
		{Role: "user", Content: string(inputJSON)},
	}, &result)
	if err != nil {
		return nil, fmt.Errorf("assess risk: %w", err)
	}

	return &result, nil
}
