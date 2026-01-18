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
