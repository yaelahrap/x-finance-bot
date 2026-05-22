package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/raflyramadhan/x-finance-bot/internal/ai"
	"github.com/raflyramadhan/x-finance-bot/internal/decision"
	"github.com/raflyramadhan/x-finance-bot/internal/fetcher"
	"github.com/raflyramadhan/x-finance-bot/internal/models"
	"github.com/raflyramadhan/x-finance-bot/internal/publisher"
	"github.com/raflyramadhan/x-finance-bot/internal/storage"
)

type Orchestrator struct {
	store       storage.Storage
	reviewer    *ai.Reviewer
	engine      *decision.Engine
	publisher   publisher.Publisher
	logger      *slog.Logger
	cmcAPIKey   string
	postingMode string
	httpClient  *http.Client
}

func NewOrchestrator(
	store storage.Storage,
	reviewer *ai.Reviewer,
	engine *decision.Engine,
	pub publisher.Publisher,
	logger *slog.Logger,
	cmcAPIKey string,
	postingMode string,
) *Orchestrator {
	return &Orchestrator{
		store:       store,
		reviewer:    reviewer,
		engine:      engine,
		publisher:   pub,
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

// ProcessScheduledDrafts publishes any drafts whose scheduled_at is at or
// before now. Failures are logged but do not abort the loop so a single bad
// draft cannot block the rest of the queue.
func (o *Orchestrator) ProcessScheduledDrafts(ctx context.Context) error {
	now := time.Now().UTC()
	due, err := o.store.GetDueScheduledDrafts(ctx, now)
	if err != nil {
		return fmt.Errorf("get due scheduled drafts: %w", err)
	}
	if len(due) == 0 {
		return nil
	}

	o.logger.Info("publishing scheduled drafts", "count", len(due))

	for _, d := range due {
		if err := o.publishDraft(ctx, d); err != nil {
			o.logger.Error("scheduled publish failed", "draft_id", d.ID, "error", err)
		}
	}
	return nil
}

// publishDraft sends a draft to the publisher and records the result.
func (o *Orchestrator) publishDraft(ctx context.Context, d models.DraftPost) error {
	publishRes, err := o.publisher.PublishText(ctx, d.Content)
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

		publishRes, err := o.publisher.PublishText(ctx, draft.Content)
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
