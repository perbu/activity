package web

// RepoSummary is a view model for repository listings.
type RepoSummary struct {
	ID          int64
	Name        string
	URL         string
	Branch      string
	Active      bool
	Private     bool   // Requires GitHub App authentication
	External    bool   // External repo: include contributor analysis in summaries
	Description string // AI-generated description from README
	ForgeType   string // "github", "forgejo", or ""
	ReportCount int
	LastReport  string         // formatted date or "No reports"
	Sparkline   []SparklineBar // commit activity for last 8 weeks (oldest to newest)
}

// SparklineBar represents a single bar in a sparkline chart.
type SparklineBar struct {
	Value  int // raw commit count
	Height int // percentage height (0-100)
}

// RepoListData is the view model for the repository list page.
type RepoListData struct {
	Repos []RepoSummary
}

// RepoReportsData is the view model for a single repo's reports.
type RepoReportsData struct {
	Repo        RepoSummary
	Reports     []ReportSummary
	Years       []int
	CurrentYear int // 0 means "all"
}
