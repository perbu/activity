package git

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Commit represents a Git commit
type Commit struct {
	SHA     string
	Author  string
	Date    time.Time
	Message string
}

// Clone clones a repository to the specified path
func Clone(url, path, branch string) error {
	cmd := exec.Command("git", "clone", "--branch", branch, url, path)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git clone failed: %w: %s", err, stderr.String())
	}

	return nil
}

// Pull pulls the latest changes for a repository
func Pull(repoPath string) error {
	cmd := exec.Command("git", "-C", repoPath, "pull")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git pull failed: %w: %s", err, stderr.String())
	}

	return nil
}

// GetCurrentSHA returns the current HEAD SHA for a repository
func GetCurrentSHA(repoPath string) (string, error) {
	cmd := exec.Command("git", "-C", repoPath, "rev-parse", "HEAD")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git rev-parse failed: %w: %s", err, stderr.String())
	}

	return strings.TrimSpace(stdout.String()), nil
}

// GetCommitRange retrieves commits between two SHAs
func GetCommitRange(repoPath, fromSHA, toSHA string) ([]Commit, error) {
	// Format: SHA|Author|UnixTimestamp|Subject
	// Using %x1e (record separator) as delimiter to avoid conflicts
	format := "%H%x1e%an%x1e%at%x1e%s"

	var commitRange string
	if fromSHA == "" {
		// All commits up to toSHA
		commitRange = toSHA
	} else {
		// Commits from fromSHA (exclusive) to toSHA (inclusive)
		commitRange = fmt.Sprintf("%s..%s", fromSHA, toSHA)
	}

	cmd := exec.Command("git", "-C", repoPath, "log", "--format="+format, commitRange)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git log failed: %w: %s", err, stderr.String())
	}

	output := strings.TrimSpace(stdout.String())
	if output == "" {
		return []Commit{}, nil
	}

	lines := strings.Split(output, "\n")
	commits := make([]Commit, 0, len(lines))

	for _, line := range lines {
		parts := strings.Split(line, "\x1e")
		if len(parts) != 4 {
			continue
		}

		var timestamp int64
		fmt.Sscanf(parts[2], "%d", &timestamp)

		commits = append(commits, Commit{
			SHA:     parts[0],
			Author:  parts[1],
			Date:    time.Unix(timestamp, 0),
			Message: parts[3],
		})
	}

	return commits, nil
}

// GetCommitDiff returns the diff for a specific commit
// WARNING: This function is not currently used. If enabled for LLM analysis,
// it could significantly increase token usage and costs. See COST_CONTROLS.md
// for details on safeguards needed before using this in AI analysis.
func GetCommitDiff(repoPath, sha string) (string, error) {
	cmd := exec.Command("git", "-C", repoPath, "show", "--format=", sha)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git show failed: %w: %s", err, stderr.String())
	}

	return stdout.String(), nil
}

// GetCommitsSince retrieves commits since a date (optionally until a date)
// Uses git's native --since and --until flags which handle date parsing
// (relative dates like "1 week ago" work automatically)
func GetCommitsSince(repoPath, since, until string) ([]Commit, error) {
	format := "%H%x1e%an%x1e%at%x1e%s"

	args := []string{"-C", repoPath, "log", "--format=" + format}
	if since != "" {
		args = append(args, "--since="+since)
	}
	if until != "" {
		args = append(args, "--until="+until)
	}

	cmd := exec.Command("git", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git log failed: %w: %s", err, stderr.String())
	}

	return parseCommitOutput(stdout.String())
}

// GetLastNCommits retrieves the last N commits from a repository
func GetLastNCommits(repoPath string, n int) ([]Commit, error) {
	format := "%H%x1e%an%x1e%at%x1e%s"

	cmd := exec.Command("git", "-C", repoPath, "log", "--format="+format, fmt.Sprintf("-n%d", n))
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git log failed: %w: %s", err, stderr.String())
	}

	return parseCommitOutput(stdout.String())
}

// parseCommitOutput parses git log output with record separator format
func parseCommitOutput(output string) ([]Commit, error) {
	output = strings.TrimSpace(output)
	if output == "" {
		return []Commit{}, nil
	}

	lines := strings.Split(output, "\n")
	commits := make([]Commit, 0, len(lines))

	for _, line := range lines {
		parts := strings.Split(line, "\x1e")
		if len(parts) != 4 {
			continue
		}

		var timestamp int64
		fmt.Sscanf(parts[2], "%d", &timestamp)

		commits = append(commits, Commit{
			SHA:     parts[0],
			Author:  parts[1],
			Date:    time.Unix(timestamp, 0),
			Message: parts[3],
		})
	}

	return commits, nil
}

// AuthorStats contains statistics about an author's contributions
type AuthorStats struct {
	Name         string
	TotalCommits int
	FirstCommit  time.Time
	LastCommit   time.Time
}

// GetAuthorStats retrieves statistics about an author in the repository
func GetAuthorStats(repoPath, authorName string) (*AuthorStats, error) {
	// Get total commit count for this author
	countCmd := exec.Command("git", "-C", repoPath, "rev-list", "--count", "--author="+authorName, "HEAD")
	var countOut, countErr bytes.Buffer
	countCmd.Stdout = &countOut
	countCmd.Stderr = &countErr

	if err := countCmd.Run(); err != nil {
		return nil, fmt.Errorf("git rev-list count failed: %w: %s", err, countErr.String())
	}

	var totalCommits int
	fmt.Sscanf(strings.TrimSpace(countOut.String()), "%d", &totalCommits)

	if totalCommits == 0 {
		return &AuthorStats{Name: authorName, TotalCommits: 0}, nil
	}

	// Get first commit date (oldest)
	firstCmd := exec.Command("git", "-C", repoPath, "log", "--author="+authorName, "--format=%at", "--reverse", "-1")
	var firstOut, firstErr bytes.Buffer
	firstCmd.Stdout = &firstOut
	firstCmd.Stderr = &firstErr

	if err := firstCmd.Run(); err != nil {
		return nil, fmt.Errorf("git log (first) failed: %w: %s", err, firstErr.String())
	}

	var firstTimestamp int64
	fmt.Sscanf(strings.TrimSpace(firstOut.String()), "%d", &firstTimestamp)

	// Get last commit date (most recent)
	lastCmd := exec.Command("git", "-C", repoPath, "log", "--author="+authorName, "--format=%at", "-1")
	var lastOut, lastErr bytes.Buffer
	lastCmd.Stdout = &lastOut
	lastCmd.Stderr = &lastErr

	if err := lastCmd.Run(); err != nil {
		return nil, fmt.Errorf("git log (last) failed: %w: %s", err, lastErr.String())
	}

	var lastTimestamp int64
	fmt.Sscanf(strings.TrimSpace(lastOut.String()), "%d", &lastTimestamp)

	return &AuthorStats{
		Name:         authorName,
		TotalCommits: totalCommits,
		FirstCommit:  time.Unix(firstTimestamp, 0),
		LastCommit:   time.Unix(lastTimestamp, 0),
	}, nil
}

// GetCommitInfo retrieves detailed information about a commit
func GetCommitInfo(repoPath, sha string) (*Commit, error) {
	format := "%H%x1e%an%x1e%at%x1e%B"
	cmd := exec.Command("git", "-C", repoPath, "show", "--format="+format, "--no-patch", sha)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git show failed: %w: %s", err, stderr.String())
	}

	parts := strings.Split(strings.TrimSpace(stdout.String()), "\x1e")
	if len(parts) != 4 {
		return nil, fmt.Errorf("unexpected git show output format")
	}

	var timestamp int64
	fmt.Sscanf(parts[2], "%d", &timestamp)

	return &Commit{
		SHA:     parts[0],
		Author:  parts[1],
		Date:    time.Unix(timestamp, 0),
		Message: parts[3],
	}, nil
}

// ISOWeekBounds returns the start (Monday 00:00:00) and end (Sunday 23:59:59) of an ISO week
func ISOWeekBounds(year, week int) (start, end time.Time) {
	// Find January 4th of the given year (always in week 1 per ISO 8601)
	jan4 := time.Date(year, time.January, 4, 0, 0, 0, 0, time.UTC)

	// Find the Monday of week 1
	weekday := int(jan4.Weekday())
	if weekday == 0 {
		weekday = 7 // Sunday = 7
	}
	week1Monday := jan4.AddDate(0, 0, -(weekday - 1))

	// Calculate the Monday of the target week
	start = week1Monday.AddDate(0, 0, (week-1)*7)

	// End is Sunday 23:59:59 of the same week
	end = start.AddDate(0, 0, 6).Add(23*time.Hour + 59*time.Minute + 59*time.Second)

	return start, end
}

// GetCommitsForWeek retrieves commits for a specific ISO week
func GetCommitsForWeek(repoPath string, year, week int) ([]Commit, error) {
	start, end := ISOWeekBounds(year, week)

	// Format dates for git --since/--until (ISO 8601 format)
	sinceStr := start.Format("2006-01-02T15:04:05")
	untilStr := end.Format("2006-01-02T15:04:05")

	return GetCommitsSince(repoPath, sinceStr, untilStr)
}

// ParseISOWeek parses a string in "2026-W02" format into year and week
func ParseISOWeek(s string) (year, week int, err error) {
	n, err := fmt.Sscanf(s, "%d-W%d", &year, &week)
	if err != nil || n != 2 {
		return 0, 0, fmt.Errorf("invalid ISO week format: %s (expected YYYY-Www)", s)
	}
	if week < 1 || week > 53 {
		return 0, 0, fmt.Errorf("invalid week number: %d (must be 1-53)", week)
	}
	return year, week, nil
}

// FormatISOWeek formats a year and week into "2026-W02" format
func FormatISOWeek(year, week int) string {
	return fmt.Sprintf("%d-W%02d", year, week)
}

// WeeksInRange returns all ISO weeks between start and end dates as [year, week] pairs
func WeeksInRange(start, end time.Time) [][2]int {
	var weeks [][2]int
	seen := make(map[string]bool)

	// Iterate day by day from start to end
	current := start
	for !current.After(end) {
		year, week := current.ISOWeek()
		key := fmt.Sprintf("%d-%d", year, week)
		if !seen[key] {
			seen[key] = true
			weeks = append(weeks, [2]int{year, week})
		}
		current = current.AddDate(0, 0, 1)
	}

	return weeks
}

// CurrentISOWeek returns the current ISO year and week number
func CurrentISOWeek() (year, week int) {
	return time.Now().ISOWeek()
}
