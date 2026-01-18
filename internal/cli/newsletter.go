package cli

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/perbu/activity/internal/email"
	"github.com/perbu/activity/internal/newsletter"
)

// Run executes the subscriber add command
func (c *SubscriberAddCmd) Run(ctx *Context) error {
	// Check if subscriber already exists
	_, err := ctx.DB.GetSubscriberByEmail(c.Email)
	if err == nil {
		return fmt.Errorf("subscriber '%s' already exists", c.Email)
	}

	// Create subscriber
	sub, err := ctx.DB.CreateSubscriber(c.Email, c.All)
	if err != nil {
		return fmt.Errorf("failed to create subscriber: %w", err)
	}

	if !ctx.Quiet {
		slog.Info("Subscriber added", "email", c.Email, "subscribe_all", c.All, "id", sub.ID)
	}

	return nil
}

// Run executes the subscriber remove command
func (c *SubscriberRemoveCmd) Run(ctx *Context) error {
	sub, err := ctx.DB.GetSubscriberByEmail(c.Email)
	if err != nil {
		return fmt.Errorf("subscriber not found: %s", c.Email)
	}

	if err := ctx.DB.DeleteSubscriber(sub.ID); err != nil {
		return fmt.Errorf("failed to delete subscriber: %w", err)
	}

	if !ctx.Quiet {
		slog.Info("Subscriber removed", "email", c.Email)
	}

	return nil
}

// Run executes the subscriber list command
func (c *SubscriberListCmd) Run(ctx *Context) error {
	subscribers, err := ctx.DB.ListSubscribers()
	if err != nil {
		return fmt.Errorf("failed to list subscribers: %w", err)
	}

	if len(subscribers) == 0 {
		if !ctx.Quiet {
			fmt.Println("No subscribers found")
		}
		return nil
	}

	for _, sub := range subscribers {
		subscribeType := "specific repos"
		if sub.SubscribeAll {
			subscribeType = "all repos"
		}

		if ctx.Verbose {
			fmt.Printf("%s (ID: %d, %s, created: %s)\n",
				sub.Email, sub.ID, subscribeType, sub.CreatedAt.Format("2006-01-02"))
		} else {
			fmt.Printf("%s (%s)\n", sub.Email, subscribeType)
		}

		// Show subscriptions if not subscribe_all
		if !sub.SubscribeAll && ctx.Verbose {
			subs, err := ctx.DB.ListSubscriptionsBySubscriber(sub.ID)
			if err == nil && len(subs) > 0 {
				for _, subscription := range subs {
					repo, err := ctx.DB.GetRepository(subscription.RepoID)
					if err == nil {
						fmt.Printf("  - %s\n", repo.Name)
					}
				}
			}
		}
	}

	return nil
}

// Run executes the newsletter subscribe command
func (c *NewsletterSubscribeCmd) Run(ctx *Context) error {
	// Get subscriber
	sub, err := ctx.DB.GetSubscriberByEmail(c.Email)
	if err != nil {
		return fmt.Errorf("subscriber not found: %s", c.Email)
	}

	if sub.SubscribeAll {
		return fmt.Errorf("subscriber '%s' is already subscribed to all repositories", c.Email)
	}

	// Get repository
	repo, err := ctx.DB.GetRepositoryByName(c.Repo)
	if err != nil {
		return fmt.Errorf("repository not found: %s", c.Repo)
	}

	// Check if already subscribed
	_, err = ctx.DB.GetSubscriptionBySubscriberAndRepo(sub.ID, repo.ID)
	if err == nil {
		return fmt.Errorf("'%s' is already subscribed to '%s'", c.Email, c.Repo)
	}

	// Create subscription
	_, err = ctx.DB.CreateSubscription(sub.ID, repo.ID)
	if err != nil {
		return fmt.Errorf("failed to create subscription: %w", err)
	}

	if !ctx.Quiet {
		slog.Info("Subscribed to repository", "email", c.Email, "repo", c.Repo)
	}

	return nil
}

// Run executes the newsletter unsubscribe command
func (c *NewsletterUnsubscribeCmd) Run(ctx *Context) error {
	// Get subscriber
	sub, err := ctx.DB.GetSubscriberByEmail(c.Email)
	if err != nil {
		return fmt.Errorf("subscriber not found: %s", c.Email)
	}

	// Get repository
	repo, err := ctx.DB.GetRepositoryByName(c.Repo)
	if err != nil {
		return fmt.Errorf("repository not found: %s", c.Repo)
	}

	// Delete subscription
	if err := ctx.DB.DeleteSubscriptionBySubscriberAndRepo(sub.ID, repo.ID); err != nil {
		return fmt.Errorf("failed to delete subscription: %w", err)
	}

	if !ctx.Quiet {
		slog.Info("Unsubscribed from repository", "email", c.Email, "repo", c.Repo)
	}

	return nil
}

// Run executes the newsletter send command
func (c *NewsletterSendCmd) Run(ctx *Context) error {
	// Parse since duration
	since, err := parseSinceDuration(c.Since)
	if err != nil {
		return fmt.Errorf("invalid --since value: %w", err)
	}

	// Check if newsletter is enabled
	if !ctx.Config.Newsletter.Enabled && !c.DryRun {
		return fmt.Errorf("newsletter is not enabled in config (set newsletter.enabled: true)")
	}

	// Get or validate API key
	apiKey := ctx.Config.GetSendGridAPIKey()
	if apiKey == "" && !c.DryRun {
		return fmt.Errorf("SendGrid API key not configured")
	}

	// Create email client
	var client email.Sender
	if c.DryRun {
		client = email.NewDryRunClient(ctx.Config.Newsletter.FromEmail, ctx.Config.Newsletter.FromName)
	} else {
		client = email.NewClient(apiKey, ctx.Config.Newsletter.FromEmail, ctx.Config.Newsletter.FromName)
	}

	// Create composer and sender
	composer := newsletter.NewComposer(ctx.DB, ctx.Config.Newsletter.SubjectPrefix)
	sender := newsletter.NewSender(ctx.DB, composer, client, c.DryRun, os.Stdout)

	if !ctx.Quiet {
		sinceTime := time.Now().Add(-since)
		slog.Info("Sending newsletters", "since", sinceTime.Format("2006-01-02 15:04"), "dry_run", c.DryRun)
	}

	// Send to all subscribers
	result, err := sender.SendAll(context.Background(), time.Now().Add(-since))
	if err != nil {
		return fmt.Errorf("failed to send newsletters: %w", err)
	}

	if !ctx.Quiet {
		slog.Info("Newsletter send complete", "sent", result.Sent, "skipped", result.Skipped, "errors", result.Errors, "total_subscribers", result.TotalSubscribers)
	}

	return nil
}

func parseSinceDuration(s string) (time.Duration, error) {
	// Handle common formats like "7d", "1w", "24h"
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
