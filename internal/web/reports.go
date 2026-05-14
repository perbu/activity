package web

import "html/template"

// ReportSummary is a lightweight view model for report listings.
type ReportSummary struct {
	ID          int64
	RepoID      int64
	RepoName    string
	Year        int
	Week        int
	WeekLabel   string // e.g., "2026-W02"
	WeekStart   string // formatted date
	WeekEnd     string // formatted date
	CommitCount int
	CreatedAt   string // formatted date
	Preview     string // first line of summary, truncated
}

// CommitView is a view model for a single commit in the report detail.
type CommitView struct {
	SHA     string
	Author  string
	Date    string
	Message string
}

// ReportDetail is a full view model for a single report.
type ReportDetail struct {
	ID          int64
	RepoID      int64
	RepoName    string
	Year        int
	Week        int
	WeekLabel   string
	WeekStart   string
	WeekEnd     string
	CommitCount int
	Authors     []string
	Commits     []CommitView
	AgentMode   bool
	CreatedAt   string
	UpdatedAt   string
	Summary     string
	SummaryHTML template.HTML
}

// ReportViewData is the view model for a single report detail.
type ReportViewData struct {
	Report       ReportDetail
	SourceRunID  int64
	AnalysisLogs []AnalysisLogView
}

// AnalysisLogView is a view model for analysis log entries.
type AnalysisLogView struct {
	ID        int64
	LogType   string // "prompt", "response", "tool_call", "tool_result"
	ToolName  string // Only for tool types
	Content   string
	Sequence  int
	CreatedAt string
}
