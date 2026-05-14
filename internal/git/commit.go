package git

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// GetCurrentSHA returns the current HEAD SHA for a repository.
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

// GetBranchSHA returns the SHA for a specific branch.
// This is needed for bare repos where HEAD points to the default branch.
func GetBranchSHA(repoPath, branch string) (string, error) {
	cmd := exec.Command("git", "-C", repoPath, "rev-parse", "refs/heads/"+branch)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git rev-parse refs/heads/%s failed: %w: %s", branch, err, stderr.String())
	}

	return strings.TrimSpace(stdout.String()), nil
}

// GetCommitRange retrieves commits between two SHAs.
func GetCommitRange(repoPath, fromSHA, toSHA string) ([]Commit, error) {
	// Format: SHA|Author|UnixTimestamp|Subject
	// Using %x1e (record separator) as delimiter to avoid conflicts
	format := "%H%x1e%an%x1e%at%x1e%s"

	var commitRange string
	if fromSHA == "" {
		commitRange = toSHA
	} else {
		commitRange = fmt.Sprintf("%s..%s", fromSHA, toSHA)
	}

	cmd := exec.Command("git", "-C", repoPath, "log", "--format="+format, commitRange)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git log failed: %w: %s", err, stderr.String())
	}

	return parseCommitOutput(stdout.String())
}

// GetCommitsSince retrieves commits since a date (optionally until a date).
// Uses git's native --since and --until flags which handle date parsing
// (relative dates like "1 week ago" work automatically).
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

// GetLastNCommits retrieves the last N commits from a repository.
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

// parseCommitOutput parses git log output with record separator format.
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

// GetAuthorStats retrieves statistics about an author in the repository.
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

// GetCommitInfo retrieves detailed information about a commit.
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
