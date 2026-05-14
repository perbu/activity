package git

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// GetFeatureBranchActivity returns commits on branches that aren't on the main branch
// within the specified week. Works with bare/mirror repositories where branches
// are local (no origin/ prefix).
func GetFeatureBranchActivity(repoPath, mainBranch string, year, week int) ([]BranchActivity, error) {
	// Get week bounds for date filtering
	start, end := ISOWeekBounds(year, week)
	sinceStr := start.Format("2006-01-02")
	untilStr := end.AddDate(0, 0, 1).Format("2006-01-02") // Add 1 day for inclusive end

	// List local branches (in a mirror, all branches are local)
	cmd := exec.Command("git", "-C", repoPath, "branch", "--format=%(refname:short)")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git branch failed: %w: %s", err, stderr.String())
	}

	branches := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	if len(branches) == 0 || (len(branches) == 1 && branches[0] == "") {
		return nil, nil
	}

	var activities []BranchActivity

	for _, branch := range branches {
		branch = strings.TrimSpace(branch)
		if branch == "" {
			continue
		}

		// Skip the main branch and HEAD pointer
		if branch == mainBranch || strings.HasSuffix(branch, "/HEAD") || strings.Contains(branch, "->") {
			continue
		}

		// Get commits on this branch that aren't on main, within the date range
		logCmd := exec.Command("git", "-C", repoPath, "log",
			branch, "--not", mainBranch,
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

		activities = append(activities, BranchActivity{
			BranchName:   branch,
			CommitCount:  len(lines),
			Authors:      authors,
			AuthorCounts: authorCounts,
		})
	}

	return activities, nil
}
