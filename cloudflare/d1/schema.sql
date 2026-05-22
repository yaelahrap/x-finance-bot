-- Cloudflare D1 Schema
-- This mirrors the SQLite schema used in local development.
-- Apply via: wrangler d1 execute <DB_NAME> --file=./schema.sql

CREATE TABLE IF NOT EXISTS sources (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  url TEXT NOT NULL,
  type TEXT NOT NULL,
  category TEXT NOT NULL,
  reliability_score INTEGER DEFAULT 5,
  enabled BOOLEAN DEFAULT true,
  created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS articles (
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
);

CREATE TABLE IF NOT EXISTS draft_posts (
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
  published_at TEXT
);

CREATE TABLE IF NOT EXISTS published_posts (
  id TEXT PRIMARY KEY,
  draft_id TEXT,
  x_post_id TEXT,
  content TEXT NOT NULL,
  media_urls TEXT,
  published_at TEXT NOT NULL,
  status TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS market_snapshots (
  id TEXT PRIMARY KEY,
  symbol TEXT NOT NULL,
  value TEXT NOT NULL,
  change_percent REAL,
  source TEXT,
  captured_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS ai_reviews (
  id TEXT PRIMARY KEY,
  draft_id TEXT NOT NULL,
  model TEXT NOT NULL,
  review_json TEXT NOT NULL,
  created_at TEXT NOT NULL
);

-- Indexes
CREATE INDEX IF NOT EXISTS idx_articles_hash ON articles(hash);
CREATE INDEX IF NOT EXISTS idx_articles_status ON articles(status);
CREATE INDEX IF NOT EXISTS idx_draft_posts_status ON draft_posts(status);
CREATE INDEX IF NOT EXISTS idx_market_snapshots_symbol_time ON market_snapshots(symbol, captured_at);
