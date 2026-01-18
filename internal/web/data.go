package web

import "html/template"

// PageData is the common data structure for all pages
type PageData struct {
	Title      string
	ActiveNav  string // "dashboard", "repos", ""
	Content    any
	Error      string
	CurrentURL string
}

// ReportSummary is a lightweight view model for report listings
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

// ReportDetail is a full view model for a single report
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
	AgentMode   bool
	CreatedAt   string
	UpdatedAt   string
	Summary     string
	SummaryHTML template.HTML
}

// RepoSummary is a view model for repository listings
type RepoSummary struct {
	ID          int64
	Name        string
	URL         string
	Branch      string
	Active      bool
	ReportCount int
	LastReport  string // formatted date or "No reports"
}

// DashboardData is the view model for the dashboard/index page
type DashboardData struct {
	Reports    []ReportSummary
	TotalCount int
}

// RepoListData is the view model for the repository list page
type RepoListData struct {
	Repos []RepoSummary
}

// RepoReportsData is the view model for a single repo's reports
type RepoReportsData struct {
	Repo        RepoSummary
	Reports     []ReportSummary
	Years       []int
	CurrentYear int // 0 means "all"
}

// ReportViewData is the view model for a single report detail
type ReportViewData struct {
	Report ReportDetail
}
