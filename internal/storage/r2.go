package storage

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
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
	publicURL   string
	httpClient  *http.Client
}

// R2Config holds the configuration for the R2 client.
type R2Config struct {
	Endpoint        string
	AccessKeyID     string
	SecretAccessKey string
	BucketMedia     string
	PublicURL       string
}

// NewR2Client creates a new Cloudflare R2 client.
func NewR2Client(cfg R2Config) *R2Client {
	return &R2Client{
		endpoint:    cfg.Endpoint,
		accessKeyID: cfg.AccessKeyID,
		secretKey:   cfg.SecretAccessKey,
		bucketMedia: cfg.BucketMedia,
		publicURL:   cfg.PublicURL,
		httpClient:  &http.Client{Timeout: 60 * time.Second},
	}
}

// Upload stores data in R2 at the given object key and returns the public URL.
func (r *R2Client) Upload(ctx context.Context, key string, data []byte, contentType string) (string, error) {
	uploadURL := fmt.Sprintf("%s/%s/%s", r.endpoint, r.bucketMedia, key)

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, uploadURL, bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("r2 upload create request: %w", err)
	}

	req.Header.Set("Content-Type", contentType)
	req.ContentLength = int64(len(data))

	// AWS Signature Version 4 for Cloudflare R2 authentication
	r.signRequest(req)

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("r2 upload request execution: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return "", fmt.Errorf("r2 upload failed (status %d): %s", resp.StatusCode, string(body))
	}

	return r.ObjectURL(key), nil
}

// ObjectURL returns the public URL for an object in the media bucket.
func (r *R2Client) ObjectURL(key string) string {
	if r.publicURL != "" {
		return fmt.Sprintf("%s/%s", r.publicURL, key)
	}
	return fmt.Sprintf("%s/%s/%s", r.endpoint, r.bucketMedia, key)
}

// signRequest signs the HTTP request using AWS Signature Version 4.
func (r *R2Client) signRequest(req *http.Request) {
	region := "auto"
	service := "s3"

	now := time.Now().UTC()
	amzDate := now.Format("20060102T150405Z")
	dateStamp := now.Format("20060102")

	req.Header.Set("X-Amz-Date", amzDate)

	payloadHash := "UNSIGNED-PAYLOAD"
	req.Header.Set("X-Amz-Content-Sha256", payloadHash)

	host := req.URL.Host
	req.Header.Set("Host", host)

	canonicalURI := req.URL.EscapedPath()
	canonicalQuery := ""

	// We sign the host, x-amz-content-sha256, and x-amz-date headers.
	canonicalHeaders := fmt.Sprintf("host:%s\nx-amz-content-sha256:%s\nx-amz-date:%s\n", host, payloadHash, amzDate)
	signedHeaders := "host;x-amz-content-sha256;x-amz-date"

	canonicalReq := fmt.Sprintf("%s\n%s\n%s\n%s\n%s\n%s",
		req.Method,
		canonicalURI,
		canonicalQuery,
		canonicalHeaders,
		signedHeaders,
		payloadHash,
	)

	hReq := sha256.New()
	hReq.Write([]byte(canonicalReq))
	hashedCanonicalReq := hex.EncodeToString(hReq.Sum(nil))

	credentialScope := fmt.Sprintf("%s/%s/%s/aws4_request", dateStamp, region, service)
	stringToSign := fmt.Sprintf("AWS4-HMAC-SHA256\n%s\n%s\n%s",
		amzDate,
		credentialScope,
		hashedCanonicalReq,
	)

	// Signing Key derivation
	hDate := hmacSHA256([]byte("AWS4"+r.secretKey), []byte(dateStamp))
	hRegion := hmacSHA256(hDate, []byte(region))
	hService := hmacSHA256(hRegion, []byte(service))
	hSigning := hmacSHA256(hService, []byte("aws4_request"))

	signature := hex.EncodeToString(hmacSHA256(hSigning, []byte(stringToSign)))

	authHeader := fmt.Sprintf("AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		r.accessKeyID,
		credentialScope,
		signedHeaders,
		signature,
	)
	req.Header.Set("Authorization", authHeader)
}

func hmacSHA256(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}
