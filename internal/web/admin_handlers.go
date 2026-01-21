package web

import (
	"context"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"os"
	"strconv"

	"github.com/perbu/activity/internal/service"
)

// handleAdmin serves the admin dashboard
func (s *Server) handleAdmin(w http.ResponseWriter, r *http.Request) {
	repos, _ := s.db.ListRepositories(nil)
	reports, _ := s.db.ListAllWeeklyReports(nil)
	subscribers, _ := s.db.ListSubscribers()
	admins, _ := s.db.ListAdmins()

	data := PageData{
		Title:     "Admin",
		ActiveNav: "admin",
		User:      GetUser(r),
		Content: AdminDashboardData{
			RepoCount:       len(repos),
			ReportCount:     len(reports),
			SubscriberCount: len(subscribers),
			AdminCount:      len(admins),
		},
	}

	s.render(w, s.templates.admin, data)
}

// handleAdminRepos serves the repository management page
func (s *Server) handleAdminRepos(w http.ResponseWriter, r *http.Request) {
	repos, err := s.db.ListRepositories(nil)
	if err != nil {
		s.renderError(w, r, "Failed to load repositories", err)
		return
	}

	summaries := make([]RepoSummary, 0, len(repos))
	for _, repo := range repos {
		reports, _ := s.db.ListWeeklyReportsByRepo(repo.ID, nil)
		summary := RepoSummary{
			ID:          repo.ID,
			Name:        repo.Name,
			URL:         repo.URL,
			Branch:      repo.Branch,
			Active:      repo.Active,
			Description: repo.Description.String,
			ReportCount: len(reports),
			LastReport:  "No reports",
		}
		if len(reports) > 0 {
			summary.LastReport = reports[0].CreatedAt.Format("2006-01-02")
		}
		summaries = append(summaries, summary)
	}

	data := PageData{
		Title:     "Admin - Repositories",
		ActiveNav: "admin",
		User:      GetUser(r),
		Content: AdminReposData{
			Repos: summaries,
		},
	}

	s.render(w, s.templates.adminRepos, data)
}

// handleAdminRepoAdd handles adding a new repository
func (s *Server) handleAdminRepoAdd(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	name := r.FormValue("name")
	url := r.FormValue("url")
	branch := r.FormValue("branch")
	private := r.FormValue("private") == "on"

	if name == "" || url == "" {
		http.Error(w, "Name and URL are required", http.StatusBadRequest)
		return
	}
	if branch == "" {
		branch = "main"
	}

	_, err := s.services.Repo.Add(context.Background(), service.AddOptions{
		Name:    name,
		URL:     url,
		Branch:  branch,
		Private: private,
	})
	if err != nil {
		slog.Error("Failed to add repository", "name", name, "error", err)
		http.Error(w, "Failed to add repository: "+err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/admin/repos", http.StatusSeeOther)
}

// handleAdminRepoRemove handles removing a repository
func (s *Server) handleAdminRepoRemove(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	name := r.FormValue("name")
	keepFiles := r.FormValue("keep_files") == "on"

	if name == "" {
		http.Error(w, "Repository name is required", http.StatusBadRequest)
		return
	}

	if err := s.services.Repo.Remove(name, keepFiles); err != nil {
		slog.Error("Failed to remove repository", "name", name, "error", err)
		http.Error(w, "Failed to remove repository: "+err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/admin/repos", http.StatusSeeOther)
}

// handleAdminRepoToggle handles activating/deactivating a repository
func (s *Server) handleAdminRepoToggle(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	name := r.FormValue("name")
	action := r.FormValue("action") // "activate" or "deactivate"

	if name == "" {
		http.Error(w, "Repository name is required", http.StatusBadRequest)
		return
	}

	var err error
	if action == "activate" {
		err = s.services.Repo.Activate(name)
	} else {
		err = s.services.Repo.Deactivate(name)
	}

	if err != nil {
		slog.Error("Failed to toggle repository", "name", name, "action", action, "error", err)
		http.Error(w, "Failed to toggle repository: "+err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/admin/repos", http.StatusSeeOther)
}

// handleAdminRepoSetURL handles updating a repository's URL
func (s *Server) handleAdminRepoSetURL(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	name := r.FormValue("name")
	url := r.FormValue("url")

	if name == "" || url == "" {
		http.Error(w, "Repository name and URL are required", http.StatusBadRequest)
		return
	}

	if err := s.services.Repo.SetURL(name, url); err != nil {
		slog.Error("Failed to set repository URL", "name", name, "error", err)
		http.Error(w, "Failed to set repository URL: "+err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/admin/repos", http.StatusSeeOther)
}

// handleAdminSubscribers serves the subscriber management page
func (s *Server) handleAdminSubscribers(w http.ResponseWriter, r *http.Request) {
	subscribers, err := s.db.ListSubscribers()
	if err != nil {
		s.renderError(w, r, "Failed to load subscribers", err)
		return
	}

	summaries := make([]SubscriberSummary, 0, len(subscribers))
	for _, sub := range subscribers {
		summary := SubscriberSummary{
			ID:           sub.ID,
			Email:        sub.Email,
			SubscribeAll: sub.SubscribeAll,
			CreatedAt:    sub.CreatedAt.Format("2006-01-02"),
		}

		// Get subscribed repos if not subscribe_all
		if !sub.SubscribeAll {
			subs, _ := s.db.ListSubscriptionsBySubscriber(sub.ID)
			for _, subscription := range subs {
				repo, err := s.db.GetRepository(subscription.RepoID)
				if err == nil {
					summary.Repos = append(summary.Repos, repo.Name)
				}
			}
		}

		summaries = append(summaries, summary)
	}

	data := PageData{
		Title:     "Admin - Subscribers",
		ActiveNav: "admin",
		User:      GetUser(r),
		Content: AdminSubscribersData{
			Subscribers: summaries,
		},
	}

	s.render(w, s.templates.adminSubscribers, data)
}

// handleAdminSubscriberAdd handles adding a new subscriber
func (s *Server) handleAdminSubscriberAdd(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	email := r.FormValue("email")
	subscribeAll := r.FormValue("subscribe_all") == "on"

	if email == "" {
		http.Error(w, "Email is required", http.StatusBadRequest)
		return
	}

	_, err := s.services.Newsletter.AddSubscriber(email, subscribeAll)
	if err != nil {
		slog.Error("Failed to add subscriber", "email", email, "error", err)
		http.Error(w, "Failed to add subscriber: "+err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/admin/subscribers", http.StatusSeeOther)
}

// handleAdminSubscriberRemove handles removing a subscriber
func (s *Server) handleAdminSubscriberRemove(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	email := r.FormValue("email")

	if email == "" {
		http.Error(w, "Email is required", http.StatusBadRequest)
		return
	}

	if err := s.services.Newsletter.RemoveSubscriber(email); err != nil {
		slog.Error("Failed to remove subscriber", "email", email, "error", err)
		http.Error(w, "Failed to remove subscriber: "+err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/admin/subscribers", http.StatusSeeOther)
}

// handleAdminActions serves the actions page for manual triggers
func (s *Server) handleAdminActions(w http.ResponseWriter, r *http.Request) {
	data := PageData{
		Title:     "Admin - Actions",
		ActiveNav: "admin",
		User:      GetUser(r),
		Content:   AdminActionsData{},
	}

	s.render(w, s.templates.adminActions, data)
}

// handleAdminUpdateRepos handles updating all repositories
func (s *Server) handleAdminUpdateRepos(w http.ResponseWriter, r *http.Request) {
	results, err := s.services.Repo.UpdateAll(context.Background())
	if err != nil {
		slog.Error("Failed to update repositories", "error", err)
		http.Error(w, "Failed to update repositories: "+err.Error(), http.StatusInternalServerError)
		return
	}

	msg := fmt.Sprintf("Updated %d repositories", len(results))
	slog.Info(msg)

	http.Redirect(w, r, "/admin/actions?success="+msg, http.StatusSeeOther)
}

// handleAdminGenerateReport handles generating reports
func (s *Server) handleAdminGenerateReport(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	// Generate reports for last week for all repos
	results, err := s.services.Report.GenerateLastWeek(context.Background(), false)
	if err != nil {
		slog.Error("Failed to generate reports", "error", err)
		http.Error(w, "Failed to generate reports: "+err.Error(), http.StatusInternalServerError)
		return
	}

	generated := 0
	for _, r := range results {
		generated += r.Generated
	}

	msg := fmt.Sprintf("Generated %d reports for %d repositories", generated, len(results))
	slog.Info(msg)

	http.Redirect(w, r, "/admin/actions?success="+msg, http.StatusSeeOther)
}

// handleAdminSendNewsletter handles sending newsletters
func (s *Server) handleAdminSendNewsletter(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	sinceStr := r.FormValue("since")
	if sinceStr == "" {
		sinceStr = "7d"
	}

	since, err := service.ParseSinceDuration(sinceStr)
	if err != nil {
		http.Error(w, "Invalid duration: "+err.Error(), http.StatusBadRequest)
		return
	}

	dryRun := r.FormValue("dry_run") == "on"

	result, err := s.services.Newsletter.Send(context.Background(), since, dryRun, os.Stdout)
	if err != nil {
		slog.Error("Failed to send newsletters", "error", err)
		http.Error(w, "Failed to send newsletters: "+err.Error(), http.StatusInternalServerError)
		return
	}

	msg := fmt.Sprintf("Sent %d newsletters (skipped %d, errors %d)", result.Sent, result.Skipped, result.Errors)
	if dryRun {
		msg = "[DRY RUN] " + msg
	}
	slog.Info(msg)

	http.Redirect(w, r, "/admin/actions?success="+msg, http.StatusSeeOther)
}

// handleAdminAdmins serves the admin user management page
func (s *Server) handleAdminAdmins(w http.ResponseWriter, r *http.Request) {
	admins, err := s.db.ListAdmins()
	if err != nil {
		s.renderError(w, r, "Failed to load admins", err)
		return
	}

	summaries := make([]AdminSummary, 0, len(admins))
	for _, admin := range admins {
		createdBy := "system"
		if admin.CreatedBy.Valid {
			createdBy = admin.CreatedBy.String
		}
		summaries = append(summaries, AdminSummary{
			ID:        admin.ID,
			Email:     admin.Email,
			CreatedAt: admin.CreatedAt.Format("2006-01-02"),
			CreatedBy: createdBy,
		})
	}

	user := GetUser(r)
	data := PageData{
		Title:     "Admin - Admin Users",
		ActiveNav: "admin",
		User:      user,
		Content: AdminAdminsData{
			Admins:      summaries,
			CurrentUser: user.Email,
		},
	}

	s.render(w, s.templates.adminAdmins, data)
}

// handleAdminAdminAdd handles adding a new admin
func (s *Server) handleAdminAdminAdd(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	email := r.FormValue("email")

	if email == "" {
		http.Error(w, "Email is required", http.StatusBadRequest)
		return
	}

	user := GetUser(r)
	_, err := s.services.Admin.Add(email, user.Email)
	if err != nil {
		slog.Error("Failed to add admin", "email", email, "error", err)
		http.Error(w, "Failed to add admin: "+err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/admin/admins", http.StatusSeeOther)
}

// handleAdminAdminRemove handles removing an admin
func (s *Server) handleAdminAdminRemove(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	idStr := r.FormValue("id")

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid admin ID", http.StatusBadRequest)
		return
	}

	// Prevent removing yourself
	admin, err := s.db.GetAdmin(id)
	if err != nil {
		http.Error(w, "Admin not found", http.StatusNotFound)
		return
	}

	user := GetUser(r)
	if admin.Email == user.Email {
		http.Error(w, "Cannot remove yourself as admin", http.StatusBadRequest)
		return
	}

	if err := s.services.Admin.Remove(id); err != nil {
		slog.Error("Failed to remove admin", "id", id, "error", err)
		http.Error(w, "Failed to remove admin: "+err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/admin/admins", http.StatusSeeOther)
}

// renderAdminError renders an error for admin pages
func (s *Server) renderAdminError(w http.ResponseWriter, r *http.Request, tmpl *template.Template, message string, err error) {
	errMsg := message
	if err != nil {
		errMsg = message + ": " + err.Error()
	}

	data := PageData{
		Title:     "Admin Error",
		ActiveNav: "admin",
		User:      GetUser(r),
		Error:     errMsg,
		Content:   nil,
	}

	w.WriteHeader(http.StatusInternalServerError)
	s.render(w, tmpl, data)
}
