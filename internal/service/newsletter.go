package service

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/perbu/activity/internal/config"
	"github.com/perbu/activity/internal/db"
	"github.com/perbu/activity/internal/email"
	"github.com/perbu/activity/internal/git"
	"github.com/perbu/activity/internal/newsletter"
	"github.com/perbu/activity/internal/progress"
)

// NewsletterService handles newsletter subscriber management and sending
type NewsletterService struct {
	db  *db.DB
	cfg *config.Config
}

// NewNewsletterService creates a new NewsletterService
func NewNewsletterService(database *db.DB, cfg *config.Config) *NewsletterService {
	return &NewsletterService{
		db:  database,
		cfg: cfg,
	}
}

// AddSubscriber creates a new subscriber
func (s *NewsletterService) AddSubscriber(email string, subscribeAll bool) (*db.Subscriber, error) {
	// Check if subscriber already exists
	_, err := s.db.GetSubscriberByEmail(email)
	if err == nil {
		return nil, fmt.Errorf("subscriber '%s' already exists", email)
	}

	sub, err := s.db.CreateSubscriber(email, subscribeAll)
	if err != nil {
		return nil, fmt.Errorf("failed to create subscriber: %w", err)
	}

	slog.Info("Subscriber added", "email", email, "subscribe_all", subscribeAll)
	return sub, nil
}

// RemoveSubscriber deletes a subscriber by email
func (s *NewsletterService) RemoveSubscriber(email string) error {
	sub, err := s.db.GetSubscriberByEmail(email)
	if err != nil {
		return fmt.Errorf("subscriber not found: %s", email)
	}

	if err := s.db.DeleteSubscriber(sub.ID); err != nil {
		return fmt.Errorf("failed to delete subscriber: %w", err)
	}

	slog.Info("Subscriber removed", "email", email)
	return nil
}

// ListSubscribers returns all subscribers
func (s *NewsletterService) ListSubscribers() ([]*db.Subscriber, error) {
	return s.db.ListSubscribers()
}

// GetSubscriber returns a subscriber by email
func (s *NewsletterService) GetSubscriber(email string) (*db.Subscriber, error) {
	return s.db.GetSubscriberByEmail(email)
}

// Subscribe adds a subscription for a subscriber to a repository
func (s *NewsletterService) Subscribe(email, repoName string) error {
	sub, err := s.db.GetSubscriberByEmail(email)
	if err != nil {
		return fmt.Errorf("subscriber not found: %s", email)
	}

	if sub.SubscribeAll {
		return fmt.Errorf("subscriber '%s' is already subscribed to all repositories", email)
	}

	repo, err := s.db.GetRepositoryByName(repoName)
	if err != nil {
		return fmt.Errorf("repository not found: %s", repoName)
	}

	// Check if already subscribed
	_, err = s.db.GetSubscriptionBySubscriberAndRepo(sub.ID, repo.ID)
	if err == nil {
		return fmt.Errorf("'%s' is already subscribed to '%s'", email, repoName)
	}

	_, err = s.db.CreateSubscription(sub.ID, repo.ID)
	if err != nil {
		return fmt.Errorf("failed to create subscription: %w", err)
	}

	slog.Info("Subscribed to repository", "email", email, "repo", repoName)
	return nil
}

// Unsubscribe removes a subscription
func (s *NewsletterService) Unsubscribe(email, repoName string) error {
	sub, err := s.db.GetSubscriberByEmail(email)
	if err != nil {
		return fmt.Errorf("subscriber not found: %s", email)
	}

	repo, err := s.db.GetRepositoryByName(repoName)
	if err != nil {
		return fmt.Errorf("repository not found: %s", repoName)
	}

	if err := s.db.DeleteSubscriptionBySubscriberAndRepo(sub.ID, repo.ID); err != nil {
		return fmt.Errorf("failed to delete subscription: %w", err)
	}

	slog.Info("Unsubscribed from repository", "email", email, "repo", repoName)
	return nil
}

// GetSubscriptions returns subscriptions for a subscriber
func (s *NewsletterService) GetSubscriptions(subscriberID int64) ([]*db.Subscription, error) {
	return s.db.ListSubscriptionsBySubscriber(subscriberID)
}

// GetOrCreateSubscriber looks up a subscriber by email; creates one with subscribe_all=false if not found
func (s *NewsletterService) GetOrCreateSubscriber(email string) (*db.Subscriber, error) {
	sub, err := s.db.GetSubscriberByEmail(email)
	if err == nil {
		return sub, nil
	}
	sub, err = s.db.CreateSubscriber(email, false)
	if err != nil {
		return nil, fmt.Errorf("failed to create subscriber: %w", err)
	}
	slog.Info("Auto-created subscriber", "email", email)
	return sub, nil
}

// SetSubscribeAll updates the subscribe_all flag for a subscriber
func (s *NewsletterService) SetSubscribeAll(email string, subscribeAll bool) error {
	sub, err := s.db.GetSubscriberByEmail(email)
	if err != nil {
		return fmt.Errorf("subscriber not found: %s", email)
	}
	sub.SubscribeAll = subscribeAll
	if err := s.db.UpdateSubscriber(sub); err != nil {
		return fmt.Errorf("failed to update subscriber: %w", err)
	}
	slog.Info("Updated subscribe_all", "email", email, "subscribe_all", subscribeAll)
	return nil
}

// SendResult contains the result of sending newsletters
type SendResult struct {
	Sent             int
	Skipped          int
	Errors           int
	TotalSubscribers int
}

// Send sends newsletters to all subscribers for activity since the given time
func (s *NewsletterService) Send(ctx context.Context, sinceTime time.Time, dryRun bool, output io.Writer) (*SendResult, error) {
	// Check if newsletter is enabled
	if !s.cfg.Newsletter.Enabled && !dryRun {
		return nil, fmt.Errorf("newsletter is not enabled in config (set newsletter.enabled: true)")
	}

	// Get or validate API key
	apiKey := s.cfg.GetSendGridAPIKey()
	if apiKey == "" && !dryRun {
		return nil, fmt.Errorf("SendGrid API key not configured")
	}

	// Create email client
	var client email.Sender
	if dryRun {
		client = email.NewDryRunClient(s.cfg.Newsletter.FromEmail, s.cfg.Newsletter.FromName)
	} else {
		client = email.NewClient(apiKey, s.cfg.Newsletter.FromEmail, s.cfg.Newsletter.FromName)
	}

	// Create composer and sender
	composer := newsletter.NewComposer(s.db, s.cfg.Newsletter.SubjectPrefix)
	sender := newsletter.NewSender(s.db, composer, client, dryRun, output)

	progress.Log(ctx, "Sending newsletters", "since", sinceTime.Format("2006-01-02"), "dry_run", dryRun)

	// Send to all subscribers
	result, err := sender.SendAll(ctx, sinceTime)
	if err != nil {
		return nil, fmt.Errorf("failed to send newsletters: %w", err)
	}

	progress.Log(ctx, "Newsletter send complete", "sent", result.Sent, "skipped", result.Skipped, "errors", result.Errors)

	return &SendResult{
		Sent:             result.Sent,
		Skipped:          result.Skipped,
		Errors:           result.Errors,
		TotalSubscribers: result.TotalSubscribers,
	}, nil
}

// SendLastWeek sends newsletters covering the previous complete ISO week
func (s *NewsletterService) SendLastWeek(ctx context.Context, dryRun bool, output io.Writer) (*SendResult, error) {
	now := time.Now()
	year, week := now.ISOWeek()
	week--
	if week < 1 {
		year--
		lastDayOfPrevYear := time.Date(year, 12, 31, 0, 0, 0, 0, time.UTC)
		_, week = lastDayOfPrevYear.ISOWeek()
	}
	start, _ := git.ISOWeekBounds(year, week)
	return s.Send(ctx, start, dryRun, output)
}

