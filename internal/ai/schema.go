package ai

import (
	"github.com/raflyramadhan/x-finance-bot/internal/models"
)

// ReviewInput is the structured input sent to Claude for editorial review.
type ReviewInput struct {
	Title    string `json:"title"`
	Content  string `json:"content"`
	Source   string `json:"source"`
	Category string `json:"category"`
	URL      string `json:"url"`
}

// RewriteInput is the structured input sent to Claude for post rewriting.
type RewriteInput struct {
	Title    string `json:"title"`
	Content  string `json:"content"`
	Source   string `json:"source"`
	Category string `json:"category"`
}

// ScoreInput is the structured input sent to Claude for relevance scoring.
type ScoreInput struct {
	Title    string `json:"title"`
	Content  string `json:"content"`
	Source   string `json:"source"`
	Category string `json:"category"`
	URL      string `json:"url"`
}

// RiskInput is the structured input sent to Claude for risk assessment.
type RiskInput struct {
	Title    string `json:"title"`
	Content  string `json:"content"`
	Source   string `json:"source"`
	Category string `json:"category"`
}

// ReviewOutput matches the JSON schema returned by Claude's editor role.
// This maps directly to models.ReviewResult.
type ReviewOutput = models.ReviewResult

// ScoreOutput matches the JSON schema returned by Claude's scorer role.
type ScoreOutput = models.Score

// RewriteOutput matches the JSON schema returned by Claude's rewriter role.
type RewriteOutput = models.RewriteResult

// RiskOutput matches the JSON schema returned by Claude's risk filter role.
type RiskOutput = models.RiskFilter
