package newsletter

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/perbu/activity/internal/db"
	"github.com/perbu/activity/internal/email"
)

// SendResult contains the result of a newsletter send operation
type SendResult struct {
	TotalSubscribers int
	Sent             int
	Skipped          int
	Errors           int
}

// Sender orchestrates the newsletter sending process
type Sender struct {
	db       *db.DB
	composer *Composer
	client   email.Sender
	dryRun   bool
	output   io.Writer
}

// NewSender creates a new newsletter sender
func NewSender(database *db.DB, composer *Composer, client email.Sender, dryRun bool, output io.Writer) *Sender {
	return &Sender{
		db:       database,
		composer: composer,
		client:   client,
		dryRun:   dryRun,
		output:   output,
	}
}

// SendAll sends newsletters to all subscribers with unsent activity runs
func (s *Sender) SendAll(ctx context.Context, since time.Time) (*SendResult, error) {
	result := &SendResult{}

	// Get all subscribers
	subscribers, err := s.db.ListSubscribers()
	if err != nil {
		return nil, fmt.Errorf("failed to list subscribers: %w", err)
	}

	result.TotalSubscribers = len(subscribers)

	for _, subscriber := range subscribers {
		// Get unsent activity runs for this subscriber
		runs, err := s.db.GetUnsentActivityRuns(subscriber.ID, since)
		if err != nil {
			fmt.Fprintf(s.output, "Error getting unsent runs for %s: %v\n", subscriber.Email, err)
			result.Errors++
			continue
		}

		if len(runs) == 0 {
			result.Skipped++
			continue
		}

		// Compose the newsletter
		email, err := s.composer.ComposeForSubscriber(subscriber, runs)
		if err != nil {
			fmt.Fprintf(s.output, "Error composing newsletter for %s: %v\n", subscriber.Email, err)
			result.Errors++
			continue
		}

		if email == nil {
			result.Skipped++
			continue
		}

		// Send or simulate sending
		if s.dryRun {
			fmt.Fprintf(s.output, "[DRY RUN] Would send to %s: %s (%d activity updates)\n",
				subscriber.Email, email.Subject, len(runs))
		} else {
			messageID, err := s.client.Send(ctx, *email)
			if err != nil {
				fmt.Fprintf(s.output, "Error sending to %s: %v\n", subscriber.Email, err)
				result.Errors++
				continue
			}

			// Record sends for deduplication
			for _, run := range runs {
				_, err := s.db.CreateNewsletterSend(subscriber.ID, run.ID, messageID)
				if err != nil {
					fmt.Fprintf(s.output, "Warning: failed to record send for run %d: %v\n", run.ID, err)
				}
			}

			fmt.Fprintf(s.output, "Sent to %s: %s (%d activity updates)\n",
				subscriber.Email, email.Subject, len(runs))
		}

		result.Sent++
	}

	return result, nil
}

// SendToSubscriber sends a newsletter to a specific subscriber
func (s *Sender) SendToSubscriber(ctx context.Context, email string, since time.Time) error {
	subscriber, err := s.db.GetSubscriberByEmail(email)
	if err != nil {
		return fmt.Errorf("subscriber not found: %s", email)
	}

	runs, err := s.db.GetUnsentActivityRuns(subscriber.ID, since)
	if err != nil {
		return fmt.Errorf("failed to get unsent runs: %w", err)
	}

	if len(runs) == 0 {
		fmt.Fprintf(s.output, "No unsent activity runs for %s\n", email)
		return nil
	}

	composed, err := s.composer.ComposeForSubscriber(subscriber, runs)
	if err != nil {
		return fmt.Errorf("failed to compose newsletter: %w", err)
	}

	if composed == nil {
		fmt.Fprintf(s.output, "No content to send for %s\n", email)
		return nil
	}

	if s.dryRun {
		fmt.Fprintf(s.output, "[DRY RUN] Would send to %s: %s (%d activity updates)\n",
			email, composed.Subject, len(runs))
		return nil
	}

	messageID, err := s.client.Send(ctx, *composed)
	if err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	// Record sends
	for _, run := range runs {
		_, err := s.db.CreateNewsletterSend(subscriber.ID, run.ID, messageID)
		if err != nil {
			fmt.Fprintf(s.output, "Warning: failed to record send for run %d: %v\n", run.ID, err)
		}
	}

	fmt.Fprintf(s.output, "Sent to %s: %s (%d activity updates)\n",
		email, composed.Subject, len(runs))

	return nil
}
