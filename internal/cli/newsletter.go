package cli

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/perbu/activity/internal/email"
	"github.com/perbu/activity/internal/newsletter"
)

// Newsletter handles the 'newsletter' subcommand
func Newsletter(ctx *Context, args []string) error {
	if len(args) == 0 {
		printNewsletterUsage()
		return nil
	}

	subcommand := args[0]
	subArgs := args[1:]

	switch subcommand {
	case "subscriber":
		return newsletterSubscriber(ctx, subArgs)
	case "subscribe":
		return newsletterSubscribe(ctx, subArgs)
	case "unsubscribe":
		return newsletterUnsubscribe(ctx, subArgs)
	case "send":
		return newsletterSend(ctx, subArgs)
	default:
		fmt.Fprintf(os.Stderr, "Unknown newsletter subcommand: %s\n\n", subcommand)
		printNewsletterUsage()
		return fmt.Errorf("unknown newsletter subcommand: %s", subcommand)
	}
}

func newsletterSubscriber(ctx *Context, args []string) error {
	if len(args) == 0 {
		printSubscriberUsage()
		return nil
	}

	subcommand := args[0]
	subArgs := args[1:]

	switch subcommand {
	case "add":
		return subscriberAdd(ctx, subArgs)
	case "remove":
		return subscriberRemove(ctx, subArgs)
	case "list":
		return subscriberList(ctx, subArgs)
	default:
		fmt.Fprintf(os.Stderr, "Unknown subscriber subcommand: %s\n\n", subcommand)
		printSubscriberUsage()
		return fmt.Errorf("unknown subscriber subcommand: %s", subcommand)
	}
}

func subscriberAdd(ctx *Context, args []string) error {
	flags := flag.NewFlagSet("subscriber add", flag.ExitOnError)
	all := flags.Bool("all", false, "Subscribe to all repositories")

	if err := flags.Parse(args); err != nil {
		return err
	}

	if flags.NArg() != 1 {
		fmt.Fprintf(os.Stderr, "Usage: activity newsletter subscriber add [--all] <email>\n")
		return fmt.Errorf("requires exactly 1 argument: email")
	}

	emailAddr := flags.Arg(0)

	// Check if subscriber already exists
	_, err := ctx.DB.GetSubscriberByEmail(emailAddr)
	if err == nil {
		return fmt.Errorf("subscriber '%s' already exists", emailAddr)
	}

	// Create subscriber
	sub, err := ctx.DB.CreateSubscriber(emailAddr, *all)
	if err != nil {
		return fmt.Errorf("failed to create subscriber: %w", err)
	}

	if !ctx.Quiet {
		if *all {
			fmt.Printf("Subscriber '%s' added (subscribed to all repositories)\n", emailAddr)
		} else {
			fmt.Printf("Subscriber '%s' added\n", emailAddr)
		}
		if ctx.Verbose {
			fmt.Printf("  ID: %d\n", sub.ID)
			fmt.Printf("  Created: %s\n", sub.CreatedAt.Format("2006-01-02 15:04:05"))
		}
	}

	return nil
}

func subscriberRemove(ctx *Context, args []string) error {
	if len(args) != 1 {
		fmt.Fprintf(os.Stderr, "Usage: activity newsletter subscriber remove <email>\n")
		return fmt.Errorf("requires exactly 1 argument: email")
	}

	emailAddr := args[0]

	sub, err := ctx.DB.GetSubscriberByEmail(emailAddr)
	if err != nil {
		return fmt.Errorf("subscriber not found: %s", emailAddr)
	}

	if err := ctx.DB.DeleteSubscriber(sub.ID); err != nil {
		return fmt.Errorf("failed to delete subscriber: %w", err)
	}

	if !ctx.Quiet {
		fmt.Printf("Subscriber '%s' removed\n", emailAddr)
	}

	return nil
}

func subscriberList(ctx *Context, _ []string) error {
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

func newsletterSubscribe(ctx *Context, args []string) error {
	if len(args) != 2 {
		fmt.Fprintf(os.Stderr, "Usage: activity newsletter subscribe <email> <repo>\n")
		return fmt.Errorf("requires exactly 2 arguments: email and repo")
	}

	emailAddr := args[0]
	repoName := args[1]

	// Get subscriber
	sub, err := ctx.DB.GetSubscriberByEmail(emailAddr)
	if err != nil {
		return fmt.Errorf("subscriber not found: %s", emailAddr)
	}

	if sub.SubscribeAll {
		return fmt.Errorf("subscriber '%s' is already subscribed to all repositories", emailAddr)
	}

	// Get repository
	repo, err := ctx.DB.GetRepositoryByName(repoName)
	if err != nil {
		return fmt.Errorf("repository not found: %s", repoName)
	}

	// Check if already subscribed
	_, err = ctx.DB.GetSubscriptionBySubscriberAndRepo(sub.ID, repo.ID)
	if err == nil {
		return fmt.Errorf("'%s' is already subscribed to '%s'", emailAddr, repoName)
	}

	// Create subscription
	_, err = ctx.DB.CreateSubscription(sub.ID, repo.ID)
	if err != nil {
		return fmt.Errorf("failed to create subscription: %w", err)
	}

	if !ctx.Quiet {
		fmt.Printf("'%s' subscribed to '%s'\n", emailAddr, repoName)
	}

	return nil
}

func newsletterUnsubscribe(ctx *Context, args []string) error {
	if len(args) != 2 {
		fmt.Fprintf(os.Stderr, "Usage: activity newsletter unsubscribe <email> <repo>\n")
		return fmt.Errorf("requires exactly 2 arguments: email and repo")
	}

	emailAddr := args[0]
	repoName := args[1]

	// Get subscriber
	sub, err := ctx.DB.GetSubscriberByEmail(emailAddr)
	if err != nil {
		return fmt.Errorf("subscriber not found: %s", emailAddr)
	}

	// Get repository
	repo, err := ctx.DB.GetRepositoryByName(repoName)
	if err != nil {
		return fmt.Errorf("repository not found: %s", repoName)
	}

	// Delete subscription
	if err := ctx.DB.DeleteSubscriptionBySubscriberAndRepo(sub.ID, repo.ID); err != nil {
		return fmt.Errorf("failed to delete subscription: %w", err)
	}

	if !ctx.Quiet {
		fmt.Printf("'%s' unsubscribed from '%s'\n", emailAddr, repoName)
	}

	return nil
}

func newsletterSend(ctx *Context, args []string) error {
	flags := flag.NewFlagSet("newsletter send", flag.ExitOnError)
	dryRun := flags.Bool("dry-run", false, "Preview what would be sent without actually sending")
	sinceStr := flags.String("since", "7d", "Send activity since (e.g., 7d, 24h, 1w)")

	if err := flags.Parse(args); err != nil {
		return err
	}

	// Parse since duration
	since, err := parseSinceDuration(*sinceStr)
	if err != nil {
		return fmt.Errorf("invalid --since value: %w", err)
	}

	// Check if newsletter is enabled
	if !ctx.Config.Newsletter.Enabled && !*dryRun {
		return fmt.Errorf("newsletter is not enabled in config (set newsletter.enabled: true)")
	}

	// Get or validate API key
	apiKey := ctx.Config.GetSendGridAPIKey()
	if apiKey == "" && !*dryRun {
		return fmt.Errorf("SendGrid API key not configured")
	}

	// Create email client
	var client email.Sender
	if *dryRun {
		client = email.NewDryRunClient(ctx.Config.Newsletter.FromEmail, ctx.Config.Newsletter.FromName)
	} else {
		client = email.NewClient(apiKey, ctx.Config.Newsletter.FromEmail, ctx.Config.Newsletter.FromName)
	}

	// Create composer and sender
	composer := newsletter.NewComposer(ctx.DB, ctx.Config.Newsletter.SubjectPrefix)
	sender := newsletter.NewSender(ctx.DB, composer, client, *dryRun, os.Stdout)

	if !ctx.Quiet {
		sinceTime := time.Now().Add(-since)
		if *dryRun {
			fmt.Printf("Dry run: checking for activity since %s\n\n", sinceTime.Format("2006-01-02 15:04"))
		} else {
			fmt.Printf("Sending newsletters for activity since %s\n\n", sinceTime.Format("2006-01-02 15:04"))
		}
	}

	// Send to all subscribers
	result, err := sender.SendAll(context.Background(), time.Now().Add(-since))
	if err != nil {
		return fmt.Errorf("failed to send newsletters: %w", err)
	}

	if !ctx.Quiet {
		fmt.Printf("\nSummary: %d sent, %d skipped, %d errors (of %d subscribers)\n",
			result.Sent, result.Skipped, result.Errors, result.TotalSubscribers)
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

func printNewsletterUsage() {
	fmt.Println(`Newsletter management commands

Usage:
  activity newsletter <subcommand> [flags] [arguments]

Subcommands:
  subscriber add [--all] <email>
                      Add a subscriber (--all = subscribe to all repos)
  subscriber remove <email>
                      Remove a subscriber
  subscriber list     List all subscribers

  subscribe <email> <repo>
                      Subscribe an email to a specific repository
  unsubscribe <email> <repo>
                      Unsubscribe an email from a repository

  send [--dry-run] [--since=7d]
                      Send newsletters to all subscribers

Examples:
  activity newsletter subscriber add user@example.com --all
  activity newsletter subscriber add user@example.com
  activity newsletter subscribe user@example.com myproject
  activity newsletter send --dry-run --since=7d
  activity newsletter send --since=24h`)
}

func printSubscriberUsage() {
	fmt.Println(`Subscriber management commands

Usage:
  activity newsletter subscriber <subcommand> [flags] [arguments]

Subcommands:
  add [--all] <email>   Add a subscriber
  remove <email>        Remove a subscriber
  list                  List all subscribers`)
}
