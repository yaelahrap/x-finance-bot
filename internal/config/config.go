// Package config provides configuration loading and access for the bot.
//
// The Config struct is the single source of truth for runtime configuration.
// Construct it once via Load() in main and pass it down explicitly; do not
// rely on global state.
package config

import (
	"errors"
	"fmt"
	"strings"

	"github.com/joho/godotenv"
)

// AppConfig holds top-level application settings.
type AppConfig struct {
	// Env is the deployment environment ("development", "staging", "production").
	Env string
	// Port is the HTTP port the admin/control server binds to.
	Port int
	// BaseURL is the public-facing base URL of the bot (used for callbacks/links).
	BaseURL string
}

// AnthropicConfig holds Claude API credentials and model selection.
type AnthropicConfig struct {
	APIKey string
	// Model is the Anthropic model identifier (default: claude-sonnet-4-5).
	Model string
}

// XConfig holds X (Twitter) API v2 OAuth 1.0a + bearer credentials.
type XConfig struct {
	APIKey       string
	APISecret    string
	AccessToken  string
	AccessSecret string
	BearerToken  string
}

// CloudflareConfig holds Cloudflare account + R2 storage settings.
type CloudflareConfig struct {
	AccountID         string
	R2AccessKeyID     string
	R2SecretAccessKey string
	R2BucketMedia     string
	R2BucketArchives  string
	R2Endpoint        string
}

// DatabaseConfig holds the database DSN.
type DatabaseConfig struct {
	// URL is the database connection string. Defaults to a local SQLite file
	// with WAL journaling for the MVP.
	URL string
}

// BotConfig holds bot-behavior tuning knobs.
type BotConfig struct {
	// PostingMode controls how posts reach X: "manual", "semi-auto", or "auto".
	// Default "manual" for safety.
	PostingMode string
	// MinAutoPostScore is the minimum decision score required to auto-publish
	// when PostingMode is "auto" or "semi-auto".
	MinAutoPostScore int
	// AdminAPIKey is the bearer token protecting the admin/control API.
	AdminAPIKey string
}

// Config is the typed, validated configuration for the entire bot.
type Config struct {
	App        AppConfig
	Anthropic  AnthropicConfig
	X          XConfig
	Cloudflare CloudflareConfig
	Database   DatabaseConfig
	Bot        BotConfig
}

// IsProduction reports whether the bot is running in the production environment.
func (c *Config) IsProduction() bool {
	return c.App.Env == "production"
}

// PostingMode constants.
const (
	PostingModeManual   = "manual"
	PostingModeSemiAuto = "semi-auto"
	PostingModeAuto     = "auto"
)

// Defaults.
const (
	defaultAppEnv           = "development"
	defaultAppPort          = 8080
	defaultAnthropicModel   = "claude-sonnet-4-5"
	defaultDatabaseURL      = "file:./data/bot.db?cache=shared&_journal=WAL"
	defaultPostingMode      = PostingModeManual
	defaultMinAutoPostScore = 42
	defaultR2BucketMedia    = "x-info-bot-media"
	defaultR2BucketArchives = "x-info-bot-archives"
)

// Load reads configuration from the process environment, optionally seeded by a
// .env file in the working directory, and returns a populated *Config.
//
// Load returns an error listing every missing required variable. ANTHROPIC_API_KEY
// and ADMIN_API_KEY are always required. X and Cloudflare credentials are
// required only when APP_ENV=production.
func Load() (*Config, error) {
	// Best-effort .env load. Production deployments typically inject env vars
	// directly, so a missing .env is not an error.
	_ = godotenv.Load()

	cfg := &Config{
		App: AppConfig{
			Env:     getEnv("APP_ENV", defaultAppEnv),
			Port:    getEnvInt("APP_PORT", defaultAppPort),
			BaseURL: getEnv("APP_BASE_URL", ""),
		},
		Anthropic: AnthropicConfig{
			APIKey: getEnv("ANTHROPIC_API_KEY", ""),
			Model:  getEnv("ANTHROPIC_MODEL", defaultAnthropicModel),
		},
		X: XConfig{
			APIKey:       getEnv("X_API_KEY", ""),
			APISecret:    getEnv("X_API_SECRET", ""),
			AccessToken:  getEnv("X_ACCESS_TOKEN", ""),
			AccessSecret: getEnv("X_ACCESS_SECRET", ""),
			BearerToken:  getEnv("X_BEARER_TOKEN", ""),
		},
		Cloudflare: CloudflareConfig{
			AccountID:         getEnv("CLOUDFLARE_ACCOUNT_ID", ""),
			R2AccessKeyID:     getEnv("CLOUDFLARE_R2_ACCESS_KEY_ID", ""),
			R2SecretAccessKey: getEnv("CLOUDFLARE_R2_SECRET_ACCESS_KEY", ""),
			R2BucketMedia:     getEnv("CLOUDFLARE_R2_BUCKET_MEDIA", defaultR2BucketMedia),
			R2BucketArchives:  getEnv("CLOUDFLARE_R2_BUCKET_ARCHIVES", defaultR2BucketArchives),
			R2Endpoint:        getEnv("CLOUDFLARE_R2_ENDPOINT", ""),
		},
		Database: DatabaseConfig{
			URL: getEnv("DATABASE_URL", defaultDatabaseURL),
		},
		Bot: BotConfig{
			PostingMode:      getEnv("POSTING_MODE", defaultPostingMode),
			MinAutoPostScore: getEnvInt("MIN_AUTO_POST_SCORE", defaultMinAutoPostScore),
			AdminAPIKey:      getEnv("ADMIN_API_KEY", ""),
		},
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// validate enforces required-variable and value constraints.
func (c *Config) validate() error {
	var missing []string

	// Always-required variables.
	if c.Anthropic.APIKey == "" {
		missing = append(missing, "ANTHROPIC_API_KEY")
	}
	if c.Bot.AdminAPIKey == "" {
		missing = append(missing, "ADMIN_API_KEY")
	}

	// Production-only requirements.
	if c.IsProduction() {
		if c.X.APIKey == "" {
			missing = append(missing, "X_API_KEY")
		}
		if c.X.APISecret == "" {
			missing = append(missing, "X_API_SECRET")
		}
		if c.X.AccessToken == "" {
			missing = append(missing, "X_ACCESS_TOKEN")
		}
		if c.X.AccessSecret == "" {
			missing = append(missing, "X_ACCESS_SECRET")
		}
		if c.X.BearerToken == "" {
			missing = append(missing, "X_BEARER_TOKEN")
		}
		if c.Cloudflare.AccountID == "" {
			missing = append(missing, "CLOUDFLARE_ACCOUNT_ID")
		}
		if c.Cloudflare.R2AccessKeyID == "" {
			missing = append(missing, "CLOUDFLARE_R2_ACCESS_KEY_ID")
		}
		if c.Cloudflare.R2SecretAccessKey == "" {
			missing = append(missing, "CLOUDFLARE_R2_SECRET_ACCESS_KEY")
		}
		if c.Cloudflare.R2Endpoint == "" {
			missing = append(missing, "CLOUDFLARE_R2_ENDPOINT")
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing required environment variable(s): %s", strings.Join(missing, ", "))
	}

	switch c.Bot.PostingMode {
	case PostingModeManual, PostingModeSemiAuto, PostingModeAuto:
		// ok
	default:
		return fmt.Errorf("invalid POSTING_MODE %q: expected one of %q, %q, %q",
			c.Bot.PostingMode, PostingModeManual, PostingModeSemiAuto, PostingModeAuto)
	}

	if c.App.Port <= 0 || c.App.Port > 65535 {
		return fmt.Errorf("invalid APP_PORT %d: must be between 1 and 65535", c.App.Port)
	}

	if c.Bot.MinAutoPostScore < 0 || c.Bot.MinAutoPostScore > 100 {
		return fmt.Errorf("invalid MIN_AUTO_POST_SCORE %d: must be between 0 and 100", c.Bot.MinAutoPostScore)
	}

	return nil
}

// ErrMissingRequired is returned when one or more required env vars are unset.
// Use errors.Is to detect; the wrapped message contains the variable names.
var ErrMissingRequired = errors.New("missing required environment variable")
