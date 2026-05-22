// Package decision implements the posting decision engine that determines
// whether a reviewed draft should be auto-posted, queued for approval, or skipped.
package decision

import (
	"github.com/raflyramadhan/x-finance-bot/internal/models"
)

// Action represents the decision engine's output.
type Action string

const (
	// ActionAutoPost means the post is safe to publish automatically.
	ActionAutoPost Action = "auto_post"
	// ActionManualApproval means the post needs human review before publishing.
	ActionManualApproval Action = "manual_approval"
	// ActionSkip means the post should not be published.
	ActionSkip Action = "skip"
)

// Result holds the decision and reasoning.
type Result struct {
	Action  Action   `json:"action"`
	Reasons []string `json:"reasons"`
}

// Policy configures the decision engine thresholds.
type Policy struct {
	// MinAutoPostScore is the minimum total score required for auto-posting.
	MinAutoPostScore int
	// MinPostScore is the minimum total score to even consider posting.
	MinPostScore int
	// MinSourceConfidence is the minimum source confidence for auto-posting.
	MinSourceConfidence int
}

// DefaultPolicy returns the default decision policy matching the plan spec.
func DefaultPolicy() Policy {
	return Policy{
		MinAutoPostScore:    42,
		MinPostScore:        28,
		MinSourceConfidence: 7,
	}
}

// Engine evaluates reviewed drafts against the posting policy.
type Engine struct {
	policy Policy
}

// NewEngine creates a decision engine with the given policy.
func NewEngine(policy Policy) *Engine {
	return &Engine{policy: policy}
}

// Decide evaluates a review result and returns the appropriate action.
func (e *Engine) Decide(review *models.ReviewResult) Result {
	var reasons []string

	// High risk always requires manual approval
	if review.RiskLevel == models.RiskLevelHigh {
		reasons = append(reasons, "high risk level")
		return Result{Action: ActionManualApproval, Reasons: reasons}
	}

	// Low source confidence requires manual approval
	if review.Scores.SourceConfidence < e.policy.MinSourceConfidence {
		reasons = append(reasons, "source confidence below threshold")
		return Result{Action: ActionManualApproval, Reasons: reasons}
	}

	// Below minimum score: skip
	if review.Scores.TotalScore < e.policy.MinPostScore {
		reasons = append(reasons, "total score below minimum posting threshold")
		return Result{Action: ActionSkip, Reasons: reasons}
	}

	// High score + safe + no manual approval required: auto-post
	if review.Scores.TotalScore >= e.policy.MinAutoPostScore &&
		review.SafeToAutoPost &&
		!review.RequiresManualApproval {
		reasons = append(reasons, "score meets auto-post threshold, safe to auto-post")
		return Result{Action: ActionAutoPost, Reasons: reasons}
	}

	// Everything else: queue for manual approval
	if review.RequiresManualApproval {
		reasons = append(reasons, "review flagged for manual approval")
	}
	if review.Scores.TotalScore < e.policy.MinAutoPostScore {
		reasons = append(reasons, "score below auto-post threshold")
	}
	if !review.SafeToAutoPost {
		reasons = append(reasons, "not marked safe to auto-post")
	}

	return Result{Action: ActionManualApproval, Reasons: reasons}
}
