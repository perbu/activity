package web

import "html/template"

// PageData is the common data structure for all pages
type PageData struct {
	Title      string
	ActiveNav  string // "dashboard", "repos", "admin", ""
	Content    any
	Error      string
	CurrentURL string
	User       *AuthUser
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
	Description string // AI-generated description from README
	ReportCount int
	LastReport  string // formatted date or "No reports"
	Sparkline []SparklineBar // commit activity for last 8 weeks (oldest to newest)
}

// SparklineBar represents a single bar in a sparkline chart
type SparklineBar struct {
	Value   int // raw commit count
	Height  int // percentage height (0-100)
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

// AdminDashboardData is the view model for the admin dashboard
type AdminDashboardData struct {
	RepoCount       int
	ReportCount     int
	SubscriberCount int
	AdminCount      int
}

// AdminReposData is the view model for admin repository management
type AdminReposData struct {
	Repos []RepoSummary
}

// AdminSubscribersData is the view model for admin subscriber management
type AdminSubscribersData struct {
	Subscribers []SubscriberSummary
}

// SubscriberSummary is a view model for subscriber listings
type SubscriberSummary struct {
	ID           int64
	Email        string
	SubscribeAll bool
	CreatedAt    string
	Repos        []string // Names of subscribed repos (if not subscribe_all)
}

// AdminAdminsData is the view model for admin user management
type AdminAdminsData struct {
	Admins      []AdminSummary
	CurrentUser string
}

// AdminSummary is a view model for admin listings
type AdminSummary struct {
	ID        int64
	Email     string
	CreatedAt string
	CreatedBy string
}

// AdminActionsData is the view model for admin actions page
type AdminActionsData struct {
	LastUpdate     string
	LastReportGen  string
	LastNewsletter string
}
