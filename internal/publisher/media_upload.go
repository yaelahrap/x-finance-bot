package publisher

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
)

// MediaUploader handles uploading media files to X's upload API.
type MediaUploader struct {
	client *XClient
}

// NewMediaUploader creates a media uploader backed by the X client.
func NewMediaUploader(client *XClient) *MediaUploader {
	return &MediaUploader{client: client}
}

// UploadFile uploads a local file to X and returns the media ID.
func (u *MediaUploader) UploadFile(ctx context.Context, filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("open media file: %w", err)
	}
	defer f.Close()

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	part, err := writer.CreateFormFile("media", filepath.Base(filePath))
	if err != nil {
		return "", fmt.Errorf("create form file: %w", err)
	}

	if _, err := io.Copy(part, f); err != nil {
		return "", fmt.Errorf("copy file data: %w", err)
	}

	if err := writer.Close(); err != nil {
		return "", fmt.Errorf("close multipart writer: %w", err)
	}

	endpoint := xUploadBaseURL + "/media/upload.json"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, &buf)
	if err != nil {
		return "", fmt.Errorf("create upload request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	if err := u.client.signRequest(req); err != nil {
		return "", fmt.Errorf("sign upload request: %w", err)
	}

	resp, err := u.client.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("upload media: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1*1024*1024))
	if err != nil {
		return "", fmt.Errorf("read upload response: %w", err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("upload failed (status %d): %s", resp.StatusCode, string(respBody))
	}

	var uploadResp struct {
		MediaID       int64  `json:"media_id"`
		MediaIDString string `json:"media_id_string"`
	}
	if err := json.Unmarshal(respBody, &uploadResp); err != nil {
		return "", fmt.Errorf("unmarshal upload response: %w", err)
	}

	return uploadResp.MediaIDString, nil
}

// UploadBytes uploads raw bytes as media to X and returns the media ID.
func (u *MediaUploader) UploadBytes(ctx context.Context, data []byte, filename string) (string, error) {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	part, err := writer.CreateFormFile("media", filename)
	if err != nil {
		return "", fmt.Errorf("create form file: %w", err)
	}

	if _, err := part.Write(data); err != nil {
		return "", fmt.Errorf("write data: %w", err)
	}

	if err := writer.Close(); err != nil {
		return "", fmt.Errorf("close multipart writer: %w", err)
	}

	endpoint := xUploadBaseURL + "/media/upload.json"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, &buf)
	if err != nil {
		return "", fmt.Errorf("create upload request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	if err := u.client.signRequest(req); err != nil {
		return "", fmt.Errorf("sign upload request: %w", err)
	}

	resp, err := u.client.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("upload media: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1*1024*1024))
	if err != nil {
		return "", fmt.Errorf("read upload response: %w", err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("upload failed (status %d): %s", resp.StatusCode, string(respBody))
	}

	var uploadResp struct {
		MediaID       int64  `json:"media_id"`
		MediaIDString string `json:"media_id_string"`
	}
	if err := json.Unmarshal(respBody, &uploadResp); err != nil {
		return "", fmt.Errorf("unmarshal upload response: %w", err)
	}

	return uploadResp.MediaIDString, nil
}
