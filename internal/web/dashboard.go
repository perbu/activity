package web

// PageData is the common data structure for all pages.
type PageData struct {
	Title      string
	ActiveNav  string // "reports", "repos", "subscriptions", "admin", ""
	Content    any
	Error      string
	CurrentURL string
	User       *AuthUser
}

// DashboardData is the view model for the dashboard/index page.
type DashboardData struct {
	Reports    []ReportSummary
	TotalCount int
}
