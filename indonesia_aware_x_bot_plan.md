# Indonesia-Aware Breaking Info Bot — Golang + Cloudflare Ecosystem

## 1. Project Goal

Build an automated X/Twitter content bot that curates and publishes high-signal updates about:

- Indonesia market pulse
- USD/IDR daily update
- Indonesia news covered internationally
- Global news that matters to Indonesian audiences
- Macro economy
- Finance and commodities
- Crypto market alerts
- Policy updates
- Tech/AI/cybersecurity
- Emergency and natural disaster alerts

The bot must prioritize accuracy, source transparency, low-noise posting, and avoid misleading financial advice.

Core backend must be written in Golang.

---

## 2. Main Positioning

Account positioning:

> Fast updates on Indonesia, global markets, policy, and major world events — summarized for people who want signal, not noise.

Tone:

- Fast
- Neutral
- Clear
- No hype
- Indonesia-aware
- No financial advice
- No rumor-based posting unless clearly labeled as unconfirmed
- Always prefer official or reliable sources

---

## 3. Recommended Architecture

### Preferred Architecture

```text
Cloudflare Cron / Worker
        ↓
Go Bot Service on VPS
        ↓
Source Fetchers
        ↓
Normalizer
        ↓
Deduplication
        ↓
Relevance Scoring
        ↓
Claude Editorial Review
        ↓
Decision Engine
        ↓
Post Queue
        ↓
X API Publisher
        ↓
Logging + Analytics
```

### Why VPS for Go Core

The Go bot should run on VPS because:

- Easier to use Go libraries
- Easier to run scheduled workers
- Easier to handle long-running background jobs
- Easier to debug
- Easier to integrate with browser-based scraping later if needed
- Easier to add Redis/Postgres later if Cloudflare-only becomes limiting

Cloudflare should handle:

- R2 for storing generated images/media assets
- D1 for lightweight metadata storage
- KV for config/cache
- Workers for API endpoints, webhook, dashboard API, and cron trigger
- Queues for async tasks
- Pages for admin dashboard
- Turnstile for dashboard protection
- Cloudflare Access for private dashboard login
- CDN for public image/media delivery

---

## 4. Cloudflare Components

### 4.1 Cloudflare R2

Use R2 for:

- Generated post images
- Market snapshot images
- Source screenshots if needed
- Logo/banner assets
- Daily briefing image archives
- JSON backup of generated drafts

Buckets:

```text
x-info-bot-media
x-info-bot-archives
x-info-bot-source-cache
```

Example object paths:

```text
media/2026/05/22/market-pulse-usdidr.png
archives/2026/05/22/daily-briefing.json
sources/2026/05/22/reuters-article-hash.json
```

### 4.2 Cloudflare D1

Use D1 for lightweight relational storage.

Tables:

```text
sources
articles
draft_posts
published_posts
market_snapshots
ai_reviews
post_scores
system_logs
```

D1 should store metadata, not large content.

### 4.3 Cloudflare KV

Use KV for fast config/cache:

```text
CONFIG:posting_mode
CONFIG:daily_market_time
CONFIG:min_auto_post_score
CONFIG:enabled_categories
CACHE:latest_usdidr
CACHE:latest_btc
CACHE:latest_gold
CACHE:last_posted_article_hash
```

### 4.4 Cloudflare Queues

Use Queues for async tasks:

```text
fetch-news-queue
score-news-queue
ai-review-queue
publish-post-queue
image-generate-queue
```

### 4.5 Cloudflare Workers

Workers responsibilities:

```text
- Receive cron trigger
- Call VPS Go bot webhook
- Expose admin API
- Serve signed R2 media URLs if needed
- Receive X webhook if later needed
- Protect dashboard routes
```

### 4.6 Cloudflare Pages

Use Pages for admin dashboard.

Dashboard features:

```text
- View drafts
- Approve/reject posts
- See AI review score
- See source links
- Force publish
- Pause bot
- Configure categories
- Configure auto-post thresholds
- View published history
```

---

## 5. Anthropic Claude Role

Claude should act as:

```text
Senior Editor + Risk Analyst + Context Explainer
```

Claude should not be the only decision maker. It should review structured input and return structured JSON.

### 5.1 Claude Job Desks

#### A. Editorial Review

Claude checks:

```text
- Is the draft clear?
- Is it too clickbait?
- Is it neutral?
- Does it sound like financial advice?
- Is it too vague?
- Is source attribution enough?
- Is the tone consistent?
```

#### B. Relevance Scoring

Claude gives scores:

```json
{
  "indonesia_relevance": 0,
  "global_importance": 0,
  "market_impact": 0,
  "urgency": 0,
  "public_interest": 0,
  "source_confidence": 0,
  "total_score": 0
}
```

#### C. Risk Filter

Claude checks:

```json
{
  "risk_level": "low | medium | high",
  "risk_reasons": [],
  "requires_manual_approval": true,
  "safe_to_auto_post": false
}
```

#### D. Context Explainer

Claude adds:

```text
Why this matters for Indonesia:
- Impact on currency
- Impact on inflation
- Impact on public policy
- Impact on market sentiment
- Impact on everyday people
```

#### E. Post Rewriter

Claude rewrites into:

```text
- Short X post
- Thread version
- Daily briefing version
- Emergency alert version
```

---

## 6. Posting Rules

### 6.1 Auto-Post Allowed

Auto-post only for:

```text
- Daily USD/IDR update
- Daily market pulse
- Official announcements from verified official sources
- Low-risk macro data releases
- Scheduled daily briefing
```

### 6.2 Manual Approval Required

Manual approval required for:

```text
- Politics
- War/conflict
- Natural disaster with casualties
- Crypto exploit/hack
- Banking crisis
- Rumors
- Sensitive legal cases
- Unverified breaking news
- Anything with only one non-official source
```

### 6.3 Skip

Skip content if:

```text
- No reliable source
- Too local with no broader impact
- Pure gossip
- Duplicate of previous post
- Too speculative
- Headline is misleading
- The bot cannot explain why it matters
```

---

## 7. Content Categories

### 7.1 Daily Market Pulse

Data:

```text
- USD/IDR
- EUR/IDR optional
- SGD/IDR optional
- JPY/IDR optional
- Gold spot
- Oil price
- BTC
- ETH
- IHSG previous close
- DXY optional
```

Post format:

```text
🇮🇩 Indonesia Market Pulse

USD/IDR: Rp xx.xxx
Gold: $x.xxx/oz
Oil: $xx.xx/barrel
BTC: $xx.xxx
IHSG: x.xxx

Focus: Rupiah, global risk sentiment, commodities, and central bank signals.
```

### 7.2 Indonesia in Global Headlines

Sources:

```text
- Reuters
- AP
- Bloomberg
- Nikkei Asia
- Financial Times
- CNBC
- Al Jazeera
- BBC
- The Guardian
- Official government sources
```

Topics:

```text
- Indonesia economy
- Rupiah
- Bank Indonesia
- Nickel
- EV battery
- Palm oil
- Coal
- Trade policy
- Fiscal policy
- Elections/political shifts
- Natural disasters
- ASEAN/geopolitics
```

### 7.3 Global News That Matters to Indonesia

Topics:

```text
- Fed rate
- US inflation
- US dollar movement
- China economy
- Oil price
- Global conflict
- Red Sea/shipping disruption
- Taiwan Strait
- Commodity supply shock
- Global recession signal
- Big tech/AI policy
```

### 7.4 Crypto Alerts

Topics:

```text
- BTC/ETH major move
- ETF flow
- Binance/Coinbase major news
- Stablecoin regulation
- Major exploit/hack
- Large liquidation event
- Funding rate extreme
- OJK/Bappebti Indonesia crypto policy
```

### 7.5 Emergency Alerts

Sources:

```text
- BMKG
- BNPB
- Official government accounts
- Verified news outlets
```

Topics:

```text
- Earthquake
- Tsunami warning
- Volcano eruption
- Flood
- Extreme weather
- Major transportation disruption
```

---

## 8. Data Source Strategy

### 8.1 Source Types

```text
- RSS feeds
- Public APIs
- Official websites
- Market data APIs
- News APIs
- Manual source list
```

### 8.2 Source Priority

```text
Priority 1: Official source
Priority 2: Major international media
Priority 3: Major Indonesian media
Priority 4: Market data provider
Priority 5: Social media signal only, not source of truth
```

### 8.3 Source Verification Rule

```text
1 official source = high confidence
2 reliable media sources = medium/high confidence
1 reliable media source = draft only unless low-risk
1 unknown source = skip
Social media only = skip or manual review
```

---

## 9. Go Project Structure

```text
x-info-bot/
  cmd/
    bot/
      main.go
    worker/
      main.go
    cli/
      main.go

  internal/
    config/
      config.go
      env.go

    logger/
      logger.go

    models/
      article.go
      source.go
      post.go
      score.go
      market.go
      review.go

    fetcher/
      fetcher.go
      rss_fetcher.go
      api_fetcher.go
      official_fetcher.go

    market/
      usdidr.go
      crypto.go
      gold.go
      oil.go
      ihsg.go

    normalize/
      normalize.go
      clean_html.go
      extract_text.go

    dedupe/
      hash.go
      similarity.go

    scoring/
      rules.go
      scorer.go

    ai/
      anthropic_client.go
      prompts.go
      review.go
      rewrite.go
      schema.go

    decision/
      decision.go
      policy.go

    publisher/
      x_client.go
      media_upload.go
      queue.go

    storage/
      d1.go
      sqlite.go
      r2.go
      kv.go

    image/
      renderer.go
      templates/
        market_pulse.go
        breaking_news.go

    scheduler/
      cron.go
      jobs.go

    server/
      http.go
      routes.go
      middleware.go

  cloudflare/
    workers/
      api/
        src/
          index.ts
        wrangler.toml
      cron/
        src/
          index.ts
        wrangler.toml

    d1/
      schema.sql
      migrations/

    pages/
      dashboard/

  prompts/
    claude_editor.md
    claude_scorer.md
    claude_rewriter.md
    claude_risk_filter.md

  scripts/
    dev.sh
    migrate.sh
    deploy-workers.sh
    deploy-pages.sh

  .env.example
  docker-compose.yml
  Dockerfile
  go.mod
  go.sum
  README.md
```

---

## 10. Database Schema

### sources

```sql
CREATE TABLE sources (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  url TEXT NOT NULL,
  type TEXT NOT NULL,
  category TEXT NOT NULL,
  reliability_score INTEGER DEFAULT 5,
  enabled BOOLEAN DEFAULT true,
  created_at TEXT NOT NULL
);
```

### articles

```sql
CREATE TABLE articles (
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
```

### draft_posts

```sql
CREATE TABLE draft_posts (
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
```

### published_posts

```sql
CREATE TABLE published_posts (
  id TEXT PRIMARY KEY,
  draft_id TEXT,
  x_post_id TEXT,
  content TEXT NOT NULL,
  media_urls TEXT,
  published_at TEXT NOT NULL,
  status TEXT NOT NULL
);
```

### market_snapshots

```sql
CREATE TABLE market_snapshots (
  id TEXT PRIMARY KEY,
  symbol TEXT NOT NULL,
  value TEXT NOT NULL,
  change_percent REAL,
  source TEXT,
  captured_at TEXT NOT NULL
);
```

### ai_reviews

```sql
CREATE TABLE ai_reviews (
  id TEXT PRIMARY KEY,
  draft_id TEXT NOT NULL,
  model TEXT NOT NULL,
  review_json TEXT NOT NULL,
  created_at TEXT NOT NULL
);
```

---

## 11. Core Go Interfaces

### Fetcher Interface

```go
type Fetcher interface {
    Name() string
    Fetch(ctx context.Context) ([]Article, error)
}
```

### AI Reviewer Interface

```go
type AIReviewer interface {
    ReviewDraft(ctx context.Context, input ReviewInput) (*ReviewResult, error)
    RewritePost(ctx context.Context, input RewriteInput) (*RewriteResult, error)
}
```

### Publisher Interface

```go
type Publisher interface {
    PublishText(ctx context.Context, content string) (*PublishResult, error)
    PublishWithMedia(ctx context.Context, content string, mediaPaths []string) (*PublishResult, error)
}
```

### Storage Interface

```go
type Storage interface {
    SaveArticle(ctx context.Context, article Article) error
    SaveDraft(ctx context.Context, draft DraftPost) error
    GetPendingDrafts(ctx context.Context) ([]DraftPost, error)
    MarkPublished(ctx context.Context, draftID string, result PublishResult) error
}
```

---

## 12. AI JSON Output Schema

Claude should return JSON only.

```json
{
  "approved": true,
  "safe_to_auto_post": false,
  "requires_manual_approval": true,
  "risk_level": "low",
  "category": "market",
  "scores": {
    "indonesia_relevance": 8,
    "global_importance": 6,
    "market_impact": 7,
    "urgency": 5,
    "public_interest": 7,
    "source_confidence": 8,
    "total_score": 41
  },
  "issues": [],
  "suggested_post": "Post content here",
  "suggested_thread": [],
  "why_it_matters": "Short explanation",
  "source_notes": "Source is official / reliable / needs confirmation"
}
```

---

## 13. Decision Engine Rules

```go
if Review.RiskLevel == "high" {
    return ManualApproval
}

if Review.SourceConfidence < 7 {
    return ManualApproval
}

if Review.TotalScore < 28 {
    return Skip
}

if Review.TotalScore >= 42 && Review.SafeToAutoPost && !Review.RequiresManualApproval {
    return AutoPost
}

return DraftForApproval
```

---

## 14. Cron Schedule

### Cloudflare Cron

```text
Every 15 minutes:
- Fetch breaking news
- Fetch market alerts
- Score and draft

Every morning 07:00 WIB:
- Daily market pulse
- What you should know today

Every evening 18:00 WIB:
- Evening recap

Every hour:
- Check USD/IDR, BTC, gold, oil movement triggers
```

Example Worker cron payload to VPS:

```json
{
  "job": "fetch_breaking_news",
  "source": "cloudflare_cron",
  "timestamp": "2026-05-22T07:00:00+07:00"
}
```

---

## 15. Environment Variables

```env
APP_ENV=production
APP_PORT=8080
APP_BASE_URL=https://bot.yourdomain.com

ANTHROPIC_API_KEY=
ANTHROPIC_MODEL=claude-sonnet-4-5

X_API_KEY=
X_API_SECRET=
X_ACCESS_TOKEN=
X_ACCESS_SECRET=
X_BEARER_TOKEN=

CLOUDFLARE_ACCOUNT_ID=
CLOUDFLARE_R2_ACCESS_KEY_ID=
CLOUDFLARE_R2_SECRET_ACCESS_KEY=
CLOUDFLARE_R2_BUCKET_MEDIA=x-info-bot-media
CLOUDFLARE_R2_BUCKET_ARCHIVES=x-info-bot-archives
CLOUDFLARE_R2_ENDPOINT=

DATABASE_URL=
ADMIN_API_KEY=
POSTING_MODE=manual
MIN_AUTO_POST_SCORE=42
```

---

## 16. MVP Scope

### MVP 1 — Safe Manual Bot

Build:

```text
- Go backend
- RSS/news fetcher
- Market data fetcher
- Deduplication
- Claude review
- Draft generation
- Store drafts
- Simple admin dashboard
- Manual approve/reject
- Publish to X API
- R2 image upload
```

Do not auto-post sensitive news yet.

### MVP 2 — Semi-Auto Bot

Add:

```text
- Auto daily market pulse
- Auto USD/IDR update
- Auto official announcement post
- Better dashboard
- Post scheduling
- Thread generation
- Image templates
```

### MVP 3 — Full Editorial System

Add:

```text
- Multi-source verification
- Trending topic detection
- Engagement analytics
- A/B hook testing
- Category-based posting schedule
- Emergency alert mode
- Telegram/Discord approval bot
```

---

## 17. Suggested First Build Order

```text
1. Create Go project structure
2. Implement config/env loader
3. Implement source model and article model
4. Implement RSS fetcher
5. Implement article normalizer
6. Implement dedupe hash
7. Implement Claude review client
8. Implement draft generator
9. Store drafts in SQLite first
10. Build X publisher
11. Build manual approval endpoint
12. Add R2 media upload
13. Move metadata to Cloudflare D1 if needed
14. Add Cloudflare Worker cron trigger
15. Add Cloudflare Pages dashboard
```

For development, use SQLite first because it is faster to build and debug. Later sync or migrate to D1.

---

## 18. Dashboard Pages

```text
/login
/dashboard
/drafts
/drafts/:id
/published
/sources
/settings
/logs
```

Draft detail page should show:

```text
- Original source
- Article title
- Article summary
- Generated post
- Claude review
- Scores
- Risk level
- Buttons: Approve, Reject, Rewrite, Publish Now
```

---

## 19. Image Generation

Use Go to generate simple branded images.

Image types:

```text
- Daily market pulse card
- Breaking news card
- Indonesia in global headlines card
- Crypto market alert card
```

Store generated PNG/JPG in Cloudflare R2.

Post can include:

```text
- Text-only for fast breaking news
- Image card for daily market pulse
- Image + thread for major updates
```

---

## 20. Safety Rules

The bot must never:

```text
- Post financial advice like "buy", "sell", "long", "short"
- Post rumors as facts
- Use copyrighted article text too heavily
- Repost full article content
- Publish casualty numbers without reliable source
- Publish sensitive political claims without manual approval
- Spam repeated similar posts
```

The bot should always:

```text
- Summarize in original wording
- Attribute source when appropriate
- Use cautious wording for developing stories
- Prefer official source for emergency/policy updates
- Keep source URL internally stored
```

---

## 21. Example Prompt for Claude Editor

```text
You are the senior editor and risk analyst for an Indonesia-aware breaking information account.

Review the draft post below.

Goals:
- Keep the post short, clear, neutral, and useful.
- Avoid hype and clickbait.
- Avoid financial advice.
- Avoid unsupported claims.
- Add Indonesia context when relevant.
- Decide whether the post is safe to auto-publish, requires manual approval, or should be skipped.

Return JSON only using this schema:
{
  "approved": boolean,
  "safe_to_auto_post": boolean,
  "requires_manual_approval": boolean,
  "risk_level": "low|medium|high",
  "category": string,
  "scores": {
    "indonesia_relevance": number,
    "global_importance": number,
    "market_impact": number,
    "urgency": number,
    "public_interest": number,
    "source_confidence": number,
    "total_score": number
  },
  "issues": string[],
  "suggested_post": string,
  "suggested_thread": string[],
  "why_it_matters": string,
  "source_notes": string
}

Input:
{{INPUT_JSON}}
```

---

## 22. Deployment

### VPS

Use VPS for Go backend:

```text
- Ubuntu 24.04
- Go latest stable
- Docker optional
- Caddy/Nginx reverse proxy
- systemd service
```

### Cloudflare

Use Cloudflare for:

```text
- DNS
- SSL
- Proxy
- R2
- Worker cron
- Worker API
- Pages dashboard
- Access protection
```

### Domain Example

```text
bot.yourdomain.com       -> Go backend on VPS via Cloudflare proxy
dashboard.yourdomain.com -> Cloudflare Pages dashboard
media.yourdomain.com     -> R2 public/custom domain
api.yourdomain.com       -> Cloudflare Worker API
```

---

## 23. Final MVP Success Criteria

The MVP is successful if:

```text
- Bot can fetch articles from configured sources
- Bot can deduplicate articles
- Bot can generate draft posts
- Claude can review and score drafts
- User can approve/reject posts
- Bot can publish approved posts to X
- Daily USD/IDR market pulse can run automatically
- All media assets are stored in R2
- Logs and post history are stored
```

---

## 24. Recommended Initial Strategy

Do not force 100% Cloudflare-only from the beginning.

Use this hybrid approach first:

```text
Go core on VPS
+ Cloudflare R2 for media
+ Cloudflare DNS/SSL/proxy
+ Cloudflare Worker cron trigger
+ Cloudflare Pages dashboard
+ SQLite first for local development
+ D1 or Postgres later if needed
```

This is the most realistic, scalable, and fast-to-build approach.
