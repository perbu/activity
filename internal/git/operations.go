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

// defaultDiffExcludes contains pathspecs to filter out vendor directories and lock files
// These patterns reduce noise in diffs and lower token usage for LLM analysis
var defaultDiffExcludes = []string{
	":(exclude)vendor",
	":(exclude)**/vendor",
	":(exclude)node_modules",
	":(exclude)**/node_modules",
	":(exclude)go.sum",
	":(exclude)package-lock.json",
	":(exclude)yarn.lock",
	":(exclude)pnpm-lock.yaml",
	":(exclude)Cargo.lock",
	":(exclude)poetry.lock",
	":(exclude)composer.lock",
}

// GetCommitDiff returns the diff for a specific commit with vendor/lock files filtered out.
// Vendor directories (vendor/, node_modules/) and lock files are excluded by default.
// The response includes a note showing how many lines were suppressed.
// Use GetCommitDiffFull if you need the complete unfiltered diff.
func GetCommitDiff(repoPath, sha string) (string, error) {
	// Get filtered diff (excluding vendor/node_modules/lock files)
	args := []string{"-C", repoPath, "show", "--format=", sha, "--"}
	args = append(args, defaultDiffExcludes...)
	filteredCmd := exec.Command("git", args...)
	var filteredOut, filteredErr bytes.Buffer
	filteredCmd.Stdout = &filteredOut
	filteredCmd.Stderr = &filteredErr

	if err := filteredCmd.Run(); err != nil {
		return "", fmt.Errorf("git show (filtered) failed: %w: %s", err, filteredErr.String())
	}

	// Get full diff to count suppressed lines
	fullCmd := exec.Command("git", "-C", repoPath, "show", "--format=", sha)
	var fullOut, fullErr bytes.Buffer
	fullCmd.Stdout = &fullOut
	fullCmd.Stderr = &fullErr

	if err := fullCmd.Run(); err != nil {
		return "", fmt.Errorf("git show (full) failed: %w: %s", err, fullErr.String())
	}

	filtered := filteredOut.String()
	full := fullOut.String()

	filteredLines := strings.Count(filtered, "\n")
	fullLines := strings.Count(full, "\n")

	if suppressed := fullLines - filteredLines; suppressed > 0 {
		return fmt.Sprintf("%s\n[%d lines suppressed from vendor/node_modules/lock files]\n",
			filtered, suppressed), nil
	}
	return filtered, nil
}

// GetCommitDiffFull returns the complete diff for a commit without any filtering.
// Use this when you need to see vendor directories or lock file changes.
func GetCommitDiffFull(repoPath, sha string) (string, error) {
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

// SetRemoteURL updates the origin remote URL for a repository
func SetRemoteURL(repoPath, newURL string) error {
	cmd := exec.Command("git", "-C", repoPath, "remote", "set-url", "origin", newURL)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git remote set-url failed: %w: %s", err, stderr.String())
	}

	return nil
}

// GetRemoteURL returns the current origin remote URL for a repository
func GetRemoteURL(repoPath string) (string, error) {
	cmd := exec.Command("git", "-C", repoPath, "remote", "get-url", "origin")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git remote get-url failed: %w: %s", err, stderr.String())
	}

	return strings.TrimSpace(stdout.String()), nil
}

// CloneWithAuth clones a repository using an authenticated URL
// The token is injected into the URL for authentication
func CloneWithAuth(url, path, branch, token string) error {
	authURL, err := injectToken(url, token)
	if err != nil {
		return fmt.Errorf("failed to create authenticated URL: %w", err)
	}

	cmd := exec.Command("git", "clone", "--branch", branch, authURL, path)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git clone failed: %w: %s", err, stderr.String())
	}

	// After cloning, set the remote URL to the original (non-authenticated) URL
	// This prevents the token from being stored in .git/config
	if err := SetRemoteURL(path, url); err != nil {
		return fmt.Errorf("failed to reset remote URL: %w", err)
	}

	return nil
}

// PullWithAuth pulls a repository using an authenticated URL
// The token is temporarily injected for the pull operation
func PullWithAuth(repoPath, url, token string) error {
	authURL, err := injectToken(url, token)
	if err != nil {
		return fmt.Errorf("failed to create authenticated URL: %w", err)
	}

	// Temporarily set the authenticated URL
	if err := SetRemoteURL(repoPath, authURL); err != nil {
		return fmt.Errorf("failed to set authenticated URL: %w", err)
	}

	// Pull
	pullErr := Pull(repoPath)

	// Always restore the original URL, even if pull failed
	restoreErr := SetRemoteURL(repoPath, url)

	if pullErr != nil {
		return pullErr
	}
	if restoreErr != nil {
		return fmt.Errorf("failed to restore remote URL: %w", restoreErr)
	}

	return nil
}

// injectToken inserts an access token into a GitHub URL
// Input: https://github.com/owner/repo.git
// Output: https://x-access-token:TOKEN@github.com/owner/repo.git
func injectToken(originalURL, token string) (string, error) {
	// Simple string manipulation for HTTPS URLs
	if !strings.HasPrefix(originalURL, "https://") {
		return "", fmt.Errorf("token injection only supported for HTTPS URLs")
	}

	// Insert x-access-token:TOKEN@ after https://
	return "https://x-access-token:" + token + "@" + strings.TrimPrefix(originalURL, "https://"), nil
}

// FetchAll fetches all remote branches
func FetchAll(repoPath string) error {
	cmd := exec.Command("git", "-C", repoPath, "fetch", "--all", "--prune")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git fetch --all failed: %w: %s", err, stderr.String())
	}

	return nil
}

// FetchAllWithAuth fetches all remote branches using an authenticated URL
func FetchAllWithAuth(repoPath, url, token string) error {
	authURL, err := injectToken(url, token)
	if err != nil {
		return fmt.Errorf("failed to create authenticated URL: %w", err)
	}

	// Temporarily set the authenticated URL
	if err := SetRemoteURL(repoPath, authURL); err != nil {
		return fmt.Errorf("failed to set authenticated URL: %w", err)
	}

	// Fetch all
	fetchErr := FetchAll(repoPath)

	// Always restore the original URL, even if fetch failed
	restoreErr := SetRemoteURL(repoPath, url)

	if fetchErr != nil {
		return fetchErr
	}
	if restoreErr != nil {
		return fmt.Errorf("failed to restore remote URL: %w", restoreErr)
	}

	return nil
}

// BranchActivity represents activity on a single branch
type BranchActivity struct {
	BranchName   string
	CommitCount  int
	Authors      []string
	AuthorCounts map[string]int
}

// GetFeatureBranchActivity returns commits on remote branches that aren't on the main branch
// within the specified week
func GetFeatureBranchActivity(repoPath, mainBranch string, year, week int) ([]BranchActivity, error) {
	// Get week bounds for date filtering
	start, end := ISOWeekBounds(year, week)
	sinceStr := start.Format("2006-01-02")
	untilStr := end.AddDate(0, 0, 1).Format("2006-01-02") // Add 1 day for inclusive end

	// List remote branches
	cmd := exec.Command("git", "-C", repoPath, "branch", "-r", "--format=%(refname:short)")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git branch -r failed: %w: %s", err, stderr.String())
	}

	branches := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	if len(branches) == 0 || (len(branches) == 1 && branches[0] == "") {
		return nil, nil
	}

	// Build the main branch ref (e.g., "origin/main")
	mainRef := "origin/" + mainBranch

	var activities []BranchActivity

	for _, branch := range branches {
		branch = strings.TrimSpace(branch)
		if branch == "" {
			continue
		}

		// Skip the main branch and HEAD pointer
		if branch == mainRef || strings.HasSuffix(branch, "/HEAD") || strings.Contains(branch, "->") {
			continue
		}

		// Get commits on this branch that aren't on main, within the date range
		// Format: author name only
		logCmd := exec.Command("git", "-C", repoPath, "log",
			branch, "--not", mainRef,
			"--since="+sinceStr, "--until="+untilStr,
			"--format=%an")
		var logOut, logErr bytes.Buffer
		logCmd.Stdout = &logOut
		logCmd.Stderr = &logErr

		if err := logCmd.Run(); err != nil {
			// Skip branches that fail (might be orphaned, etc.)
			continue
		}

		output := strings.TrimSpace(logOut.String())
		if output == "" {
			continue // No commits in this date range
		}

		// Count commits by author
		authorCounts := make(map[string]int)
		lines := strings.Split(output, "\n")
		for _, author := range lines {
			author = strings.TrimSpace(author)
			if author != "" {
				authorCounts[author]++
			}
		}

		if len(authorCounts) == 0 {
			continue
		}

		// Build unique author list
		var authors []string
		for author := range authorCounts {
			authors = append(authors, author)
		}

		// Strip "origin/" prefix from branch name for cleaner display
		displayName := strings.TrimPrefix(branch, "origin/")

		activities = append(activities, BranchActivity{
			BranchName:   displayName,
			CommitCount:  len(lines),
			Authors:      authors,
			AuthorCounts: authorCounts,
		})
	}

	return activities, nil
}
