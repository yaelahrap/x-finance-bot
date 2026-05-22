package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	"github.com/raflyramadhan/x-finance-bot/internal/models"
	_ "modernc.org/sqlite"
)

// SQLiteStore implements Storage using a local SQLite database.
type SQLiteStore struct {
	db *sql.DB
}

// NewSQLite opens the SQLite database at the given DSN, runs migrations,
// and returns a ready-to-use store.
func NewSQLite(dsn string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}

	// Enable WAL mode for better concurrent read performance.
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, err
	}
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		db.Close()
		return nil, err
	}

	s := &SQLiteStore{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

// Close closes the underlying database connection.
func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

func (s *SQLiteStore) migrate() error {
	migrations := []string{
		`CREATE TABLE IF NOT EXISTS sources (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			url TEXT NOT NULL,
			type TEXT NOT NULL,
			category TEXT NOT NULL,
			reliability_score INTEGER DEFAULT 5,
			enabled BOOLEAN DEFAULT true,
			created_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS articles (
			id TEXT PRIMARY KEY,
			source_id TEXT NOT NULL,
			title TEXT NOT NULL,
			url TEXT NOT NULL,
			content TEXT,
			summary TEXT,
			published_at TEXT,
			fetched_at TEXT NOT NULL,
			hash TEXT NOT NULL,
			category TEXT,
			status TEXT DEFAULT 'fetched',
			FOREIGN KEY(source_id) REFERENCES sources(id)
		)`,
		`CREATE TABLE IF NOT EXISTS draft_posts (
			id TEXT PRIMARY KEY,
			article_id TEXT,
			post_type TEXT NOT NULL,
			content TEXT NOT NULL,
			thread_json TEXT,
			score_json TEXT,
			review_json TEXT,
			status TEXT DEFAULT 'pending',
			requires_manual_approval BOOLEAN DEFAULT true,
			created_at TEXT NOT NULL,
			approved_at TEXT,
			scheduled_at TEXT,
			published_at TEXT,
			media_url TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS published_posts (
			id TEXT PRIMARY KEY,
			draft_id TEXT,
			x_post_id TEXT,
			content TEXT NOT NULL,
			media_urls TEXT,
			published_at TEXT NOT NULL,
			status TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS market_snapshots (
			id TEXT PRIMARY KEY,
			symbol TEXT NOT NULL,
			value TEXT NOT NULL,
			change_percent REAL,
			source TEXT,
			captured_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS ai_reviews (
			id TEXT PRIMARY KEY,
			draft_id TEXT NOT NULL,
			model TEXT NOT NULL,
			review_json TEXT NOT NULL,
			created_at TEXT NOT NULL
		)`,
		// Indexes
		`CREATE INDEX IF NOT EXISTS idx_articles_hash ON articles(hash)`,
		`CREATE INDEX IF NOT EXISTS idx_articles_status ON articles(status)`,
		`CREATE INDEX IF NOT EXISTS idx_draft_posts_status ON draft_posts(status)`,
		`CREATE INDEX IF NOT EXISTS idx_market_snapshots_symbol_time ON market_snapshots(symbol, captured_at)`,
	}

	for _, m := range migrations {
		if _, err := s.db.Exec(m); err != nil {
			return err
		}
	}

	// Idempotent ALTER migrations: tolerate "duplicate column" on existing DBs.
	addColumnMigrations := []string{
		`ALTER TABLE draft_posts ADD COLUMN scheduled_at TEXT`,
		`ALTER TABLE draft_posts ADD COLUMN media_url TEXT`,
	}
	for _, m := range addColumnMigrations {
		if _, err := s.db.Exec(m); err != nil && !strings.Contains(err.Error(), "duplicate column") {
			return err
		}
	}

	// Indexes that depend on columns added by ALTER above.
	postAlterIndexes := []string{
		`CREATE INDEX IF NOT EXISTS idx_draft_posts_scheduled_at ON draft_posts(scheduled_at)`,
	}
	for _, m := range postAlterIndexes {
		if _, err := s.db.Exec(m); err != nil {
			return err
		}
	}
	return nil
}

// --- Articles ---

func (s *SQLiteStore) SaveArticle(ctx context.Context, article models.Article) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO articles (id, source_id, title, url, content, summary, published_at, fetched_at, hash, category, status)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		article.ID, article.SourceID, article.Title, article.URL,
		article.Content, article.Summary,
		formatTimePtr(article.PublishedAt),
		formatTime(article.FetchedAt),
		article.Hash, article.Category, string(article.Status),
	)
	return err
}

func (s *SQLiteStore) GetArticleByHash(ctx context.Context, hash string) (*models.Article, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, source_id, title, url, content, summary, published_at, fetched_at, hash, category, status
		 FROM articles WHERE hash = ?`, hash)
	return scanArticle(row)
}

func (s *SQLiteStore) GetArticlesByStatus(ctx context.Context, status models.ArticleStatus, limit int) ([]models.Article, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, source_id, title, url, content, summary, published_at, fetched_at, hash, category, status
		 FROM articles WHERE status = ? ORDER BY fetched_at DESC LIMIT ?`,
		string(status), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var articles []models.Article
	for rows.Next() {
		a, err := scanArticleRow(rows)
		if err != nil {
			return nil, err
		}
		articles = append(articles, *a)
	}
	return articles, rows.Err()
}

// GetRecentArticles returns the most recently fetched articles regardless of status.
func (s *SQLiteStore) GetRecentArticles(ctx context.Context, limit int) ([]models.Article, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, source_id, title, url, content, summary, published_at, fetched_at, hash, category, status
		 FROM articles ORDER BY fetched_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var articles []models.Article
	for rows.Next() {
		a, err := scanArticleRow(rows)
		if err != nil {
			return nil, err
		}
		articles = append(articles, *a)
	}
	return articles, rows.Err()
}

func (s *SQLiteStore) UpdateArticleStatus(ctx context.Context, id string, status models.ArticleStatus) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE articles SET status = ? WHERE id = ?`, string(status), id)
	return err
}

// --- Drafts ---

func (s *SQLiteStore) SaveDraft(ctx context.Context, draft models.DraftPost) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO draft_posts (id, article_id, post_type, content, thread_json, score_json, review_json, status, requires_manual_approval, created_at, approved_at, scheduled_at, published_at, media_url)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		draft.ID, draft.ArticleID, string(draft.PostType), draft.Content,
		draft.ThreadJSON, draft.ScoreJSON, draft.ReviewJSON,
		string(draft.Status), draft.RequiresManualApproval,
		formatTime(draft.CreatedAt),
		formatTimePtr(draft.ApprovedAt),
		formatTimePtr(draft.ScheduledAt),
		formatTimePtr(draft.PublishedAt),
		draft.MediaURL,
	)
	return err
}

func (s *SQLiteStore) GetPendingDrafts(ctx context.Context) ([]models.DraftPost, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, article_id, post_type, content, thread_json, score_json, review_json, status, requires_manual_approval, created_at, approved_at, scheduled_at, published_at, media_url
		 FROM draft_posts WHERE status = ? ORDER BY created_at DESC`,
		string(models.DraftStatusPending))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var drafts []models.DraftPost
	for rows.Next() {
		d, err := scanDraftRow(rows)
		if err != nil {
			return nil, err
		}
		drafts = append(drafts, *d)
	}
	return drafts, rows.Err()
}

func (s *SQLiteStore) GetDraftByID(ctx context.Context, id string) (*models.DraftPost, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, article_id, post_type, content, thread_json, score_json, review_json, status, requires_manual_approval, created_at, approved_at, scheduled_at, published_at, media_url
		 FROM draft_posts WHERE id = ?`, id)
	return scanDraft(row)
}

func (s *SQLiteStore) UpdateDraftStatus(ctx context.Context, id string, status models.DraftStatus) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE draft_posts SET status = ? WHERE id = ?`, string(status), id)
	return err
}

func (s *SQLiteStore) ApproveDraft(ctx context.Context, id string) error {
	now := formatTime(time.Now().UTC())
	_, err := s.db.ExecContext(ctx,
		`UPDATE draft_posts SET status = ?, approved_at = ? WHERE id = ?`,
		string(models.DraftStatusApproved), now, id)
	return err
}

func (s *SQLiteStore) RejectDraft(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE draft_posts SET status = ? WHERE id = ?`,
		string(models.DraftStatusRejected), id)
	return err
}

// ScheduleDraft sets the draft to scheduled status with the given publish time.
func (s *SQLiteStore) ScheduleDraft(ctx context.Context, id string, at time.Time) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE draft_posts SET status = ?, scheduled_at = ? WHERE id = ?`,
		string(models.DraftStatusScheduled), formatTime(at.UTC()), id)
	return err
}

// GetDraftsByStatus returns drafts in the given status, newest first.
func (s *SQLiteStore) GetDraftsByStatus(ctx context.Context, status models.DraftStatus, limit int) ([]models.DraftPost, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, article_id, post_type, content, thread_json, score_json, review_json, status, requires_manual_approval, created_at, approved_at, scheduled_at, published_at, media_url
		 FROM draft_posts WHERE status = ? ORDER BY created_at DESC LIMIT ?`,
		string(status), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var drafts []models.DraftPost
	for rows.Next() {
		d, err := scanDraftRow(rows)
		if err != nil {
			return nil, err
		}
		drafts = append(drafts, *d)
	}
	return drafts, rows.Err()
}

// GetDueScheduledDrafts returns scheduled drafts whose scheduled_at is at or before "before".
func (s *SQLiteStore) GetDueScheduledDrafts(ctx context.Context, before time.Time) ([]models.DraftPost, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, article_id, post_type, content, thread_json, score_json, review_json, status, requires_manual_approval, created_at, approved_at, scheduled_at, published_at, media_url
		 FROM draft_posts WHERE status = ? AND scheduled_at IS NOT NULL AND scheduled_at <= ? ORDER BY scheduled_at ASC`,
		string(models.DraftStatusScheduled), formatTime(before.UTC()))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var drafts []models.DraftPost
	for rows.Next() {
		d, err := scanDraftRow(rows)
		if err != nil {
			return nil, err
		}
		drafts = append(drafts, *d)
	}
	return drafts, rows.Err()
}

// --- Published ---

func (s *SQLiteStore) SavePublished(ctx context.Context, post models.PublishedPost) error {
	mediaJSON, err := json.Marshal(post.MediaURLs)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO published_posts (id, draft_id, x_post_id, content, media_urls, published_at, status)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		post.ID, post.DraftID, post.XPostID, post.Content,
		string(mediaJSON), formatTime(post.PublishedAt), string(post.Status),
	)
	return err
}

func (s *SQLiteStore) GetPublishedPosts(ctx context.Context, limit, offset int) ([]models.PublishedPost, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, draft_id, x_post_id, content, media_urls, published_at, status
		 FROM published_posts ORDER BY published_at DESC LIMIT ? OFFSET ?`,
		limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var posts []models.PublishedPost
	for rows.Next() {
		p, err := scanPublishedRow(rows)
		if err != nil {
			return nil, err
		}
		posts = append(posts, *p)
	}
	return posts, rows.Err()
}

// GetPublishedByDraftID returns the published record for a draft, or nil if none.
func (s *SQLiteStore) GetPublishedByDraftID(ctx context.Context, draftID string) (*models.PublishedPost, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, draft_id, x_post_id, content, media_urls, published_at, status
		 FROM published_posts WHERE draft_id = ? ORDER BY published_at DESC LIMIT 1`, draftID)
	return scanPublishedSingle(row)
}

// GetPublishedByID returns the published record by its primary id, or nil if not found.
func (s *SQLiteStore) GetPublishedByID(ctx context.Context, id string) (*models.PublishedPost, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, draft_id, x_post_id, content, media_urls, published_at, status
		 FROM published_posts WHERE id = ?`, id)
	return scanPublishedSingle(row)
}

// MarkPublishedDeleted flips the published_posts.status to "deleted" for the given record.
func (s *SQLiteStore) MarkPublishedDeleted(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE published_posts SET status = ? WHERE id = ?`,
		string(models.PublishStatusDeleted), id)
	return err
}

// --- Market ---

func (s *SQLiteStore) SaveMarketSnapshot(ctx context.Context, snap models.MarketSnapshot) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO market_snapshots (id, symbol, value, change_percent, source, captured_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		snap.ID, snap.Symbol, snap.Value, snap.ChangePercent,
		snap.Source, formatTime(snap.CapturedAt),
	)
	return err
}

func (s *SQLiteStore) GetLatestSnapshot(ctx context.Context, symbol string) (*models.MarketSnapshot, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, symbol, value, change_percent, source, captured_at
		 FROM market_snapshots WHERE symbol = ? ORDER BY captured_at DESC LIMIT 1`, symbol)

	var snap models.MarketSnapshot
	var capturedAt string
	var changePercent sql.NullFloat64
	var source sql.NullString

	err := row.Scan(&snap.ID, &snap.Symbol, &snap.Value, &changePercent, &source, &capturedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	snap.CapturedAt, _ = time.Parse(time.RFC3339, capturedAt)
	if changePercent.Valid {
		snap.ChangePercent = changePercent.Float64
	}
	if source.Valid {
		snap.Source = source.String
	}
	return &snap, nil
}

// --- Sources ---

func (s *SQLiteStore) GetEnabledSources(ctx context.Context) ([]models.Source, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, url, type, category, reliability_score, enabled, created_at
		 FROM sources WHERE enabled = true ORDER BY reliability_score DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sources []models.Source
	for rows.Next() {
		var src models.Source
		var createdAt string
		err := rows.Scan(&src.ID, &src.Name, &src.URL, &src.Type, &src.Category,
			&src.ReliabilityScore, &src.Enabled, &createdAt)
		if err != nil {
			return nil, err
		}
		src.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		sources = append(sources, src)
	}
	return sources, rows.Err()
}

func (s *SQLiteStore) SaveSource(ctx context.Context, source models.Source) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO sources (id, name, url, type, category, reliability_score, enabled, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		source.ID, source.Name, source.URL, source.Type, source.Category,
		source.ReliabilityScore, source.Enabled, formatTime(source.CreatedAt),
	)
	return err
}

// --- Stats ---

// Counts returns aggregate tallies for dashboard summaries.
func (s *SQLiteStore) Counts(ctx context.Context) (Counts, error) {
	var c Counts

	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM articles`).Scan(&c.Articles); err != nil {
		return c, err
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sources WHERE enabled = true`).Scan(&c.Sources); err != nil {
		return c, err
	}

	draftCounts := []struct {
		status string
		dest   *int
	}{
		{string(models.DraftStatusPending), &c.DraftsPending},
		{string(models.DraftStatusApproved), &c.DraftsApproved},
		{string(models.DraftStatusScheduled), &c.DraftsScheduled},
		{string(models.DraftStatusRejected), &c.DraftsRejected},
		{string(models.DraftStatusPublished), &c.DraftsPublished},
	}
	for _, dc := range draftCounts {
		if err := s.db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM draft_posts WHERE status = ?`, dc.status).Scan(dc.dest); err != nil {
			return c, err
		}
	}

	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM published_posts WHERE status = ?`,
		string(models.PublishStatusSuccess)).Scan(&c.PublishedSuccess); err != nil {
		return c, err
	}
	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM published_posts WHERE status = ?`,
		string(models.PublishStatusFailed)).Scan(&c.PublishedFailed); err != nil {
		return c, err
	}
	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM published_posts WHERE status = ?`,
		string(models.PublishStatusDeleted)).Scan(&c.PublishedDeleted); err != nil {
		return c, err
	}

	return c, nil
}

// --- Helpers ---

func formatTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}

func formatTimePtr(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.UTC().Format(time.RFC3339)
	return &s
}

func parseTimePtr(s *string) *time.Time {
	if s == nil || *s == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339, *s)
	if err != nil {
		return nil
	}
	return &t
}

// scanArticle scans a single article row from QueryRow.
func scanArticle(row *sql.Row) (*models.Article, error) {
	var a models.Article
	var publishedAt, fetchedAt sql.NullString
	var content, summary, category sql.NullString
	var status string

	err := row.Scan(&a.ID, &a.SourceID, &a.Title, &a.URL,
		&content, &summary, &publishedAt, &fetchedAt,
		&a.Hash, &category, &status)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	a.Status = models.ArticleStatus(status)
	if content.Valid {
		a.Content = content.String
	}
	if summary.Valid {
		a.Summary = summary.String
	}
	if category.Valid {
		a.Category = category.String
	}
	if fetchedAt.Valid {
		a.FetchedAt, _ = time.Parse(time.RFC3339, fetchedAt.String)
	}
	if publishedAt.Valid {
		t, _ := time.Parse(time.RFC3339, publishedAt.String)
		a.PublishedAt = &t
	}
	return &a, nil
}

// scanArticleRow scans an article from rows.Next().
func scanArticleRow(rows *sql.Rows) (*models.Article, error) {
	var a models.Article
	var publishedAt, fetchedAt sql.NullString
	var content, summary, category sql.NullString
	var status string

	err := rows.Scan(&a.ID, &a.SourceID, &a.Title, &a.URL,
		&content, &summary, &publishedAt, &fetchedAt,
		&a.Hash, &category, &status)
	if err != nil {
		return nil, err
	}

	a.Status = models.ArticleStatus(status)
	if content.Valid {
		a.Content = content.String
	}
	if summary.Valid {
		a.Summary = summary.String
	}
	if category.Valid {
		a.Category = category.String
	}
	if fetchedAt.Valid {
		a.FetchedAt, _ = time.Parse(time.RFC3339, fetchedAt.String)
	}
	if publishedAt.Valid {
		t, _ := time.Parse(time.RFC3339, publishedAt.String)
		a.PublishedAt = &t
	}
	return &a, nil
}

// scanDraft scans a single draft from QueryRow.
func scanDraft(row *sql.Row) (*models.DraftPost, error) {
	var d models.DraftPost
	var articleID, threadJSON, scoreJSON, reviewJSON sql.NullString
	var createdAt string
	var approvedAt, scheduledAt, publishedAt sql.NullString
	var postType, status string
	var mediaURL sql.NullString

	err := row.Scan(&d.ID, &articleID, &postType, &d.Content,
		&threadJSON, &scoreJSON, &reviewJSON,
		&status, &d.RequiresManualApproval, &createdAt,
		&approvedAt, &scheduledAt, &publishedAt, &mediaURL)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	d.PostType = models.PostType(postType)
	d.Status = models.DraftStatus(status)
	if articleID.Valid {
		d.ArticleID = articleID.String
	}
	if threadJSON.Valid {
		d.ThreadJSON = threadJSON.String
	}
	if scoreJSON.Valid {
		d.ScoreJSON = scoreJSON.String
	}
	if reviewJSON.Valid {
		d.ReviewJSON = reviewJSON.String
	}
	d.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	if approvedAt.Valid {
		s := approvedAt.String
		d.ApprovedAt = parseTimePtr(&s)
	}
	if scheduledAt.Valid {
		s := scheduledAt.String
		d.ScheduledAt = parseTimePtr(&s)
	}
	if publishedAt.Valid {
		s := publishedAt.String
		d.PublishedAt = parseTimePtr(&s)
	}
	if mediaURL.Valid {
		d.MediaURL = mediaURL.String
	}
	return &d, nil
}

// scanDraftRow scans a draft from rows.Next().
func scanDraftRow(rows *sql.Rows) (*models.DraftPost, error) {
	var d models.DraftPost
	var articleID, threadJSON, scoreJSON, reviewJSON sql.NullString
	var createdAt string
	var approvedAt, scheduledAt, publishedAt sql.NullString
	var postType, status string
	var mediaURL sql.NullString

	err := rows.Scan(&d.ID, &articleID, &postType, &d.Content,
		&threadJSON, &scoreJSON, &reviewJSON,
		&status, &d.RequiresManualApproval, &createdAt,
		&approvedAt, &scheduledAt, &publishedAt, &mediaURL)
	if err != nil {
		return nil, err
	}

	d.PostType = models.PostType(postType)
	d.Status = models.DraftStatus(status)
	if articleID.Valid {
		d.ArticleID = articleID.String
	}
	if threadJSON.Valid {
		d.ThreadJSON = threadJSON.String
	}
	if scoreJSON.Valid {
		d.ScoreJSON = scoreJSON.String
	}
	if reviewJSON.Valid {
		d.ReviewJSON = reviewJSON.String
	}
	d.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	if approvedAt.Valid {
		s := approvedAt.String
		d.ApprovedAt = parseTimePtr(&s)
	}
	if scheduledAt.Valid {
		s := scheduledAt.String
		d.ScheduledAt = parseTimePtr(&s)
	}
	if publishedAt.Valid {
		s := publishedAt.String
		d.PublishedAt = parseTimePtr(&s)
	}
	if mediaURL.Valid {
		d.MediaURL = mediaURL.String
	}
	return &d, nil
}

// scanPublishedRow scans a published post from rows.Next().
func scanPublishedRow(rows *sql.Rows) (*models.PublishedPost, error) {
	var p models.PublishedPost
	var draftID, xPostID sql.NullString
	var mediaURLsJSON sql.NullString
	var publishedAt string
	var status string

	err := rows.Scan(&p.ID, &draftID, &xPostID, &p.Content,
		&mediaURLsJSON, &publishedAt, &status)
	if err != nil {
		return nil, err
	}

	p.Status = models.PublishStatus(status)
	if draftID.Valid {
		p.DraftID = draftID.String
	}
	if xPostID.Valid {
		p.XPostID = xPostID.String
	}
	p.PublishedAt, _ = time.Parse(time.RFC3339, publishedAt)
	if mediaURLsJSON.Valid && mediaURLsJSON.String != "" {
		_ = json.Unmarshal([]byte(mediaURLsJSON.String), &p.MediaURLs)
	}
	return &p, nil
}

// scanPublishedSingle scans a published post from QueryRow.
// Returns (nil, nil) when sql.ErrNoRows.
func scanPublishedSingle(row *sql.Row) (*models.PublishedPost, error) {
	var p models.PublishedPost
	var draftID, xPostID sql.NullString
	var mediaURLsJSON sql.NullString
	var publishedAt string
	var status string

	err := row.Scan(&p.ID, &draftID, &xPostID, &p.Content,
		&mediaURLsJSON, &publishedAt, &status)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	p.Status = models.PublishStatus(status)
	if draftID.Valid {
		p.DraftID = draftID.String
	}
	if xPostID.Valid {
		p.XPostID = xPostID.String
	}
	p.PublishedAt, _ = time.Parse(time.RFC3339, publishedAt)
	if mediaURLsJSON.Valid && mediaURLsJSON.String != "" {
		_ = json.Unmarshal([]byte(mediaURLsJSON.String), &p.MediaURLs)
	}
	return &p, nil
}
