package scheduler

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

	"github.com/perbu/activity/internal/config"
	"github.com/perbu/activity/internal/db"
	"github.com/perbu/activity/internal/email"
	"github.com/perbu/activity/internal/service"
)

// Scheduler runs the weekly automation pipeline.
type Scheduler struct {
	services *service.Services
	cfg      *config.Config
	db       *db.DB
}

// New creates a new Scheduler.
func New(services *service.Services, cfg *config.Config, database *db.DB) *Scheduler {
	return &Scheduler{
		services: services,
		cfg:      cfg,
		db:       database,
	}
}

// Run blocks until ctx is cancelled, running the weekly pipeline on schedule.
func (s *Scheduler) Run(ctx context.Context) {
	if !s.cfg.ScheduleEnabled() {
		slog.Info("Scheduled pipeline is disabled")
		return
	}

	// Log what's enabled
	if s.cfg.Schedule.Enabled {
		slog.Info("Scheduled pipeline enabled (full: update, generate, newsletter)")
	} else if s.cfg.Newsletter.AutoSend {
		slog.Info("Scheduled pipeline enabled (newsletter auto_send backward compat)")
	}

	for {
		next := s.nextFireTime(time.Now())
		slog.Info("Pipeline scheduled", "next_fire", next.Format(time.RFC3339))

		timer := time.NewTimer(time.Until(next))
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}

		s.runPipeline(ctx)
	}
}

// runPipeline executes the full weekly pipeline and notifies admins on errors.
func (s *Scheduler) runPipeline(ctx context.Context) {
	slog.Info("Pipeline starting")
	start := time.Now()
	var errs []pipelineError

	// Step 1: Update all repositories
	if s.cfg.Schedule.Enabled {
		slog.Info("Pipeline step: updating repositories")
		results, err := s.services.Repo.UpdateAll(ctx)
		if err != nil {
			slog.Error("Pipeline: repo update failed", "error", err)
			errs = append(errs, pipelineError{Step: "Update Repositories", Err: err.Error()})
		} else {
			slog.Info("Pipeline: repos updated", "count", len(results))
		}
	}

	// Step 2: Generate reports for last week
	if s.cfg.Schedule.Enabled {
		slog.Info("Pipeline step: generating reports")
		results, err := s.services.Report.GenerateLastWeek(ctx, false)
		if err != nil {
			slog.Error("Pipeline: report generation failed", "error", err)
			errs = append(errs, pipelineError{Step: "Generate Reports", Err: err.Error()})
		} else {
			generated, skipped, failed, noCommits := 0, 0, 0, 0
			for _, r := range results {
				generated += r.Generated
				skipped += r.Skipped
				noCommits += r.NoCommits
				if r.Generated == 0 && r.Skipped == 0 && r.NoCommits == 0 {
					failed++
				}
			}
			slog.Info("Pipeline: reports generated",
				"generated", generated, "skipped", skipped, "no_commits", noCommits, "failed", failed)
			if failed > 0 {
				errs = append(errs, pipelineError{
					Step: "Generate Reports",
					Err:  fmt.Sprintf("%d repositories failed to generate reports", failed),
				})
			}
		}
	}

	// Step 3: Send newsletter
	if s.cfg.Newsletter.Enabled {
		slog.Info("Pipeline step: sending newsletter")
		result, err := s.services.Newsletter.SendLastWeek(ctx, false, io.Discard)
		if err != nil {
			slog.Error("Pipeline: newsletter send failed", "error", err)
			errs = append(errs, pipelineError{Step: "Send Newsletter", Err: err.Error()})
		} else {
			slog.Info("Pipeline: newsletter sent",
				"sent", result.Sent, "skipped", result.Skipped, "errors", result.Errors)
			if result.Errors > 0 {
				errs = append(errs, pipelineError{
					Step: "Send Newsletter",
					Err:  fmt.Sprintf("%d newsletter sends failed", result.Errors),
				})
			}
		}
	}

	duration := time.Since(start)
	if len(errs) > 0 {
		slog.Error("Pipeline completed with errors", "errors", len(errs), "duration", duration)
		if s.cfg.Schedule.NotifyAdmins {
			s.notifyAdmins(ctx, errs, duration)
		}
	} else {
		slog.Info("Pipeline completed successfully", "duration", duration)
	}
}

// pipelineError records a failure in a pipeline step.
type pipelineError struct {
	Step string
	Err  string
}

// notifyAdmins sends an error report email to all admins.
func (s *Scheduler) notifyAdmins(ctx context.Context, errs []pipelineError, duration time.Duration) {
	apiKey := s.cfg.GetSendGridAPIKey()
	if apiKey == "" {
		slog.Warn("Cannot notify admins: no SendGrid API key configured")
		return
	}

	admins, err := s.db.ListAdmins()
	if err != nil {
		slog.Error("Cannot notify admins: failed to list admins", "error", err)
		return
	}
	if len(admins) == 0 {
		slog.Warn("Cannot notify admins: no admins configured")
		return
	}

	subject := fmt.Sprintf("%s Pipeline errors - %s",
		s.cfg.Newsletter.SubjectPrefix, time.Now().Format("2006-01-02"))

	body := buildErrorEmail(errs, duration)

	client := email.NewClient(apiKey, s.cfg.Newsletter.FromEmail, s.cfg.Newsletter.FromName)
	for _, admin := range admins {
		msg := email.Email{
			To:          admin.Email,
			Subject:     subject,
			HTMLContent: body,
			TextContent: buildErrorEmailText(errs, duration),
		}
		if _, err := client.Send(ctx, msg); err != nil {
			slog.Error("Failed to notify admin", "email", admin.Email, "error", err)
		} else {
			slog.Info("Admin notified of pipeline errors", "email", admin.Email)
		}
	}
}

func buildErrorEmail(errs []pipelineError, duration time.Duration) string {
	var b strings.Builder
	b.WriteString("<h2>Activity Pipeline Errors</h2>")
	b.WriteString(fmt.Sprintf("<p>The scheduled pipeline completed in %s with %d error(s):</p>", duration.Round(time.Second), len(errs)))
	b.WriteString("<table border='1' cellpadding='8' cellspacing='0' style='border-collapse:collapse'>")
	b.WriteString("<tr><th>Step</th><th>Error</th></tr>")
	for _, e := range errs {
		b.WriteString(fmt.Sprintf("<tr><td>%s</td><td>%s</td></tr>", e.Step, e.Err))
	}
	b.WriteString("</table>")
	b.WriteString("<p>Check server logs for details.</p>")
	return b.String()
}

func buildErrorEmailText(errs []pipelineError, duration time.Duration) string {
	var b strings.Builder
	b.WriteString("Activity Pipeline Errors\n\n")
	b.WriteString(fmt.Sprintf("Pipeline completed in %s with %d error(s):\n\n", duration.Round(time.Second), len(errs)))
	for _, e := range errs {
		b.WriteString(fmt.Sprintf("- [%s] %s\n", e.Step, e.Err))
	}
	b.WriteString("\nCheck server logs for details.\n")
	return b.String()
}

// nextFireTime returns the next scheduled time strictly after now.
func (s *Scheduler) nextFireTime(now time.Time) time.Time {
	day := s.cfg.GetScheduleDay()
	hour := s.cfg.Schedule.Hour
	minute := s.cfg.Schedule.Minute

	// Start from today at the configured time
	candidate := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, now.Location())

	// Advance to the target day of week
	daysUntil := (int(day) - int(candidate.Weekday()) + 7) % 7
	candidate = candidate.AddDate(0, 0, daysUntil)

	// If candidate is not strictly after now, advance by a week
	if !candidate.After(now) {
		candidate = candidate.AddDate(0, 0, 7)
	}

	return candidate
}
