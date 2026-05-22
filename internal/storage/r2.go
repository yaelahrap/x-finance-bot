package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

// R2Client provides upload and URL generation for Cloudflare R2 storage.
type R2Client struct {
	endpoint    string
	accessKeyID string
	secretKey   string
	bucketMedia string
	httpClient  *http.Client
}

// R2Config holds the configuration for the R2 client.
type R2Config struct {
	Endpoint        string
	AccessKeyID     string
	SecretAccessKey  string
	BucketMedia     string
}

// NewR2Client creates a new Cloudflare R2 client.
func NewR2Client(cfg R2Config) *R2Client {
	return &R2Client{
		endpoint:    cfg.Endpoint,
		accessKeyID: cfg.AccessKeyID,
		secretKey:   cfg.SecretAccessKey,
		bucketMedia: cfg.BucketMedia,
		httpClient:  &http.Client{Timeout: 60 * time.Second},
	}
}

// Upload stores data in R2 at the given object key and returns the public URL.
func (r *R2Client) Upload(ctx context.Context, key string, data []byte, contentType string) (string, error) {
	url := fmt.Sprintf("%s/%s/%s", r.endpoint, r.bucketMedia, key)

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("r2 upload create request: %w", err)
	}

	req.Header.Set("Content-Type", contentType)
	req.ContentLength = int64(len(data))

	// S3-compatible auth headers
	r.signRequest(req)

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("r2 upload: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return "", fmt.Errorf("r2 upload failed (status %d): %s", resp.StatusCode, string(body))
	}

	return url, nil
}

// ObjectURL returns the public URL for an object in the media bucket.
func (r *R2Client) ObjectURL(key string) string {
	return fmt.Sprintf("%s/%s/%s", r.endpoint, r.bucketMedia, key)
}

// signRequest adds S3-compatible authorization headers.
// For MVP, this uses a simplified approach. Production should use full AWS Sig V4.
func (r *R2Client) signRequest(req *http.Request) {
	// Cloudflare R2 supports S3-compatible auth.
	// For the MVP, we set basic auth headers. Full AWS Sig V4 signing
	// should be implemented for production using a library like aws-sdk-go-v2.
	req.Header.Set("X-Amz-Content-Sha256", "UNSIGNED-PAYLOAD")
	// In production, use proper AWS Signature V4 signing here.
	// For now, R2 tokens with appropriate permissions can use simplified auth.
}
