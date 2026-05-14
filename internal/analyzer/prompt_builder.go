package analyzer

import (
	"fmt"
	"strings"

	"github.com/perbu/activity/internal/config"
	"github.com/perbu/activity/internal/db"
	"github.com/perbu/activity/internal/git"
)

// promptMode describes which analysis mode is building the prompt.
type promptMode int

const (
	promptModeSimple promptMode = iota // Phase 2: simple LLM
	promptModeAgent                    // Phase 3: agent with tools
)

// promptBuilder builds prompt strings for LLM commit analysis.
type promptBuilder struct {
	mode             promptMode
	repo             *db.Repository
	commits          []git.Commit
	branchActivity   []git.BranchActivity
	previousSummary  string
	prData           string
	maxCommits       int
	maxMessageLength int
	phase2Prompt     string // only used in simple mode
}

// buildCommitPrompt assembles the full commit analysis prompt using the builder.
func (b *promptBuilder) buildCommitPrompt() string {
	var sb strings.Builder

	sb.WriteString("You are analyzing git commits for a software project.\n\n")
	sb.WriteString(fmt.Sprintf("Repository: %s\n", b.repo.Name))
	if b.repo.Description.Valid && b.repo.Description.String != "" {
		sb.WriteString(fmt.Sprintf("About: %s\n", b.repo.Description.String))
	}
	sb.WriteString(fmt.Sprintf("Branch: %s\n", b.repo.Branch))
	sb.WriteString(fmt.Sprintf("Total commits: %d\n\n", len(b.commits)))

	b.writeCommitList(&sb)
	b.writeBranchActivity(&sb)
	b.writePRData(&sb)
	b.writePreviousSummary(&sb)

	// Mode-specific trailing prompt
	if b.mode == promptModeSimple {
		sb.WriteString(b.phase2Prompt)
		sb.WriteString("\n")
		// For external repositories, add contributor analysis instruction
		if b.repo.External {
			sb.WriteString("\n5. Contributors: Brief section about active contributors and their areas of focus.\n")
		}
	} else {
		sb.WriteString("Please analyze these commits and provide a summary.\n")
	}

	return sb.String()
}

func (b *promptBuilder) writeCommitList(sb *strings.Builder) {
	maxCommits := b.maxCommits
	if maxCommits <= 0 {
		maxCommits = 50
	}

	limit := len(b.commits)
	if limit > maxCommits {
		limit = maxCommits
	}

	maxMsgLen := b.maxMessageLength
	if maxMsgLen <= 0 {
		maxMsgLen = 1000
	}

	sb.WriteString("Commits (newest first):\n\n")

	for i := 0; i < limit; i++ {
		commit := b.commits[i]
		sb.WriteString(fmt.Sprintf("Commit %d:\n", i+1))
		sb.WriteString(fmt.Sprintf("  SHA: %s\n", commit.SHA[:8]))
		sb.WriteString(fmt.Sprintf("  Author: %s\n", commit.Author))
		sb.WriteString(fmt.Sprintf("  Date: %s\n", commit.Date.Format("2006-01-02 15:04")))

		message := commit.Message
		truncated := false
		if len(message) > maxMsgLen {
			message = message[:maxMsgLen]
			truncated = true
		}
		sb.WriteString(fmt.Sprintf("  Message: %s", message))
		if truncated {
			if b.mode == promptModeAgent {
				sb.WriteString(" [truncated - use get_full_commit_message for complete text]")
			} else {
				sb.WriteString("... [truncated]")
			}
		}
		sb.WriteString("\n\n")
	}

	if len(b.commits) > maxCommits {
		sb.WriteString(fmt.Sprintf("... and %d more commits\n\n", len(b.commits)-maxCommits))
	}
}

func (b *promptBuilder) writeBranchActivity(sb *strings.Builder) {
	if len(b.branchActivity) == 0 {
		return
	}

	sb.WriteString("## Other Branch Activity\n")
	sb.WriteString("The following feature branches had commits this week that haven't been merged to the main branch:\n")
	for _, ba := range b.branchActivity {
		sb.WriteString(fmt.Sprintf("- %s: %d commits (", ba.BranchName, ba.CommitCount))
		first := true
		for author, count := range ba.AuthorCounts {
			if !first {
				sb.WriteString(", ")
			}
			sb.WriteString(fmt.Sprintf("%s: %d", author, count))
			first = false
		}
		sb.WriteString(")\n")
	}
	sb.WriteString("\nInclude a brief mention of this parallel work in your summary.\n\n")
}

func (b *promptBuilder) writePRData(sb *strings.Builder) {
	if b.prData == "" {
		return
	}

	if b.mode == promptModeAgent {
		sb.WriteString("## Pull Requests and Reviews\n")
		sb.WriteString("The following PR data was retrieved from the forge. Use this data directly in your analysis — do NOT call get_pr_reviews, as the data is already provided here. Only report on information that is present; do NOT comment on the absence of reviews or discussion.\n\n")
	} else {
		sb.WriteString("## Pull Requests\n")
		sb.WriteString("Only report on information that is present; do NOT comment on the absence of reviews or discussion.\n\n")
	}
	sb.WriteString(b.prData)
	sb.WriteString("\n")
}

func (b *promptBuilder) writePreviousSummary(sb *strings.Builder) {
	if b.previousSummary == "" {
		return
	}
	sb.WriteString("## Previous Week's Summary (for context)\n")
	sb.WriteString(b.previousSummary)
	sb.WriteString("\n\nUse this context to maintain narrative continuity and reference ongoing work where relevant.\n\n")
}

// buildAnalysisPrompt creates the prompt for Phase 2 (simple LLM) analysis.
func buildAnalysisPrompt(repo *db.Repository, commits []git.Commit, branchActivity []git.BranchActivity, cfg *config.Config, previousSummary string, prData string) string {
	b := promptBuilder{
		mode:             promptModeSimple,
		repo:             repo,
		commits:          commits,
		branchActivity:   branchActivity,
		previousSummary:  previousSummary,
		prData:           prData,
		maxCommits:       cfg.LLM.MaxCommits,
		maxMessageLength: cfg.LLM.MaxMessageLength,
		phase2Prompt:     cfg.GetPhase2Prompt(),
	}
	return b.buildCommitPrompt()
}

// buildAgentPrompt creates the user prompt for Phase 3 (agent-based) analysis.
func buildAgentPrompt(repo *db.Repository, commits []git.Commit, branchActivity []git.BranchActivity, maxMessageLength int, previousSummary string, prData string) string {
	b := promptBuilder{
		mode:             promptModeAgent,
		repo:             repo,
		commits:          commits,
		branchActivity:   branchActivity,
		previousSummary:  previousSummary,
		prData:           prData,
		maxCommits:       len(commits), // Agent always sees all commits
		maxMessageLength: maxMessageLength,
	}
	return b.buildCommitPrompt()
}
