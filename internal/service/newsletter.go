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
	"github.com/perbu/activity/internal/newsletter"
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

// SendResult contains the result of sending newsletters
type SendResult struct {
	Sent             int
	Skipped          int
	Errors           int
	TotalSubscribers int
}

// Send sends newsletters to all subscribers
func (s *NewsletterService) Send(ctx context.Context, since time.Duration, dryRun bool, output io.Writer) (*SendResult, error) {
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

	sinceTime := time.Now().Add(-since)
	slog.Info("Sending newsletters", "since", sinceTime.Format("2006-01-02 15:04"), "dry_run", dryRun)

	// Send to all subscribers
	result, err := sender.SendAll(ctx, sinceTime)
	if err != nil {
		return nil, fmt.Errorf("failed to send newsletters: %w", err)
	}

	slog.Info("Newsletter send complete", "sent", result.Sent, "skipped", result.Skipped, "errors", result.Errors)

	return &SendResult{
		Sent:             result.Sent,
		Skipped:          result.Skipped,
		Errors:           result.Errors,
		TotalSubscribers: result.TotalSubscribers,
	}, nil
}

// ParseSinceDuration parses a duration string like "7d", "1w", "24h"
func ParseSinceDuration(s string) (time.Duration, error) {
	if len(s) == 0 {
		return 7 * 24 * time.Hour, nil // Default to 7 days
	}

	lastChar := s[len(s)-1]
	numPart := s[:len(s)-1]

	var multiplier time.Duration
	switch lastChar {
	case 'd':
		multiplier = 24 * time.Hour
	case 'w':
		multiplier = 7 * 24 * time.Hour
	case 'h':
		multiplier = time.Hour
	case 'm':
		multiplier = time.Minute
	default:
		// Try standard Go duration parsing
		return time.ParseDuration(s)
	}

	var num int
	if _, err := fmt.Sscanf(numPart, "%d", &num); err != nil {
		return 0, fmt.Errorf("invalid number: %s", numPart)
	}

	return time.Duration(num) * multiplier, nil
}
