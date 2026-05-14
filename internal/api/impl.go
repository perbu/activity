package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"strings"
	"time"

	openapitypes "github.com/oapi-codegen/runtime/types"
	"github.com/perbu/activity/internal/db"
	"github.com/perbu/activity/internal/service"
)

type Impl struct {
	repos   *service.RepoService
	reports *service.ReportService
}

func NewImpl(repos *service.RepoService, reports *service.ReportService) *Impl {
	return &Impl{repos: repos, reports: reports}
}

func (i *Impl) ListRepos(_ context.Context, req ListReposRequestObject) (ListReposResponseObject, error) {
	repos, err := i.repos.List(req.Params.Active)
	if err != nil {
		return nil, err
	}
	out := make(ListRepos200JSONResponse, 0, len(repos))
	for _, r := range repos {
		out = append(out, toRepository(r))
	}
	return out, nil
}

func (i *Impl) ListReportsForRepo(_ context.Context, req ListReportsForRepoRequestObject) (ListReportsForRepoResponseObject, error) {
	repo, err := i.repos.Get(req.Name)
	if err != nil {
		if isNotFound(err) {
			return ListReportsForRepo404JSONResponse{NotFoundJSONResponse: NotFoundJSONResponse{Error: "repository not found"}}, nil
		}
		return nil, err
	}
	reports, err := i.reports.ListReports(repo.ID, req.Params.Year)
	if err != nil {
		return nil, err
	}
	out := make(ListReportsForRepo200JSONResponse, 0, len(reports))
	for _, r := range reports {
		out = append(out, toReportSummary(r))
	}
	return out, nil
}

func (i *Impl) GetReport(_ context.Context, req GetReportRequestObject) (GetReportResponseObject, error) {
	report, err := i.reports.GetReport(req.Id)
	if err != nil {
		if isNotFound(err) {
			return GetReport404JSONResponse{NotFoundJSONResponse: NotFoundJSONResponse{Error: "report not found"}}, nil
		}
		return nil, err
	}
	return GetReport200JSONResponse(toReport(report)), nil
}

// isNotFound matches the db package's "not found" error text. Brittle but
// consistent with how the rest of the codebase signals missing rows; revisit
// when db gains a proper sentinel.
func isNotFound(err error) bool {
	return err != nil && strings.Contains(err.Error(), "not found")
}

func toRepository(r *db.Repository) Repository {
	out := Repository{
		Id:          r.ID,
		Name:        r.Name,
		Url:         r.URL,
		Branch:      r.Branch,
		Active:      r.Active,
		Private:     r.Private,
		External:    r.External,
		CreatedAt:   r.CreatedAt,
		UpdatedAt:   r.UpdatedAt,
		Description: nullStrPtr(r.Description),
		ForgeOwner:  nullStrPtr(r.ForgeOwner),
		ForgeRepo:   nullStrPtr(r.ForgeRepo),
	}
	if r.ForgeType.Valid {
		ft := RepositoryForgeType(r.ForgeType.String)
		out.ForgeType = &ft
	}
	if r.LastRunAt.Valid {
		t := r.LastRunAt.Time
		out.LastRunAt = &t
	}
	return out
}

func toReportSummary(r *db.WeeklyReport) ReportSummary {
	return ReportSummary{
		Id:          r.ID,
		RepoId:      r.RepoID,
		Year:        r.Year,
		Week:        r.Week,
		WeekStart:   openapitypes.Date{Time: r.WeekStart},
		WeekEnd:     openapitypes.Date{Time: r.WeekEnd},
		CommitCount: r.CommitCount,
		CreatedAt:   r.CreatedAt,
	}
}

func toReport(r *db.WeeklyReport) Report {
	out := Report{
		Id:          r.ID,
		RepoId:      r.RepoID,
		Year:        r.Year,
		Week:        r.Week,
		WeekStart:   openapitypes.Date{Time: r.WeekStart},
		WeekEnd:     openapitypes.Date{Time: r.WeekEnd},
		CommitCount: r.CommitCount,
		CreatedAt:   r.CreatedAt,
		UpdatedAt:   r.UpdatedAt,
		Summary:     nullStrPtr(r.Summary),
	}
	if !r.Metadata.Valid {
		return out
	}
	var meta service.ReportMetadata
	if err := json.Unmarshal([]byte(r.Metadata.String), &meta); err != nil {
		slog.Warn("failed to decode report metadata", "report_id", r.ID, "error", err)
		return out
	}
	if len(meta.Authors) > 0 {
		out.Authors = &meta.Authors
	}
	if len(meta.AuthorCounts) > 0 {
		out.AuthorCounts = &meta.AuthorCounts
	}
	if len(meta.Commits) > 0 {
		commits := make([]Commit, 0, len(meta.Commits))
		for _, c := range meta.Commits {
			d, err := parseDate(c.Date)
			if err != nil {
				slog.Warn("invalid commit date in report metadata", "report_id", r.ID, "sha", c.SHA, "date", c.Date, "error", err)
			}
			commits = append(commits, Commit{
				Sha:     c.SHA,
				Author:  c.Author,
				Date:    d,
				Message: c.Message,
			})
		}
		out.Commits = &commits
	}
	return out
}

func nullStrPtr(n sql.NullString) *string {
	if !n.Valid {
		return nil
	}
	s := n.String
	return &s
}

func parseDate(s string) (openapitypes.Date, error) {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return openapitypes.Date{}, err
	}
	return openapitypes.Date{Time: t}, nil
}
