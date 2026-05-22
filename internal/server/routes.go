package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/raflyramadhan/x-finance-bot/internal/models"
	"github.com/raflyramadhan/x-finance-bot/internal/storage"
)

// Routes registers all HTTP routes on the given mux.
func Routes(mux *http.ServeMux, store storage.Storage, logger *slog.Logger) {
	mux.HandleFunc("GET /health", handleHealth)
	mux.HandleFunc("GET /api/drafts", handleGetDrafts(store, logger))
	mux.HandleFunc("GET /api/drafts/{id}", handleGetDraft(store, logger))
	mux.HandleFunc("POST /api/drafts/{id}/approve", handleApproveDraft(store, logger))
	mux.HandleFunc("POST /api/drafts/{id}/reject", handleRejectDraft(store, logger))
	mux.HandleFunc("GET /api/published", handleGetPublished(store, logger))
	mux.HandleFunc("GET /api/sources", handleGetSources(store, logger))
	mux.HandleFunc("POST /api/sources", handleCreateSource(store, logger))
	mux.HandleFunc("GET /api/market/{symbol}", handleGetMarket(store, logger))
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func handleGetDrafts(store storage.Storage, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		drafts, err := store.GetPendingDrafts(r.Context())
		if err != nil {
			logger.Error("get drafts", "error", err)
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
		posts, err := store.GetPublishedPosts(r.Context(), 50, 0)
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

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
