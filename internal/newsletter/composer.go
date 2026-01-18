package newsletter

import (
	"fmt"

	"github.com/perbu/activity/internal/db"
	"github.com/perbu/activity/internal/email"
)

// Composer builds newsletter content from activity runs
type Composer struct {
	db            *db.DB
	subjectPrefix string
}

// NewComposer creates a new newsletter composer
func NewComposer(database *db.DB, subjectPrefix string) *Composer {
	return &Composer{
		db:            database,
		subjectPrefix: subjectPrefix,
	}
}

// ComposeForSubscriber builds a newsletter email for a subscriber based on unsent activity runs
func (c *Composer) ComposeForSubscriber(subscriber *db.Subscriber, runs []*db.ActivityRun) (*email.Email, error) {
	if len(runs) == 0 {
		return nil, nil
	}

	// Build sections for each run
	sections := make([]RepoSection, 0, len(runs))
	for _, run := range runs {
		// Get repo info
		repo, err := c.db.GetRepository(run.RepoID)
		if err != nil {
			// Skip runs for deleted repos
			continue
		}

		summary := ""
		if run.Summary.Valid {
			summary = run.Summary.String
		}

		// Convert markdown summary to HTML
		summaryHTML, err := MarkdownToHTML(summary)
		if err != nil {
			// Fall back to plain text if conversion fails
			summaryHTML = ""
		}

		// Format commit range
		commitRange := fmt.Sprintf("%s...%s", shortSHA(run.StartSHA), shortSHA(run.EndSHA))

		// Format analysis time
		analyzedAt := ""
		if run.CompletedAt.Valid {
			analyzedAt = run.CompletedAt.Time.Format("2006-01-02 15:04")
		}

		sections = append(sections, RepoSection{
			RepoName:    repo.Name,
			Summary:     summary,
			SummaryHTML: summaryHTML,
			CommitRange: commitRange,
			AnalyzedAt:  analyzedAt,
		})
	}

	if len(sections) == 0 {
		return nil, nil
	}

	// Build newsletter data
	data := &NewsletterData{
		Sections:      sections,
		TotalRepos:    len(sections),
		SubjectPrefix: c.subjectPrefix,
	}

	// Render HTML and text versions
	htmlContent, err := RenderHTML(data)
	if err != nil {
		return nil, fmt.Errorf("failed to render HTML: %w", err)
	}

	textContent, err := RenderText(data)
	if err != nil {
		return nil, fmt.Errorf("failed to render text: %w", err)
	}

	return &email.Email{
		To:          subscriber.Email,
		Subject:     data.Subject(),
		HTMLContent: htmlContent,
		TextContent: textContent,
	}, nil
}

func shortSHA(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}
