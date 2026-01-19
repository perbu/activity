package analyzer

import (
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/perbu/activity/internal/config"
	"github.com/perbu/activity/internal/db"
	"github.com/perbu/activity/internal/git"
)

func TestShortSHA(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"full sha", "abc123def456789012345678901234567890abcd", "abc123de"},
		{"exactly 8 chars", "abc123de", "abc123de"},
		{"short sha", "abc123", "abc123"},
		{"empty", "", ""},
		{"9 chars", "abc123def", "abc123de"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shortSHA(tt.input)
			if got != tt.want {
				t.Errorf("shortSHA(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestExtractAuthors(t *testing.T) {
	tests := []struct {
		name    string
		commits []git.Commit
		want    []string
	}{
		{
			name:    "empty commits",
			commits: []git.Commit{},
			want:    []string{},
		},
		{
			name: "single author",
			commits: []git.Commit{
				{Author: "John Doe"},
				{Author: "John Doe"},
			},
			want: []string{"John Doe"},
		},
		{
			name: "multiple authors",
			commits: []git.Commit{
				{Author: "John Doe"},
				{Author: "Jane Smith"},
				{Author: "John Doe"},
				{Author: "Bob Wilson"},
			},
			want: []string{"John Doe", "Jane Smith", "Bob Wilson"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractAuthors(tt.commits)
			if len(got) != len(tt.want) {
				t.Errorf("extractAuthors() returned %d authors, want %d", len(got), len(tt.want))
				return
			}

			// Check that all expected authors are present (order may vary)
			gotMap := make(map[string]bool)
			for _, a := range got {
				gotMap[a] = true
			}
			for _, expected := range tt.want {
				if !gotMap[expected] {
					t.Errorf("extractAuthors() missing author %q", expected)
				}
			}
		})
	}
}

func TestBuildAnalysisPrompt(t *testing.T) {
	cfg := config.DefaultConfig()

	repo := &db.Repository{
		Name:   "test-repo",
		Branch: "main",
	}

	commits := []git.Commit{
		{
			SHA:     "abc123def456",
			Author:  "John Doe",
			Date:    time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
			Message: "Add new feature",
		},
		{
			SHA:     "def789ghi012",
			Author:  "Jane Smith",
			Date:    time.Date(2024, 1, 14, 9, 0, 0, 0, time.UTC),
			Message: "Fix bug in parser",
		},
	}

	t.Run("basic prompt structure", func(t *testing.T) {
		prompt := buildAnalysisPrompt(repo, commits, nil, cfg, "")

		// Check that key elements are present
		if !strings.Contains(prompt, "test-repo") {
			t.Error("prompt should contain repository name")
		}
		if !strings.Contains(prompt, "main") {
			t.Error("prompt should contain branch name")
		}
		if !strings.Contains(prompt, "Total commits: 2") {
			t.Error("prompt should contain commit count")
		}
		if !strings.Contains(prompt, "abc123de") {
			t.Error("prompt should contain shortened SHA")
		}
		if !strings.Contains(prompt, "John Doe") {
			t.Error("prompt should contain author name")
		}
		if !strings.Contains(prompt, "Add new feature") {
			t.Error("prompt should contain commit message")
		}
	})

	t.Run("with repository description", func(t *testing.T) {
		repoWithDesc := &db.Repository{
			Name:        "test-repo",
			Branch:      "main",
			Description: sql.NullString{String: "A test repository for testing", Valid: true},
		}

		prompt := buildAnalysisPrompt(repoWithDesc, commits, nil, cfg, "")

		if !strings.Contains(prompt, "A test repository for testing") {
			t.Error("prompt should contain repository description")
		}
	})

	t.Run("with branch activity", func(t *testing.T) {
		branchActivity := []git.BranchActivity{
			{
				BranchName:   "feature-x",
				CommitCount:  5,
				Authors:      []string{"John Doe"},
				AuthorCounts: map[string]int{"John Doe": 5},
			},
		}

		prompt := buildAnalysisPrompt(repo, commits, branchActivity, cfg, "")

		if !strings.Contains(prompt, "Other Branch Activity") {
			t.Error("prompt should contain branch activity section")
		}
		if !strings.Contains(prompt, "feature-x") {
			t.Error("prompt should contain branch name")
		}
		if !strings.Contains(prompt, "5 commits") {
			t.Error("prompt should contain commit count")
		}
	})

	t.Run("with previous summary", func(t *testing.T) {
		previousSummary := "Last week the team focused on bug fixes and code refactoring."

		prompt := buildAnalysisPrompt(repo, commits, nil, cfg, previousSummary)

		if !strings.Contains(prompt, "Previous Week's Summary") {
			t.Error("prompt should contain previous summary section header")
		}
		if !strings.Contains(prompt, previousSummary) {
			t.Error("prompt should contain previous summary content")
		}
	})

	t.Run("message truncation", func(t *testing.T) {
		longMessage := strings.Repeat("x", 1500) // Longer than default max
		commitsWithLongMsg := []git.Commit{
			{
				SHA:     "abc123def456789012", // Must be at least 8 chars
				Author:  "John",
				Date:    time.Now(),
				Message: longMessage,
			},
		}

		prompt := buildAnalysisPrompt(repo, commitsWithLongMsg, nil, cfg, "")

		if !strings.Contains(prompt, "[truncated]") {
			t.Error("long message should be truncated")
		}
	})

	t.Run("respects max commits config", func(t *testing.T) {
		// Create more commits than the default max
		manyCommits := make([]git.Commit, 60)
		for i := range manyCommits {
			manyCommits[i] = git.Commit{
				SHA:     "abc123def456789012", // Must be at least 8 chars
				Author:  "John",
				Date:    time.Now(),
				Message: "Commit message",
			}
		}

		prompt := buildAnalysisPrompt(repo, manyCommits, nil, cfg, "")

		// Should mention remaining commits
		if !strings.Contains(prompt, "... and 10 more commits") {
			t.Error("prompt should indicate remaining commits when exceeding max")
		}
	})
}

func TestNewAnalyzer(t *testing.T) {
	cfg := config.DefaultConfig()

	analyzer := New(nil, nil, cfg)

	if analyzer == nil {
		t.Error("New() returned nil")
	}
	if analyzer.config != cfg {
		t.Error("New() did not set config correctly")
	}
}
