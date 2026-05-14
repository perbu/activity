package git

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// defaultDiffExcludes contains pathspecs to filter out vendor directories and lock files.
// These patterns reduce noise in diffs and lower token usage for LLM analysis.
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
func GetCommitDiff(repoPath, sha string) (*DiffResult, error) {
	// Get filtered diff (excluding vendor/node_modules/lock files)
	args := []string{"-C", repoPath, "show", "--format=", sha, "--"}
	args = append(args, defaultDiffExcludes...)
	filteredCmd := exec.Command("git", args...)
	var filteredOut, filteredErr bytes.Buffer
	filteredCmd.Stdout = &filteredOut
	filteredCmd.Stderr = &filteredErr

	if err := filteredCmd.Run(); err != nil {
		return nil, fmt.Errorf("git show (filtered) failed: %w: %s", err, filteredErr.String())
	}

	// Get full diff to count suppressed lines
	fullCmd := exec.Command("git", "-C", repoPath, "show", "--format=", sha)
	var fullOut, fullErr bytes.Buffer
	fullCmd.Stdout = &fullOut
	fullCmd.Stderr = &fullErr

	if err := fullCmd.Run(); err != nil {
		return nil, fmt.Errorf("git show (full) failed: %w: %s", err, fullErr.String())
	}

	filtered := filteredOut.String()
	full := fullOut.String()

	filteredLines := strings.Count(filtered, "\n")
	fullLines := strings.Count(full, "\n")
	suppressed := fullLines - filteredLines

	result := &DiffResult{
		SuppressedLines: suppressed,
	}

	if suppressed > 0 {
		result.Diff = fmt.Sprintf("%s\n[%d lines suppressed from vendor/node_modules/lock files]\n",
			filtered, suppressed)
	} else {
		result.Diff = filtered
	}
	return result, nil
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
