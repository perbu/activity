package email

import (
	"context"
	"fmt"

	"github.com/sendgrid/sendgrid-go"
	"github.com/sendgrid/sendgrid-go/helpers/mail"
)

// Email represents an email to be sent
type Email struct {
	To          string
	Subject     string
	HTMLContent string
	TextContent string
}

// Client wraps the SendGrid API client
type Client struct {
	apiKey    string
	fromEmail string
	fromName  string
}

// NewClient creates a new SendGrid client
func NewClient(apiKey, fromEmail, fromName string) *Client {
	return &Client{
		apiKey:    apiKey,
		fromEmail: fromEmail,
		fromName:  fromName,
	}
}

// Send sends an email via SendGrid and returns the message ID
func (c *Client) Send(ctx context.Context, email Email) (string, error) {
	from := mail.NewEmail(c.fromName, c.fromEmail)
	to := mail.NewEmail("", email.To)
	message := mail.NewSingleEmail(from, email.Subject, to, email.TextContent, email.HTMLContent)

	client := sendgrid.NewSendClient(c.apiKey)
	response, err := client.SendWithContext(ctx, message)
	if err != nil {
		return "", fmt.Errorf("failed to send email: %w", err)
	}

	// SendGrid returns 2xx status codes for success
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", fmt.Errorf("sendgrid returned status %d: %s", response.StatusCode, response.Body)
	}

	// Extract message ID from response headers
	messageID := ""
	if ids, ok := response.Headers["X-Message-Id"]; ok && len(ids) > 0 {
		messageID = ids[0]
	}

	return messageID, nil
}

// DryRunClient is a client that doesn't actually send emails
type DryRunClient struct {
	fromEmail string
	fromName  string
}

// NewDryRunClient creates a client that logs instead of sending
func NewDryRunClient(fromEmail, fromName string) *DryRunClient {
	return &DryRunClient{
		fromEmail: fromEmail,
		fromName:  fromName,
	}
}

// Send pretends to send an email (for dry runs)
func (c *DryRunClient) Send(ctx context.Context, email Email) (string, error) {
	// In dry run mode, we just return a fake message ID
	return "dry-run-message-id", nil
}

// Sender is the interface for sending emails
type Sender interface {
	Send(ctx context.Context, email Email) (string, error)
}
