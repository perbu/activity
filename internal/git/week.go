package git

import (
	"fmt"
	"time"
)

// ISOWeekBounds returns the start (Monday 00:00:00) and end (Sunday 23:59:59) of an ISO week.
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

// GetCommitsForWeek retrieves commits for a specific ISO week.
func GetCommitsForWeek(repoPath string, year, week int) ([]Commit, error) {
	start, end := ISOWeekBounds(year, week)

	// Format dates for git --since/--until (ISO 8601 format)
	sinceStr := start.Format("2006-01-02T15:04:05")
	untilStr := end.Format("2006-01-02T15:04:05")

	return GetCommitsSince(repoPath, sinceStr, untilStr)
}

// ParseISOWeek parses a string in "2026-W02" format into year and week.
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

// FormatISOWeek formats a year and week into "2026-W02" format.
func FormatISOWeek(year, week int) string {
	return fmt.Sprintf("%d-W%02d", year, week)
}

// WeeksInRange returns all ISO weeks between start and end dates as [year, week] pairs.
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

// CurrentISOWeek returns the current ISO year and week number.
func CurrentISOWeek() (year, week int) {
	return time.Now().ISOWeek()
}
