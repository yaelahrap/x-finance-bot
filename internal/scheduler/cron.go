package scheduler

import (
	"context"
	"log/slog"
	"time"
)

// Job represents a scheduled task.
type Job struct {
	Name     string
	Schedule string // cron expression or interval description
	Fn       func(ctx context.Context) error
}

// Scheduler manages periodic job execution.
type Scheduler struct {
	jobs   []scheduledJob
	logger *slog.Logger
	done   chan struct{}
}

type scheduledJob struct {
	job      Job
	interval time.Duration
}

// New creates a new scheduler.
func New(logger *slog.Logger) *Scheduler {
	return &Scheduler{
		logger: logger,
		done:   make(chan struct{}),
	}
}

// Register adds a job to run at the given interval.
func (s *Scheduler) Register(job Job, interval time.Duration) {
	s.jobs = append(s.jobs, scheduledJob{job: job, interval: interval})
}

// Start begins executing all registered jobs. It blocks until Stop is called
// or the context is cancelled.
func (s *Scheduler) Start(ctx context.Context) {
	for _, sj := range s.jobs {
		go s.runJob(ctx, sj)
	}

	select {
	case <-ctx.Done():
	case <-s.done:
	}
}

// Stop signals the scheduler to stop.
func (s *Scheduler) Stop() {
	close(s.done)
}

func (s *Scheduler) runJob(ctx context.Context, sj scheduledJob) {
	ticker := time.NewTicker(sj.interval)
	defer ticker.Stop()

	// Run immediately on start
	s.executeJob(ctx, sj.job)

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.done:
			return
		case <-ticker.C:
			s.executeJob(ctx, sj.job)
		}
	}
}

func (s *Scheduler) executeJob(ctx context.Context, job Job) {
	s.logger.Info("running job", "name", job.Name)
	start := time.Now()

	if err := job.Fn(ctx); err != nil {
		s.logger.Error("job failed", "name", job.Name, "error", err, "duration", time.Since(start))
		return
	}

	s.logger.Info("job completed", "name", job.Name, "duration", time.Since(start))
}
