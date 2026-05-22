package models

import "time"

// DraftStatus represents the approval lifecycle of a DraftPost.
type DraftStatus string

const (
	// DraftStatusPending means the draft is awaiting review or approval.
	DraftStatusPending DraftStatus = "pending"
	// DraftStatusApproved means the draft passed review and is ready to publish.
	DraftStatusApproved DraftStatus = "approved"
	// DraftStatusScheduled means the draft is approved and queued to publish at ScheduledAt.
	DraftStatusScheduled DraftStatus = "scheduled"
	// DraftStatusRejected means the draft was rejected and will not be published.
	DraftStatusRejected DraftStatus = "rejected"
	// DraftStatusPublished means the draft has been published to X.
	DraftStatusPublished DraftStatus = "published"
)

// PostType describes the structural form of a draft or published post.
type PostType string

const (
	// PostTypeSingle is a single standalone tweet.
	PostTypeSingle PostType = "single"
	// PostTypeThread is a multi-part thread.
	PostTypeThread PostType = "thread"
	// PostTypeBriefing is a daily/recap briefing line.
	PostTypeBriefing PostType = "briefing"
	// PostTypeAlert is a high-urgency alert post.
	PostTypeAlert PostType = "alert"
)

// DraftPost is a generated, not-yet-published candidate post.
type DraftPost struct {
	// ID is the stable unique identifier (UUID) for the draft.
	ID string `json:"id"`
	// ArticleID optionally references the source Article that produced this draft.
	ArticleID string `json:"article_id,omitempty"`
	// PostType is the structural form of the post.
	PostType PostType `json:"post_type"`
	// Content is the primary text body of the draft (single post or first thread item).
	Content string `json:"content"`
	// ThreadJSON is a JSON-serialized array of thread parts when PostType is thread.
	ThreadJSON string `json:"thread_json,omitempty"`
	// ScoreJSON is a JSON-serialized Score captured at draft time.
	ScoreJSON string `json:"score_json,omitempty"`
	// ReviewJSON is a JSON-serialized ReviewResult captured at draft time.
	ReviewJSON string `json:"review_json,omitempty"`
	// Status is the current approval state of the draft.
	Status DraftStatus `json:"status"`
	// RequiresManualApproval indicates the draft cannot auto-publish without human review.
	RequiresManualApproval bool `json:"requires_manual_approval"`
	// CreatedAt is when the draft was generated.
	CreatedAt time.Time `json:"created_at"`
	// ApprovedAt is when the draft was approved, if applicable.
	ApprovedAt *time.Time `json:"approved_at,omitempty"`
	// ScheduledAt is when the draft is scheduled to publish, if applicable.
	ScheduledAt *time.Time `json:"scheduled_at,omitempty"`
	// PublishedAt is when the draft was published, if applicable.
	PublishedAt *time.Time `json:"published_at,omitempty"`
}

// PublishStatus represents the result of a publish attempt.
type PublishStatus string

const (
	// PublishStatusSuccess indicates the post was published successfully.
	PublishStatusSuccess PublishStatus = "success"
	// PublishStatusFailed indicates the publish attempt failed.
	PublishStatusFailed PublishStatus = "failed"
)

// PublishedPost is the record of a draft that was sent to X.
type PublishedPost struct {
	// ID is the stable unique identifier (UUID) for the published record.
	ID string `json:"id"`
	// DraftID references the originating DraftPost.
	DraftID string `json:"draft_id"`
	// XPostID is the post identifier returned by the X API.
	XPostID string `json:"x_post_id"`
	// Content is the final published text.
	Content string `json:"content"`
	// MediaURLs lists media attachments included with the post.
	MediaURLs []string `json:"media_urls,omitempty"`
	// PublishedAt is when the post went live.
	PublishedAt time.Time `json:"published_at"`
	// Status is the result of the publish attempt.
	Status PublishStatus `json:"status"`
}
