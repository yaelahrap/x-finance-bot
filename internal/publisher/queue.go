package publisher

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// QueueItem represents a post waiting to be published.
type QueueItem struct {
	DraftID  string
	Content  string
	MediaIDs []string
}

// Queue manages a FIFO queue of posts to be published with rate limiting.
type Queue struct {
	mu        sync.Mutex
	items     []QueueItem
	publisher Publisher
	logger    *slog.Logger
	interval  time.Duration // minimum time between posts
}

// NewQueue creates a post queue with the given publisher and rate limit interval.
func NewQueue(pub Publisher, logger *slog.Logger, interval time.Duration) *Queue {
	if interval == 0 {
		interval = 2 * time.Minute // default: max 30 posts/hour
	}
	return &Queue{
		publisher: pub,
		logger:    logger,
		interval:  interval,
	}
}

// Enqueue adds a post to the publish queue.
func (q *Queue) Enqueue(item QueueItem) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.items = append(q.items, item)
}

// Len returns the current queue length.
func (q *Queue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.items)
}

// Dequeue removes and returns the next item, or nil if empty.
func (q *Queue) Dequeue() *QueueItem {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.items) == 0 {
		return nil
	}
	item := q.items[0]
	q.items = q.items[1:]
	return &item
}

// ProcessNext publishes the next item in the queue. Returns nil if queue is empty.
func (q *Queue) ProcessNext(ctx context.Context) (*PublishResult, error) {
	item := q.Dequeue()
	if item == nil {
		return nil, nil
	}

	q.logger.Info("publishing from queue",
		"draft_id", item.DraftID,
		"has_media", len(item.MediaIDs) > 0,
	)

	var result *PublishResult
	var err error

	if len(item.MediaIDs) > 0 {
		result, err = q.publisher.PublishWithMedia(ctx, item.Content, item.MediaIDs)
	} else {
		result, err = q.publisher.PublishText(ctx, item.Content)
	}

	if err != nil {
		q.logger.Error("publish failed", "draft_id", item.DraftID, "error", err)
		return nil, err
	}

	q.logger.Info("published successfully",
		"draft_id", item.DraftID,
		"post_id", result.PostID,
	)

	return result, nil
}

// Drain processes all items in the queue with rate limiting between posts.
// It stops on context cancellation.
func (q *Queue) Drain(ctx context.Context) ([]PublishResult, error) {
	var results []PublishResult

	for {
		select {
		case <-ctx.Done():
			return results, ctx.Err()
		default:
		}

		result, err := q.ProcessNext(ctx)
		if err != nil {
			return results, err
		}
		if result == nil {
			// Queue empty
			return results, nil
		}
		results = append(results, *result)

		// Rate limit
		select {
		case <-ctx.Done():
			return results, ctx.Err()
		case <-time.After(q.interval):
		}
	}
}
