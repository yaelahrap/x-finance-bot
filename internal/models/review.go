package models

// RiskLevel classifies the perceived risk of publishing a draft.
type RiskLevel string

const (
	// RiskLevelLow indicates the content is safe for automatic publishing.
	RiskLevelLow RiskLevel = "low"
	// RiskLevelMedium indicates caution; manual review is recommended.
	RiskLevelMedium RiskLevel = "medium"
	// RiskLevelHigh indicates the content must not auto-post and requires human approval.
	RiskLevelHigh RiskLevel = "high"
)

// ReviewResult is the structured AI review output for a candidate post.
// Its shape mirrors the AI JSON contract used by the review pipeline.
type ReviewResult struct {
	// Approved indicates the reviewer approves the content.
	Approved bool `json:"approved"`
	// SafeToAutoPost indicates the content can be auto-published without human review.
	SafeToAutoPost bool `json:"safe_to_auto_post"`
	// RequiresManualApproval indicates the content must be manually approved before publishing.
	RequiresManualApproval bool `json:"requires_manual_approval"`
	// RiskLevel is the overall risk classification.
	RiskLevel RiskLevel `json:"risk_level"`
	// Category is a free-form topical classification (e.g., "market", "policy").
	Category string `json:"category"`
	// Scores carries the relevance scoring used during review.
	Scores Score `json:"scores"`
	// Issues lists any concerns the reviewer flagged.
	Issues []string `json:"issues"`
	// SuggestedPost is the recommended single-post version of the content.
	SuggestedPost string `json:"suggested_post"`
	// SuggestedThread is the recommended threaded version of the content.
	SuggestedThread []string `json:"suggested_thread"`
	// WhyItMatters explains the relevance to the audience.
	WhyItMatters string `json:"why_it_matters"`
	// SourceNotes captures any notes about the source reliability or context.
	SourceNotes string `json:"source_notes"`
	// PublisherName is the extracted name of the original news publisher (e.g., "Finimize", "Gotrade").
	PublisherName string `json:"publisher_name"`
}

// RewriteResult is the structured AI rewriter output producing different post forms.
type RewriteResult struct {
	// SinglePost is the standalone tweet form.
	SinglePost string `json:"single_post"`
	// ThreadVersion is the multi-part thread form.
	ThreadVersion []string `json:"thread_version"`
	// BriefingLine is the recap/briefing form.
	BriefingLine string `json:"briefing_line"`
	// AlertVersion is the high-urgency alert form, when applicable.
	AlertVersion string `json:"alert_version,omitempty"`
}

// RiskFilter is the structured output of the risk-filtering step.
type RiskFilter struct {
	// RiskLevel is the overall risk classification.
	RiskLevel RiskLevel `json:"risk_level"`
	// RiskReasons lists factors contributing to the risk classification.
	RiskReasons []string `json:"risk_reasons"`
	// RequiresManualApproval indicates the content must be manually approved before publishing.
	RequiresManualApproval bool `json:"requires_manual_approval"`
	// SafeToAutoPost indicates the content can be auto-published without human review.
	SafeToAutoPost bool `json:"safe_to_auto_post"`
}
