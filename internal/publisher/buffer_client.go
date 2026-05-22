// Package publisher provides posting clients for X/Twitter.
// This file contains the Buffer GraphQL API client, which handles publishing
// via Buffer (https://buffer.com) instead of the X API directly.
package publisher

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/raflyramadhan/x-finance-bot/internal/models"
)

const bufferGraphQLEndpoint = "https://api.buffer.com"

// BufferClient publishes posts to X/Twitter via the Buffer GraphQL API.
// It implements the Publisher interface.
type BufferClient struct {
	apiKey     string
	channelID  string
	httpClient *http.Client
}

// NewBufferClient creates a new BufferClient.
//   - apiKey:    Buffer Bearer token from https://publish.buffer.com/settings/api
//   - channelID: Buffer channel ID for the connected X/Twitter account
func NewBufferClient(apiKey, channelID string) *BufferClient {
	return &BufferClient{
		apiKey:    apiKey,
		channelID: channelID,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// bufferRequest executes a GraphQL request against the Buffer API.
func (c *BufferClient) bufferRequest(ctx context.Context, query string, variables map[string]any) (map[string]any, error) {
	payload := map[string]any{"query": query}
	if variables != nil {
		payload["variables"] = variables
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("buffer: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, bufferGraphQLEndpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("buffer: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("buffer: do request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("buffer: read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("buffer: HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	var result map[string]any
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("buffer: unmarshal response: %w", err)
	}

	// Surface GraphQL-level errors
	if errs, ok := result["errors"]; ok {
		errBytes, _ := json.Marshal(errs)
		return nil, fmt.Errorf("buffer: GraphQL error: %s", string(errBytes))
	}

	return result, nil
}

// createPost sends a createPost mutation to Buffer.
// imageURL is optional — pass "" for text-only posts.
func (c *BufferClient) createPost(ctx context.Context, text, imageURL string) (*PublishResult, error) {
	// Build assets array if image URL provided
	assetsGQL := ""
	if imageURL != "" {
		assetsGQL = fmt.Sprintf(`
      assets: [
        {
          image: {
            url: %q
          }
        }
      ]`, imageURL)
	}

	mutation := fmt.Sprintf(`
mutation CreatePost {
  createPost(
    input: {
      text: %q
      channelId: %q
      schedulingType: automatic
      mode: addToQueue
      %s
    }
  ) {
    ... on PostActionSuccess {
      post {
        id
        text
        status
      }
    }
    ... on MutationError {
      message
    }
  }
}`, text, c.channelID, assetsGQL)

	result, err := c.bufferRequest(ctx, mutation, nil)
	if err != nil {
		return nil, fmt.Errorf("buffer createPost: %w", err)
	}

	// Navigate the response
	data, _ := result["data"].(map[string]any)
	createPost, _ := data["createPost"].(map[string]any)

	// Check for MutationError
	if msg, ok := createPost["message"]; ok {
		return nil, fmt.Errorf("buffer createPost mutation error: %v", msg)
	}

	// Extract PostActionSuccess
	post, _ := createPost["post"].(map[string]any)
	if post == nil {
		return nil, fmt.Errorf("buffer createPost: unexpected response shape: %v", result)
	}

	postID, _ := post["id"].(string)
	return &PublishResult{
		PostID: postID,
		Status: models.PublishStatusSuccess,
		URL:    fmt.Sprintf("https://twitter.com/i/web/status/%s", postID),
	}, nil
}

// PublishText posts a text-only tweet via Buffer (implements Publisher).
func (c *BufferClient) PublishText(ctx context.Context, content string) (*PublishResult, error) {
	return c.createPost(ctx, content, "")
}

// PublishWithMedia posts a tweet with an attached image via Buffer (implements Publisher).
// mediaIDs here are treated as image URLs (R2 public URLs), not X media IDs.
func (c *BufferClient) PublishWithMedia(ctx context.Context, content string, mediaIDs []string) (*PublishResult, error) {
	imageURL := ""
	if len(mediaIDs) > 0 {
		imageURL = mediaIDs[0] // Buffer only supports one image for X posts
	}
	return c.createPost(ctx, content, imageURL)
}

// DeletePost removes a previously published post via Buffer (implements Publisher).
// postID is the Buffer post ID returned during publish.
func (c *BufferClient) DeletePost(ctx context.Context, postID string) error {
	mutation := fmt.Sprintf(`
mutation DeletePost {
  deletePost(input: { postId: %q }) {
    ... on PostActionSuccess {
      post {
        id
      }
    }
    ... on MutationError {
      message
    }
  }
}`, postID)

	result, err := c.bufferRequest(ctx, mutation, nil)
	if err != nil {
		return fmt.Errorf("buffer deletePost: %w", err)
	}

	data, _ := result["data"].(map[string]any)
	deletePost, _ := data["deletePost"].(map[string]any)
	if msg, ok := deletePost["message"]; ok {
		return fmt.Errorf("buffer deletePost mutation error: %v", msg)
	}

	return nil
}
