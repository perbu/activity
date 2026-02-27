package scheduler

import (
	"context"
	"io"
	"log/slog"
	"time"

	"github.com/perbu/activity/internal/config"
	"github.com/perbu/activity/internal/service"
)

// Scheduler runs periodic background jobs.
type Scheduler struct {
	services *service.Services
	cfg      *config.Config
}

// New creates a new Scheduler.
func New(services *service.Services, cfg *config.Config) *Scheduler {
	return &Scheduler{
		services: services,
		cfg:      cfg,
	}
}

// Run blocks until ctx is cancelled, sending newsletters every Monday at 02:42.
// If auto_send is disabled in config, it returns immediately.
func (s *Scheduler) Run(ctx context.Context) {
	if !s.cfg.Newsletter.AutoSend {
		slog.Info("Newsletter auto-send is disabled")
		return
	}

	for {
		next := nextFireTime(time.Now())
		slog.Info("Newsletter auto-send scheduled", "next_fire", next.Format(time.RFC3339))

		timer := time.NewTimer(time.Until(next))
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}

		slog.Info("Newsletter auto-send firing")
		result, err := s.services.Newsletter.SendLastWeek(ctx, false, io.Discard)
		if err != nil {
			slog.Error("Newsletter auto-send failed", "error", err)
		} else {
			slog.Info("Newsletter auto-send complete",
				"sent", result.Sent,
				"skipped", result.Skipped,
				"errors", result.Errors,
				"total_subscribers", result.TotalSubscribers,
			)
		}
	}
}

// nextFireTime returns the next Monday at 02:42 strictly after now, in now's location.
func nextFireTime(now time.Time) time.Time {
	// Start from today at 02:42
	candidate := time.Date(now.Year(), now.Month(), now.Day(), 2, 42, 0, 0, now.Location())

	// Advance to Monday
	daysUntilMonday := (time.Monday - candidate.Weekday() + 7) % 7
	candidate = candidate.AddDate(0, 0, int(daysUntilMonday))

	// If candidate is not strictly after now, advance by a week
	if !candidate.After(now) {
		candidate = candidate.AddDate(0, 0, 7)
	}

	return candidate
}
