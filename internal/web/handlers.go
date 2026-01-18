package web

import (
	"bytes"
	"encoding/json"
	"html/template"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/perbu/activity/internal/db"
	"github.com/perbu/activity/internal/git"
	"github.com/yuin/goldmark"
)

// handleIndex serves the dashboard with recent reports
func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	reports, err := s.db.ListAllWeeklyReports(nil)
	if err != nil {
		s.renderError(w, "Failed to load reports", err)
		return
	}

	// Limit to 20 most recent
	if len(reports) > 20 {
		reports = reports[:20]
	}

	// Get repo names for all reports
	repoNames := make(map[int64]string)
	repos, _ := s.db.ListRepositories(nil)
	for _, repo := range repos {
		repoNames[repo.ID] = repo.Name
	}

	// Convert to view models
	summaries := make([]ReportSummary, 0, len(reports))
	for _, r := range reports {
		summaries = append(summaries, toReportSummary(r, repoNames[r.RepoID]))
	}

	data := PageData{
		Title:     "Dashboard",
		ActiveNav: "dashboard",
		Content: DashboardData{
			Reports:    summaries,
			TotalCount: len(reports),
		},
	}

	s.render(w, s.templates.index, data)
}

// handleRepoList serves the repository list page
func (s *Server) handleRepoList(w http.ResponseWriter, r *http.Request) {
	repos, err := s.db.ListRepositories(nil)
	if err != nil {
		s.renderError(w, "Failed to load repositories", err)
		return
	}

	// Build view models with report counts
	summaries := make([]RepoSummary, 0, len(repos))
	for _, repo := range repos {
		reports, _ := s.db.ListWeeklyReportsByRepo(repo.ID, nil)
		summary := RepoSummary{
			ID:          repo.ID,
			Name:        repo.Name,
			URL:         repo.URL,
			Branch:      repo.Branch,
			Active:      repo.Active,
			ReportCount: len(reports),
			LastReport:  "No reports",
		}
		if len(reports) > 0 {
			summary.LastReport = reports[0].CreatedAt.Format("2006-01-02")
		}
		summaries = append(summaries, summary)
	}

	data := PageData{
		Title:     "Repositories",
		ActiveNav: "repos",
		Content: RepoListData{
			Repos: summaries,
		},
	}

	s.render(w, s.templates.repos, data)
}

// handleRepoReports serves the reports page for a specific repository
func (s *Server) handleRepoReports(w http.ResponseWriter, r *http.Request) {
	repoName := r.PathValue("name")
	if repoName == "" {
		s.renderError(w, "Repository name required", nil)
		return
	}

	repo, err := s.db.GetRepositoryByName(repoName)
	if err != nil {
		s.renderError(w, "Repository not found: "+repoName, err)
		return
	}

	// Parse year filter
	var yearFilter *int
	yearStr := r.URL.Query().Get("year")
	if yearStr != "" {
		if y, err := strconv.Atoi(yearStr); err == nil && y > 0 {
			yearFilter = &y
		}
	}

	reports, err := s.db.ListWeeklyReportsByRepo(repo.ID, yearFilter)
	if err != nil {
		s.renderError(w, "Failed to load reports", err)
		return
	}

	// Build report summaries
	summaries := make([]ReportSummary, 0, len(reports))
	for _, r := range reports {
		summaries = append(summaries, toReportSummary(r, repo.Name))
	}

	// Collect unique years for filter
	allReports, _ := s.db.ListWeeklyReportsByRepo(repo.ID, nil)
	yearSet := make(map[int]bool)
	for _, r := range allReports {
		yearSet[r.Year] = true
	}
	var years []int
	for y := range yearSet {
		years = append(years, y)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(years)))

	currentYear := 0
	if yearFilter != nil {
		currentYear = *yearFilter
	}

	repoSummary := RepoSummary{
		ID:          repo.ID,
		Name:        repo.Name,
		URL:         repo.URL,
		Branch:      repo.Branch,
		Active:      repo.Active,
		ReportCount: len(allReports),
		LastReport:  "No reports",
	}
	if len(allReports) > 0 {
		repoSummary.LastReport = allReports[0].CreatedAt.Format("2006-01-02")
	}

	data := PageData{
		Title:     repo.Name + " Reports",
		ActiveNav: "repos",
		Content: RepoReportsData{
			Repo:        repoSummary,
			Reports:     summaries,
			Years:       years,
			CurrentYear: currentYear,
		},
	}

	s.render(w, s.templates.repoDetail, data)
}

// handleReportView serves a single report detail page
func (s *Server) handleReportView(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		s.renderError(w, "Invalid report ID", err)
		return
	}

	report, err := s.db.GetWeeklyReport(id)
	if err != nil {
		s.renderError(w, "Report not found", err)
		return
	}

	// Get repo name
	repo, err := s.db.GetRepository(report.RepoID)
	if err != nil {
		s.renderError(w, "Repository not found", err)
		return
	}

	detail := toReportDetail(report, repo.Name)

	data := PageData{
		Title:     repo.Name + " " + detail.WeekLabel,
		ActiveNav: "",
		Content: ReportViewData{
			Report: detail,
		},
	}

	s.render(w, s.templates.report, data)
}

// render executes a template and writes to the response
func (s *Server) render(w http.ResponseWriter, tmpl *template.Template, data PageData) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.Execute(w, data); err != nil {
		http.Error(w, "Template error: "+err.Error(), http.StatusInternalServerError)
	}
}

// renderError renders an error page
func (s *Server) renderError(w http.ResponseWriter, message string, err error) {
	errMsg := message
	if err != nil {
		errMsg = message + ": " + err.Error()
	}

	data := PageData{
		Title:     "Error",
		ActiveNav: "",
		Error:     errMsg,
		Content:   nil,
	}

	w.WriteHeader(http.StatusInternalServerError)
	s.render(w, s.templates.index, data)
}

// toReportSummary converts a db.WeeklyReport to a ReportSummary view model
func toReportSummary(r *db.WeeklyReport, repoName string) ReportSummary {
	preview := ""
	if r.Summary.Valid && r.Summary.String != "" {
		preview = r.Summary.String
		// Get first line and truncate
		if idx := strings.Index(preview, "\n"); idx > 0 {
			preview = preview[:idx]
		}
		// Strip markdown header prefix
		preview = strings.TrimPrefix(preview, "# ")
		preview = strings.TrimPrefix(preview, "## ")
		preview = strings.TrimPrefix(preview, "### ")
		if len(preview) > 60 {
			preview = preview[:57] + "..."
		}
	}

	return ReportSummary{
		ID:          r.ID,
		RepoID:      r.RepoID,
		RepoName:    repoName,
		Year:        r.Year,
		Week:        r.Week,
		WeekLabel:   git.FormatISOWeek(r.Year, r.Week),
		WeekStart:   r.WeekStart.Format("Jan 2"),
		WeekEnd:     r.WeekEnd.Format("Jan 2"),
		CommitCount: r.CommitCount,
		CreatedAt:   r.CreatedAt.Format("2006-01-02"),
		Preview:     preview,
	}
}

// toReportDetail converts a db.WeeklyReport to a ReportDetail view model
func toReportDetail(r *db.WeeklyReport, repoName string) ReportDetail {
	detail := ReportDetail{
		ID:          r.ID,
		RepoID:      r.RepoID,
		RepoName:    repoName,
		Year:        r.Year,
		Week:        r.Week,
		WeekLabel:   git.FormatISOWeek(r.Year, r.Week),
		WeekStart:   r.WeekStart.Format("2006-01-02"),
		WeekEnd:     r.WeekEnd.Format("2006-01-02"),
		CommitCount: r.CommitCount,
		AgentMode:   r.AgentMode,
		CreatedAt:   r.CreatedAt.Format("2006-01-02 15:04"),
		UpdatedAt:   r.UpdatedAt.Format("2006-01-02 15:04"),
	}

	// Parse authors from metadata
	if r.Metadata.Valid && r.Metadata.String != "" {
		var metadata struct {
			Authors []string `json:"authors"`
		}
		if err := json.Unmarshal([]byte(r.Metadata.String), &metadata); err == nil {
			detail.Authors = metadata.Authors
		}
	}

	// Convert summary markdown to HTML
	if r.Summary.Valid && r.Summary.String != "" {
		detail.Summary = r.Summary.String
		var buf bytes.Buffer
		if err := goldmark.Convert([]byte(r.Summary.String), &buf); err == nil {
			detail.SummaryHTML = template.HTML(buf.String())
		}
	}

	return detail
}
