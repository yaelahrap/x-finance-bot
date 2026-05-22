package server

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/raflyramadhan/x-finance-bot/internal/models"
	"github.com/raflyramadhan/x-finance-bot/internal/publisher"
	"github.com/raflyramadhan/x-finance-bot/internal/storage"
)

// PublishNow publishes an approved or scheduled draft immediately.
type PublishNow interface {
	PublishDraftNow(ctx context.Context, draftID string) (*models.PublishedPost, error)
}

// Routes registers all HTTP routes on the given mux.
func Routes(mux *http.ServeMux, store storage.Storage, pub publisher.Publisher, publisher PublishNow, logger *slog.Logger) {
	mux.HandleFunc("GET /health", handleHealth)
	mux.HandleFunc("GET /api/stats", handleGetStats(store, logger))
	mux.HandleFunc("GET /api/drafts", handleGetDrafts(store, logger))
	mux.HandleFunc("GET /api/drafts/{id}", handleGetDraft(store, logger))
	mux.HandleFunc("POST /api/drafts/{id}/approve", handleApproveDraft(store, logger))
	mux.HandleFunc("POST /api/drafts/{id}/reject", handleRejectDraft(store, logger))
	mux.HandleFunc("POST /api/drafts/{id}/schedule", handleScheduleDraft(store, logger))
	mux.HandleFunc("POST /api/drafts/{id}/unschedule", handleUnscheduleDraft(store, logger))
	mux.HandleFunc("POST /api/drafts/{id}/publish", handlePublishDraftNow(publisher, logger))
	mux.HandleFunc("GET /api/articles", handleGetArticles(store, logger))
	mux.HandleFunc("GET /api/published", handleGetPublished(store, logger))
	mux.HandleFunc("DELETE /api/published/{id}", handleDeletePublished(store, pub, logger))
	mux.HandleFunc("GET /api/sources", handleGetSources(store, logger))
	mux.HandleFunc("POST /api/sources", handleCreateSource(store, logger))
	mux.HandleFunc("GET /api/market/{symbol}", handleGetMarket(store, logger))
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func handleGetDrafts(store storage.Storage, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		statusFilter := r.URL.Query().Get("status")
		limit := parseLimit(r.URL.Query().Get("limit"), 100)

		var drafts []models.DraftPost
		var err error

		if statusFilter == "" || statusFilter == "pending" {
			drafts, err = store.GetPendingDrafts(r.Context())
		} else {
			drafts, err = store.GetDraftsByStatus(r.Context(), models.DraftStatus(statusFilter), limit)
		}
		if err != nil {
			logger.Error("get drafts", "error", err, "status", statusFilter)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get drafts"})
			return
		}
		if drafts == nil {
			drafts = []models.DraftPost{}
		}
		writeJSON(w, http.StatusOK, drafts)
	}
}

func handleGetDraft(store storage.Storage, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if id == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing draft id"})
			return
		}

		draft, err := store.GetDraftByID(r.Context(), id)
		if err != nil {
			logger.Error("get draft", "error", err, "id", id)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get draft"})
			return
		}
		if draft == nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "draft not found"})
			return
		}
		writeJSON(w, http.StatusOK, draft)
	}
}

func handleApproveDraft(store storage.Storage, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if id == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing draft id"})
			return
		}

		if err := store.ApproveDraft(r.Context(), id); err != nil {
			logger.Error("approve draft", "error", err, "id", id)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to approve draft"})
			return
		}

		logger.Info("draft approved", "id", id)
		writeJSON(w, http.StatusOK, map[string]string{"status": "approved", "id": id})
	}
}

func handleRejectDraft(store storage.Storage, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if id == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing draft id"})
			return
		}

		if err := store.RejectDraft(r.Context(), id); err != nil {
			logger.Error("reject draft", "error", err, "id", id)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to reject draft"})
			return
		}

		logger.Info("draft rejected", "id", id)
		writeJSON(w, http.StatusOK, map[string]string{"status": "rejected", "id": id})
	}
}

func handleGetPublished(store storage.Storage, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limit := parseLimit(r.URL.Query().Get("limit"), 50)
		posts, err := store.GetPublishedPosts(r.Context(), limit, 0)
		if err != nil {
			logger.Error("get published", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get published posts"})
			return
		}
		if posts == nil {
			posts = []models.PublishedPost{}
		}
		writeJSON(w, http.StatusOK, posts)
	}
}

func handleGetSources(store storage.Storage, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sources, err := store.GetEnabledSources(r.Context())
		if err != nil {
			logger.Error("get sources", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get sources"})
			return
		}
		if sources == nil {
			sources = []models.Source{}
		}
		writeJSON(w, http.StatusOK, sources)
	}
}

func handleCreateSource(store storage.Storage, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var source models.Source
		if err := json.NewDecoder(r.Body).Decode(&source); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}

		if source.Name == "" || source.URL == "" || source.Type == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name, url, and type are required"})
			return
		}

		if err := store.SaveSource(r.Context(), source); err != nil {
			logger.Error("save source", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to save source"})
			return
		}

		logger.Info("source created", "name", source.Name)
		writeJSON(w, http.StatusCreated, source)
	}
}

func handleGetMarket(store storage.Storage, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		symbol := strings.ToUpper(r.PathValue("symbol"))
		if symbol == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing symbol"})
			return
		}

		snap, err := store.GetLatestSnapshot(r.Context(), symbol)
		if err != nil {
			logger.Error("get market", "error", err, "symbol", symbol)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get market data"})
			return
		}
		if snap == nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "no data for symbol"})
			return
		}
		writeJSON(w, http.StatusOK, snap)
	}
}

func handleGetStats(store storage.Storage, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		counts, err := store.Counts(r.Context())
		if err != nil {
			logger.Error("get stats", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get stats"})
			return
		}
		writeJSON(w, http.StatusOK, counts)
	}
}

func handleScheduleDraft(store storage.Storage, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if id == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing draft id"})
			return
		}

		var body struct {
			ScheduledAt string `json:"scheduled_at"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}
		if body.ScheduledAt == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "scheduled_at is required (RFC3339)"})
			return
		}
		at, err := time.Parse(time.RFC3339, body.ScheduledAt)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "scheduled_at must be RFC3339"})
			return
		}
		if at.Before(time.Now().UTC().Add(-1 * time.Minute)) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "scheduled_at must be in the future"})
			return
		}

		if err := store.ScheduleDraft(r.Context(), id, at); err != nil {
			logger.Error("schedule draft", "error", err, "id", id)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to schedule draft"})
			return
		}

		logger.Info("draft scheduled", "id", id, "at", at)
		writeJSON(w, http.StatusOK, map[string]any{"status": "scheduled", "id": id, "scheduled_at": at})
	}
}

func handleUnscheduleDraft(store storage.Storage, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if id == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing draft id"})
			return
		}
		if err := store.UpdateDraftStatus(r.Context(), id, models.DraftStatusApproved); err != nil {
			logger.Error("unschedule draft", "error", err, "id", id)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to unschedule draft"})
			return
		}
		logger.Info("draft unscheduled", "id", id)
		writeJSON(w, http.StatusOK, map[string]string{"status": "approved", "id": id})
	}
}

func handlePublishDraftNow(p PublishNow, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if id == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing draft id"})
			return
		}
		if p == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "publisher not configured"})
			return
		}

		published, err := p.PublishDraftNow(r.Context(), id)
		if err != nil {
			logger.Error("publish now", "error", err, "id", id)
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
			return
		}

		logger.Info("draft published immediately", "id", id)
		writeJSON(w, http.StatusOK, published)
	}
}

func handleGetArticles(store storage.Storage, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limit := parseLimit(r.URL.Query().Get("limit"), 50)
		statusFilter := r.URL.Query().Get("status")

		var articles []models.Article
		var err error

		if statusFilter == "" {
			articles, err = store.GetRecentArticles(r.Context(), limit)
		} else {
			articles, err = store.GetArticlesByStatus(r.Context(), models.ArticleStatus(statusFilter), limit)
		}
		if err != nil {
			logger.Error("get articles", "error", err, "status", statusFilter)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get articles"})
			return
		}
		if articles == nil {
			articles = []models.Article{}
		}
		writeJSON(w, http.StatusOK, articles)
	}
}

func parseLimit(raw string, fallback int) int {
	if raw == "" {
		return fallback
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return fallback
	}
	if n > 500 {
		return 500
	}
	return n
}

func handleDeletePublished(store storage.Storage, pub publisher.Publisher, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if id == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing published id"})
			return
		}

		post, err := store.GetPublishedByID(r.Context(), id)
		if err != nil {
			logger.Error("get published", "error", err, "id", id)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to look up post"})
			return
		}
		if post == nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "post not found"})
			return
		}
		if post.Status == models.PublishStatusDeleted {
			writeJSON(w, http.StatusOK, map[string]string{"status": "already_deleted", "id": id})
			return
		}
		if post.XPostID == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "post has no x_post_id, nothing to delete on X"})
			return
		}
		if pub == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "publisher not configured"})
			return
		}

		if err := pub.DeletePost(r.Context(), post.XPostID); err != nil {
			logger.Error("delete post on X", "error", err, "x_post_id", post.XPostID)
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": "X API delete failed: " + err.Error()})
			return
		}

		if err := store.MarkPublishedDeleted(r.Context(), id); err != nil {
			logger.Error("mark published deleted", "error", err, "id", id)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "deleted on X but failed to update local record"})
			return
		}

		logger.Info("published post deleted", "id", id, "x_post_id", post.XPostID)
		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted", "id": id, "x_post_id": post.XPostID})
	}
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
