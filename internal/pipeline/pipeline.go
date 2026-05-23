package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/raflyramadhan/x-finance-bot/internal/ai"
	"github.com/raflyramadhan/x-finance-bot/internal/decision"
	"github.com/raflyramadhan/x-finance-bot/internal/fetcher"
	cardgen "github.com/raflyramadhan/x-finance-bot/internal/image"
	"github.com/raflyramadhan/x-finance-bot/internal/models"
	"github.com/raflyramadhan/x-finance-bot/internal/publisher"
	"github.com/raflyramadhan/x-finance-bot/internal/storage"
)

// staggerInterval controls how far apart non-urgent posts are scheduled in Buffer
// when the bot routes them via BufferModeCustom.
const staggerInterval = 1 * time.Hour

type Orchestrator struct {
	store       storage.Storage
	reviewer    *ai.Reviewer
	engine      *decision.Engine
	publisher   publisher.Publisher
	r2Client    *storage.R2Client
	logger      *slog.Logger
	cmcAPIKey   string
	postingMode string
	httpClient  *http.Client

	// staggerMu protects nextStagger to prevent two concurrent ticks from
	// scheduling overlapping slots.
	staggerMu   sync.Mutex
	nextStagger time.Time
}

func NewOrchestrator(
	store storage.Storage,
	reviewer *ai.Reviewer,
	engine *decision.Engine,
	pub publisher.Publisher,
	r2Client *storage.R2Client,
	logger *slog.Logger,
	cmcAPIKey string,
	postingMode string,
) *Orchestrator {
	return &Orchestrator{
		store:       store,
		reviewer:    reviewer,
		engine:      engine,
		publisher:   pub,
		r2Client:    r2Client,
		logger:      logger,
		cmcAPIKey:   cmcAPIKey,
		postingMode: postingMode,
		httpClient:  &http.Client{Timeout: 30 * time.Second},
	}
}

func (o *Orchestrator) ProcessNewsSources(ctx context.Context) error {
	o.logger.Info("starting ProcessNewsSources cycle")
	sources, err := o.store.GetEnabledSources(ctx)
	if err != nil {
		return fmt.Errorf("get enabled sources: %w", err)
	}

	for _, src := range sources {
		if src.Type != "rss" && src.Type != "aggregator" {
			continue
		}

		o.logger.Info("polling news source", "name", src.Name, "type", src.Type)

		var f fetcher.Fetcher
		if src.Type == "rss" {
			f = fetcher.NewRSSFetcher(src, o.httpClient)
		} else {
			f = fetcher.NewGoogleNewsFetcher(src, o.httpClient)
		}

		articles, err := f.Fetch(ctx)
		if err != nil {
			o.logger.Error("failed to fetch from source", "source", src.Name, "error", err)
			continue
		}

		o.logger.Info("fetched articles", "source", src.Name, "count", len(articles))

		for _, art := range articles {
			if err := o.processArticle(ctx, src, art); err != nil {
				o.logger.Error("failed to process article", "title", art.Title, "error", err)
			}
		}
	}

	return nil
}

func (o *Orchestrator) ProcessBIPulse(ctx context.Context) error {
	o.logger.Info("starting ProcessBIPulse cycle")
	sources, err := o.store.GetEnabledSources(ctx)
	if err != nil {
		return fmt.Errorf("get enabled sources: %w", err)
	}

	var biSource *models.Source
	for _, s := range sources {
		if s.ID == "bi-usdidr" {
			biSource = &s
			break
		}
	}

	if biSource == nil {
		return fmt.Errorf("BI JISDOR source (bi-usdidr) not found or not enabled")
	}

	f := fetcher.NewBIFetcher(*biSource, o.httpClient, o.store)
	articles, err := f.Fetch(ctx)
	if err != nil {
		return fmt.Errorf("bi fetch: %w", err)
	}

	for _, art := range articles {
		if err := o.processArticle(ctx, *biSource, art); err != nil {
			o.logger.Error("failed to process BI article", "error", err)
		}
	}

	return nil
}

func (o *Orchestrator) ProcessBMKGAlerts(ctx context.Context) error {
	o.logger.Info("starting ProcessBMKGAlerts cycle")
	sources, err := o.store.GetEnabledSources(ctx)
	if err != nil {
		return fmt.Errorf("get enabled sources: %w", err)
	}

	var bmkgSource *models.Source
	for _, s := range sources {
		if s.ID == "bmkg-gempa" {
			bmkgSource = &s
			break
		}
	}

	if bmkgSource == nil {
		return fmt.Errorf("BMKG source (bmkg-gempa) not found or not enabled")
	}

	f := fetcher.NewBMKGFetcher(*bmkgSource, o.httpClient)
	articles, err := f.Fetch(ctx)
	if err != nil {
		return fmt.Errorf("bmkg fetch: %w", err)
	}

	for _, art := range articles {
		if err := o.processArticle(ctx, *bmkgSource, art); err != nil {
			o.logger.Error("failed to process BMKG article", "error", err)
		}
	}

	return nil
}

// PublishDraftNow publishes a single draft immediately, bypassing scheduling.
// Returns the published record. Caller is responsible for ensuring the draft
// is in an appropriate state (approved or scheduled) before calling.
func (o *Orchestrator) PublishDraftNow(ctx context.Context, draftID string) (*models.PublishedPost, error) {
	d, err := o.store.GetDraftByID(ctx, draftID)
	if err != nil {
		return nil, fmt.Errorf("get draft: %w", err)
	}
	if d == nil {
		return nil, fmt.Errorf("draft %s not found", draftID)
	}
	if d.Status == models.DraftStatusPublished {
		return nil, fmt.Errorf("draft %s already published", draftID)
	}
	if d.Status == models.DraftStatusRejected {
		return nil, fmt.Errorf("draft %s is rejected", draftID)
	}

	if err := o.publishDraft(ctx, *d); err != nil {
		return nil, err
	}

	return o.store.GetPublishedByDraftID(ctx, draftID)
}

// ProcessScheduledDrafts publishes drafts whose scheduled_at is at or before now.
//
// Only "scheduled" drafts are flushed here; "approved" drafts intentionally stay
// in the queue waiting for the user to either Schedule or Publish-now from the
// dashboard. Auto-flushing all approved drafts would (a) violate the UX contract
// and (b) burst-blast Buffer's API into a 429.
//
// To stay within Buffer's rate limits even when many scheduled drafts come due
// at once, we cap the per-tick batch and add a small inter-call delay.
// Failures are logged but do not abort the loop so a single bad draft cannot
// block the rest of the queue.
func (o *Orchestrator) ProcessScheduledDrafts(ctx context.Context) error {
	const (
		maxPerTick     = 5
		interCallDelay = 1500 * time.Millisecond
	)

	now := time.Now().UTC()
	due, err := o.store.GetDueScheduledDrafts(ctx, now)
	if err != nil {
		return fmt.Errorf("get due scheduled drafts: %w", err)
	}
	if len(due) == 0 {
		return nil
	}

	batch := due
	if len(batch) > maxPerTick {
		batch = batch[:maxPerTick]
	}

	o.logger.Info("publishing scheduled drafts",
		"due", len(due), "publishing", len(batch), "deferred", len(due)-len(batch))

	for i, d := range batch {
		if err := o.publishDraft(ctx, d); err != nil {
			o.logger.Error("publish failed", "draft_id", d.ID, "error", err)
			// If Buffer rate-limits us, stop the whole batch; further calls in
			// the same window will also fail and burn quota.
			if publisher.IsRateLimited(err) {
				o.logger.Warn("buffer rate limited, aborting remaining batch",
					"remaining", len(batch)-i-1)
				return nil
			}
		}
		// Throttle between calls so we don't exceed Buffer's per-window rate limit.
		// The last iteration skips the sleep.
		if i < len(batch)-1 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(interCallDelay):
			}
		}
	}
	return nil
}

// errNotBufferPublisher signals that publishViaBuffer cannot route this draft
// because the configured publisher is not a *publisher.BufferClient. Callers
// fall back to the generic Publisher interface for non-Buffer providers.
var errNotBufferPublisher = fmt.Errorf("publisher is not a buffer client")

// classifyDraft picks the Buffer publish mode and (for custom mode) a target
// scheduledAt for a draft, based on its category, post type, risk level, and
// AI score.
//
// Tiers:
//   1. emergency / alert       -> BufferModeNow         (publish immediately, bypass queue)
//   2. low risk + high score   -> BufferModeQueue       (let Buffer's posting schedule decide)
//   3. everything else lulus   -> BufferModeCustom      (staggered 1h apart by the bot)
func (o *Orchestrator) classifyDraft(d models.DraftPost) (publisher.BufferMode, time.Time) {
	review := parseReviewJSON(d.ReviewJSON)

	if d.PostType == models.PostTypeAlert ||
		strings.EqualFold(review.Category, "emergency") {
		return publisher.BufferModeNow, time.Time{}
	}

	if review.RiskLevel == models.RiskLevelLow && review.Scores.TotalScore >= 42 {
		return publisher.BufferModeQueue, time.Time{}
	}

	return publisher.BufferModeCustom, o.reserveStaggerSlot()
}

// reserveStaggerSlot returns the next available staggered slot and advances
// the cursor so subsequent calls return distinct, monotonically-increasing
// times. The cursor is anchored at max(now+5m, lastSlot+staggerInterval).
func (o *Orchestrator) reserveStaggerSlot() time.Time {
	o.staggerMu.Lock()
	defer o.staggerMu.Unlock()

	earliest := time.Now().UTC().Add(5 * time.Minute)
	next := o.nextStagger
	if next.Before(earliest) {
		next = earliest
	}
	o.nextStagger = next.Add(staggerInterval)
	return next
}

// publishViaBuffer dispatches a draft through the Buffer GraphQL API using the
// tier classification. If the configured publisher is not a *BufferClient,
// it returns errNotBufferPublisher so callers can fall back to the generic
// Publisher interface.
func (o *Orchestrator) publishViaBuffer(ctx context.Context, d models.DraftPost, imageURL string) (*publisher.PublishResult, error) {
	bufClient, ok := o.publisher.(*publisher.BufferClient)
	if !ok {
		return nil, errNotBufferPublisher
	}

	mode, scheduledAt := o.classifyDraft(d)

	o.logger.Info("buffer routing decision",
		"draft_id", d.ID,
		"post_type", d.PostType,
		"mode", mode,
		"scheduled_at", scheduledAt.Format(time.RFC3339),
	)

	return bufClient.PublishWithOptions(ctx, d.Content, publisher.BufferPublishOptions{
		Mode:        mode,
		ScheduledAt: scheduledAt,
		ImageURL:    imageURL,
	})
}

// parseReviewJSON best-effort decodes a draft's stored review_json into a
// ReviewResult. Errors yield a zero-value result so the caller can apply
// conservative defaults rather than crashing.
func parseReviewJSON(raw string) models.ReviewResult {
	var r models.ReviewResult
	if raw == "" {
		return r
	}
	_ = json.Unmarshal([]byte(raw), &r)
	return r
}

// downloadMediaBytes fetches the image data from a public URL.
func (o *Orchestrator) downloadMediaBytes(ctx context.Context, mediaURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, mediaURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create download request: %w", err)
	}

	resp, err := o.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download request execution: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download failed (status %d)", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

// publishDraft sends a draft to the publisher and records the result.
func (o *Orchestrator) publishDraft(ctx context.Context, d models.DraftPost) error {
	var publishRes *publisher.PublishResult
	var err error

	var mediaIDs []string
	if d.MediaURL != "" {
		// XClient requires binary upload; Buffer (and others) accept a public URL directly.
		if xClient, ok := o.publisher.(*publisher.XClient); ok {
			o.logger.Info("downloading media for X upload", "url", d.MediaURL)
			mediaBytes, dlErr := o.downloadMediaBytes(ctx, d.MediaURL)
			if dlErr != nil {
				o.logger.Error("failed to download media, falling back to text-only publish", "error", dlErr)
			} else {
				uploader := publisher.NewMediaUploader(xClient)
				o.logger.Info("uploading media to X")
				mediaID, upErr := uploader.UploadBytes(ctx, mediaBytes, "card.png")
				if upErr != nil {
					o.logger.Error("failed to upload media to X, falling back to text-only publish", "error", upErr)
				} else {
					o.logger.Info("media uploaded to X successfully", "media_id", mediaID)
					mediaIDs = []string{mediaID}
				}
			}
		} else {
			// Buffer and other publishers: pass the public R2 URL directly.
			o.logger.Info("attaching media URL for publisher", "url", d.MediaURL)
			mediaIDs = []string{d.MediaURL}
		}
	}

	if len(mediaIDs) > 0 {
		publishRes, err = o.publishViaBuffer(ctx, d, mediaIDs[0])
		// Only fall back to the generic Publisher interface when the configured
		// publisher is not a Buffer client. Real Buffer errors (429, GraphQL,
		// network) must propagate so the batch loop can detect rate limits and
		// stop hammering the API.
		if err == errNotBufferPublisher {
			publishRes, err = o.publisher.PublishWithMedia(ctx, d.Content, mediaIDs)
		}
	} else {
		publishRes, err = o.publishViaBuffer(ctx, d, "")
		if err == errNotBufferPublisher {
			publishRes, err = o.publisher.PublishText(ctx, d.Content)
		}
	}

	if err != nil {
		return fmt.Errorf("publish: %w", err)
	}

	pub := models.PublishedPost{
		ID:          uuid.New().String(),
		DraftID:     d.ID,
		XPostID:     publishRes.PostID,
		Content:     d.Content,
		PublishedAt: time.Now().UTC(),
		Status:      publishRes.Status,
	}
	if len(mediaIDs) > 0 && d.MediaURL != "" {
		pub.MediaURLs = []string{d.MediaURL}
	}
	if err := o.store.SavePublished(ctx, pub); err != nil {
		o.logger.Error("save published record", "draft_id", d.ID, "error", err)
	}

	if publishRes.Status == models.PublishStatusSuccess {
		if err := o.store.UpdateDraftStatus(ctx, d.ID, models.DraftStatusPublished); err != nil {
			o.logger.Error("update draft status", "draft_id", d.ID, "error", err)
		}
		if d.ArticleID != "" {
			if err := o.store.UpdateArticleStatus(ctx, d.ArticleID, models.ArticleStatusPublished); err != nil {
				o.logger.Error("update article status", "article_id", d.ArticleID, "error", err)
			}
		}
	}
	return nil
}

func (o *Orchestrator) ProcessCryptoAlerts(ctx context.Context) error {
	o.logger.Info("starting ProcessCryptoAlerts cycle")
	sources, err := o.store.GetEnabledSources(ctx)
	if err != nil {
		return fmt.Errorf("get enabled sources: %w", err)
	}

	var cmcSource *models.Source
	for _, s := range sources {
		if s.ID == "cmc-crypto" {
			cmcSource = &s
			break
		}
	}

	if cmcSource == nil {
		return fmt.Errorf("CoinMarketCap source (cmc-crypto) not found or not enabled")
	}

	f := fetcher.NewCoinMarketCapFetcher(*cmcSource, o.httpClient, o.cmcAPIKey, o.store)
	articles, err := f.Fetch(ctx)
	if err != nil {
		return fmt.Errorf("cmc fetch: %w", err)
	}

	for _, art := range articles {
		// Only proceed if movement threshold is met (e.g. >= 3.0% 24h change)
		// We can check latest snapshots from storage for BTC and ETH
		btcSnap, errBTC := o.store.GetLatestSnapshot(ctx, "BTC")
		ethSnap, errETH := o.store.GetLatestSnapshot(ctx, "ETH")

		shouldDraft := false
		if errBTC == nil && btcSnap != nil && math.Abs(btcSnap.ChangePercent) >= 3.0 {
			shouldDraft = true
			o.logger.Info("BTC threshold met", "change", btcSnap.ChangePercent)
		}
		if errETH == nil && ethSnap != nil && math.Abs(ethSnap.ChangePercent) >= 3.0 {
			shouldDraft = true
			o.logger.Info("ETH threshold met", "change", ethSnap.ChangePercent)
		}

		if !shouldDraft {
			o.logger.Info("crypto changes below threshold, skipping draft generation")
			// Save the article to DB as skipped
			art.Status = models.ArticleStatusSkipped
			existing, err := o.store.GetArticleByHash(ctx, art.Hash)
			if err == nil && existing == nil {
				_ = o.store.SaveArticle(ctx, art)
			}
			continue
		}

		if err := o.processArticle(ctx, *cmcSource, art); err != nil {
			o.logger.Error("failed to process crypto article", "error", err)
		}
	}

	return nil
}

func (o *Orchestrator) processArticle(ctx context.Context, src models.Source, art models.Article) error {
	// Deduplicate
	existing, err := o.store.GetArticleByHash(ctx, art.Hash)
	if err != nil {
		return fmt.Errorf("check existing article: %w", err)
	}
	if existing != nil {
		o.logger.Debug("article already exists, skipping", "hash", art.Hash, "title", art.Title)
		return nil
	}

	// Save raw article
	if err := o.store.SaveArticle(ctx, art); err != nil {
		return fmt.Errorf("save raw article: %w", err)
	}

	o.logger.Info("new article saved, triggering AI review", "id", art.ID, "title", art.Title)

	// AI Review
	reviewInput := ai.ReviewInput{
		Title:    art.Title,
		Content:  art.Content,
		Source:   src.Name,
		Category: art.Category,
		URL:      art.URL,
	}

	reviewOutput, err := o.reviewer.Review(ctx, reviewInput)
	if err != nil {
		return fmt.Errorf("ai reviewer: %w", err)
	}

	// Evaluate Decision
	decisionResult := o.engine.Decide(reviewOutput)
	o.logger.Info("posting decision", "action", decisionResult.Action, "reasons", decisionResult.Reasons)

	if decisionResult.Action == decision.ActionSkip {
		if err := o.store.UpdateArticleStatus(ctx, art.ID, models.ArticleStatusSkipped); err != nil {
			o.logger.Error("failed to update article status to skipped", "id", art.ID, "error", err)
		}
		return nil
	}

	// Build Draft Post
	draftID := uuid.New().String()
	threadJSON, _ := json.Marshal(reviewOutput.SuggestedThread)
	scoreJSON, _ := json.Marshal(reviewOutput.Scores)
	reviewJSON, _ := json.Marshal(reviewOutput)

	postType := models.PostTypeSingle
	if len(reviewOutput.SuggestedThread) > 0 {
		postType = models.PostTypeThread
	}
	if src.Category == "emergency" {
		postType = models.PostTypeAlert
	} else if src.Category == "market" {
		postType = models.PostTypeBriefing
	}

	draft := models.DraftPost{
		ID:                     draftID,
		ArticleID:              art.ID,
		PostType:               postType,
		Content:                reviewOutput.SuggestedPost,
		ThreadJSON:             string(threadJSON),
		ScoreJSON:              string(scoreJSON),
		ReviewJSON:             string(reviewJSON),
		Status:                 models.DraftStatusPending,
		RequiresManualApproval: decisionResult.Action == decision.ActionManualApproval,
		CreatedAt:              time.Now().UTC(),
	}

	// Generate and upload card image if category matches
	category := src.Category
	if category == "" {
		category = reviewOutput.Category
	}
	normCat := strings.ToLower(strings.TrimSpace(category))
	if normCat == "emergency" || normCat == "alert" || normCat == "darurat" ||
		normCat == "market" || normCat == "jisdor" || normCat == "kurs" ||
		normCat == "crypto" || normCat == "bitcoin" || normCat == "news" {

		cardDetails := reviewOutput.WhyItMatters
		if cardDetails == "" {
			cardDetails = reviewOutput.SuggestedPost
		}

		cardSourceName := reviewOutput.PublisherName
		if cardSourceName == "" {
			cardSourceName = src.Name
		}

		o.logger.Info("generating card image", "category", category, "title", art.Title)
		pngBytes, err := cardgen.GenerateCard(category, art.Title, cardDetails, cardSourceName)
		if err != nil {
			o.logger.Error("failed to generate card image", "error", err)
		} else if o.r2Client != nil {
			// Upload PNG bytes to Cloudflare R2
			key := fmt.Sprintf("cards/%s.png", draftID)
			o.logger.Info("uploading card image to R2", "key", key)
			publicURL, err := o.r2Client.Upload(ctx, key, pngBytes, "image/png")
			if err != nil {
				o.logger.Error("failed to upload card image to R2", "error", err)
			} else {
				o.logger.Info("card image uploaded successfully", "url", publicURL)
				draft.MediaURL = publicURL
			}
		}
	}

	// Determine if auto-approving
	isAutoApproved := decisionResult.Action == decision.ActionAutoPost
	if isAutoApproved {
		draft.Status = models.DraftStatusApproved
		now := time.Now().UTC()
		draft.ApprovedAt = &now
	}

	if err := o.store.SaveDraft(ctx, draft); err != nil {
		return fmt.Errorf("save draft: %w", err)
	}

	if isAutoApproved && o.postingMode == "auto" {
		o.logger.Info("auto-publishing approved draft to X", "draft_id", draftID)

		var publishRes *publisher.PublishResult
		var err error

		var mediaIDs []string
		if draft.MediaURL != "" {
			o.logger.Info("downloading media for auto-publish", "url", draft.MediaURL)
			mediaBytes, dlErr := o.downloadMediaBytes(ctx, draft.MediaURL)
			if dlErr != nil {
				o.logger.Error("failed to download media, falling back to text-only publish", "error", dlErr)
			} else {
				if xClient, ok := o.publisher.(*publisher.XClient); ok {
					uploader := publisher.NewMediaUploader(xClient)
					o.logger.Info("uploading media to X")
					mediaID, upErr := uploader.UploadBytes(ctx, mediaBytes, "card.png")
					if upErr != nil {
						o.logger.Error("failed to upload media to X, falling back to text-only publish", "error", upErr)
					} else {
						o.logger.Info("media uploaded to X successfully", "media_id", mediaID)
						mediaIDs = []string{mediaID}
					}
				}
			}
		}

		if len(mediaIDs) > 0 {
			publishRes, err = o.publisher.PublishWithMedia(ctx, draft.Content, mediaIDs)
		} else {
			publishRes, err = o.publisher.PublishText(ctx, draft.Content)
		}

		if err != nil {
			o.logger.Error("auto-publish failed", "draft_id", draftID, "error", err)
			return nil
		}

		// Save published post record
		pubPost := models.PublishedPost{
			ID:          uuid.New().String(),
			DraftID:     draftID,
			XPostID:     publishRes.PostID,
			Content:     draft.Content,
			PublishedAt: time.Now().UTC(),
			Status:      publishRes.Status,
		}
		if len(mediaIDs) > 0 && draft.MediaURL != "" {
			pubPost.MediaURLs = []string{draft.MediaURL}
		}
		if err := o.store.SavePublished(ctx, pubPost); err != nil {
			o.logger.Error("failed to save published post record", "draft_id", draftID, "error", err)
		}

		// Update draft status
		if publishRes.Status == models.PublishStatusSuccess {
			if err := o.store.UpdateDraftStatus(ctx, draftID, models.DraftStatusPublished); err != nil {
				o.logger.Error("failed to update draft status to published", "draft_id", draftID, "error", err)
			}
			// Update article status to published
			if err := o.store.UpdateArticleStatus(ctx, art.ID, models.ArticleStatusPublished); err != nil {
				o.logger.Error("failed to update article status to published", "id", art.ID, "error", err)
			}
		}
	} else {
		// Update article status to drafted
		if err := o.store.UpdateArticleStatus(ctx, art.ID, models.ArticleStatusDrafted); err != nil {
			o.logger.Error("failed to update article status to drafted", "id", art.ID, "error", err)
		}
	}

	return nil
}
