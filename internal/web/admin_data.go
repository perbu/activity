package web

// AdminDashboardData is the view model for the admin dashboard.
type AdminDashboardData struct {
	RepoCount       int
	ReportCount     int
	SubscriberCount int
	AdminCount      int
}

// AdminReposData is the view model for admin repository management.
type AdminReposData struct {
	Repos []RepoSummary
}

// AdminSubscribersData is the view model for admin subscriber management.
type AdminSubscribersData struct {
	Subscribers []SubscriberSummary
}

// SubscriberSummary is a view model for subscriber listings.
type SubscriberSummary struct {
	ID           int64
	Email        string
	SubscribeAll bool
	CreatedAt    string
	Repos        []string // Names of subscribed repos (if not subscribe_all)
}

// AdminAdminsData is the view model for admin user management.
type AdminAdminsData struct {
	Admins      []AdminSummary
	CurrentUser string
}

// AdminSummary is a view model for admin listings.
type AdminSummary struct {
	ID        int64
	Email     string
	CreatedAt string
	CreatedBy string
}

// AdminActionsData is the view model for admin actions page.
type AdminActionsData struct {
	LastUpdate      string
	LastReportGen   string
	LastNewsletter  string
	ScheduleEnabled bool
	IsLeader        bool
}
