package publisher

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/raflyramadhan/x-finance-bot/internal/models"
)

const (
	xAPIBaseURL    = "https://api.x.com/2"
	xUploadBaseURL = "https://upload.twitter.com/1.1"
)

// XClient implements Publisher using the X API v2 with OAuth 1.0a.
type XClient struct {
	apiKey       string
	apiSecret    string
	accessToken  string
	accessSecret string
	httpClient   *http.Client
}

// XClientConfig holds the credentials for the X API client.
type XClientConfig struct {
	APIKey       string
	APISecret    string
	AccessToken  string
	AccessSecret string
}

// NewXClient creates a new X/Twitter API client.
func NewXClient(cfg XClientConfig) *XClient {
	return &XClient{
		apiKey:       cfg.APIKey,
		apiSecret:    cfg.APISecret,
		accessToken:  cfg.AccessToken,
		accessSecret: cfg.AccessSecret,
		httpClient:   &http.Client{Timeout: 30 * time.Second},
	}
}

// PublishText posts a text-only tweet.
func (c *XClient) PublishText(ctx context.Context, content string) (*PublishResult, error) {
	return c.createTweet(ctx, content, nil)
}

// PublishWithMedia posts a tweet with media attachments.
func (c *XClient) PublishWithMedia(ctx context.Context, content string, mediaIDs []string) (*PublishResult, error) {
	return c.createTweet(ctx, content, mediaIDs)
}

func (c *XClient) createTweet(ctx context.Context, content string, mediaIDs []string) (*PublishResult, error) {
	endpoint := xAPIBaseURL + "/tweets"

	payload := map[string]interface{}{
		"text": content,
	}
	if len(mediaIDs) > 0 {
		payload["media"] = map[string]interface{}{
			"media_ids": mediaIDs,
		}
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal tweet payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	if err := c.signRequest(req); err != nil {
		return nil, fmt.Errorf("sign request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("post tweet: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return &PublishResult{
			Status: models.PublishStatusFailed,
			Error:  fmt.Sprintf("X API error (status %d): %s", resp.StatusCode, string(respBody)),
		}, nil
	}

	var tweetResp struct {
		Data struct {
			ID   string `json:"id"`
			Text string `json:"text"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &tweetResp); err != nil {
		return nil, fmt.Errorf("unmarshal tweet response: %w", err)
	}

	return &PublishResult{
		PostID: tweetResp.Data.ID,
		Status: models.PublishStatusSuccess,
		URL:    fmt.Sprintf("https://x.com/i/status/%s", tweetResp.Data.ID),
	}, nil
}

// signRequest adds OAuth 1.0a signature to the request.
func (c *XClient) signRequest(req *http.Request) error {
	nonce := generateNonce()
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)

	params := map[string]string{
		"oauth_consumer_key":     c.apiKey,
		"oauth_nonce":            nonce,
		"oauth_signature_method": "HMAC-SHA1",
		"oauth_timestamp":        timestamp,
		"oauth_token":            c.accessToken,
		"oauth_version":          "1.0",
	}

	// Build signature base string
	paramStr := buildParamString(params)
	baseStr := strings.ToUpper(req.Method) + "&" +
		percentEncode(strings.Split(req.URL.String(), "?")[0]) + "&" +
		percentEncode(paramStr)

	// Sign
	signingKey := percentEncode(c.apiSecret) + "&" + percentEncode(c.accessSecret)
	signature := computeHMACSHA1(signingKey, baseStr)

	params["oauth_signature"] = signature

	// Build Authorization header
	var authParts []string
	for k, v := range params {
		authParts = append(authParts, fmt.Sprintf(`%s="%s"`, k, percentEncode(v)))
	}
	sort.Strings(authParts)
	req.Header.Set("Authorization", "OAuth "+strings.Join(authParts, ", "))

	return nil
}

func buildParamString(params map[string]string) string {
	var keys []string
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var parts []string
	for _, k := range keys {
		parts = append(parts, percentEncode(k)+"="+percentEncode(params[k]))
	}
	return strings.Join(parts, "&")
}

func percentEncode(s string) string {
	return url.QueryEscape(s)
}

func computeHMACSHA1(key, data string) string {
	h := hmac.New(sha1.New, []byte(key))
	h.Write([]byte(data))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

func generateNonce() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}
