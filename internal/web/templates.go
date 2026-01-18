package web

import (
	"html/template"
)

// Templates holds all parsed templates
type Templates struct {
	base       *template.Template
	index      *template.Template
	repos      *template.Template
	repoDetail *template.Template
	report     *template.Template
}

// ParseTemplates parses all templates and returns a Templates struct
func ParseTemplates() (*Templates, error) {
	funcs := template.FuncMap{
		"safe": func(s string) template.HTML {
			return template.HTML(s)
		},
	}

	base := template.Must(template.New("base").Funcs(funcs).Parse(baseTemplate))

	index := template.Must(template.Must(base.Clone()).Parse(indexTemplate))
	repos := template.Must(template.Must(base.Clone()).Parse(reposTemplate))
	repoDetail := template.Must(template.Must(base.Clone()).Parse(repoDetailTemplate))
	report := template.Must(template.Must(base.Clone()).Parse(reportTemplate))

	return &Templates{
		base:       base,
		index:      index,
		repos:      repos,
		repoDetail: repoDetail,
		report:     report,
	}, nil
}

const baseTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{.Title}} // activity</title>
    <link rel="preconnect" href="https://fonts.googleapis.com">
    <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
    <link href="https://fonts.googleapis.com/css2?family=JetBrains+Mono:wght@400;500;600;700&display=swap" rel="stylesheet">
    <style>
        :root {
            --bg-primary: #0d1117;
            --bg-secondary: #161b22;
            --bg-tertiary: #21262d;
            --border: #30363d;
            --text-primary: #e6edf3;
            --text-secondary: #8b949e;
            --text-muted: #6e7681;
            --accent: #58a6ff;
            --accent-hover: #79c0ff;
            --success: #3fb950;
            --warning: #d29922;
            --error: #f85149;
            --commit-dot: #238636;
        }

        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }

        body {
            font-family: 'JetBrains Mono', monospace;
            background-color: var(--bg-primary);
            color: var(--text-primary);
            min-height: 100vh;
            font-size: 14px;
            line-height: 1.6;
        }

        a {
            color: var(--accent);
            text-decoration: none;
            transition: color 0.15s ease;
        }

        a:hover {
            color: var(--accent-hover);
        }

        /* Navigation */
        .nav {
            background: var(--bg-secondary);
            border-bottom: 1px solid var(--border);
            padding: 0 24px;
            position: sticky;
            top: 0;
            z-index: 100;
        }

        .nav-inner {
            max-width: 1200px;
            margin: 0 auto;
            display: flex;
            align-items: center;
            height: 56px;
            gap: 32px;
        }

        .nav-brand {
            display: flex;
            align-items: center;
            gap: 8px;
            color: var(--text-primary);
            font-weight: 600;
            font-size: 15px;
        }

        .nav-brand::before {
            content: ">";
            color: var(--success);
        }

        .nav-links {
            display: flex;
            gap: 4px;
        }

        .nav-link {
            padding: 8px 16px;
            border-radius: 6px;
            color: var(--text-secondary);
            font-size: 13px;
            transition: all 0.15s ease;
        }

        .nav-link:hover {
            color: var(--text-primary);
            background: var(--bg-tertiary);
        }

        .nav-link.active {
            color: var(--text-primary);
            background: var(--bg-tertiary);
        }

        /* Main content */
        .main {
            max-width: 1200px;
            margin: 0 auto;
            padding: 32px 24px;
        }

        /* Page header */
        .page-header {
            margin-bottom: 32px;
        }

        .page-title {
            font-size: 20px;
            font-weight: 600;
            color: var(--text-primary);
            margin-bottom: 4px;
        }

        .page-subtitle {
            font-size: 13px;
            color: var(--text-secondary);
        }

        /* Breadcrumb */
        .breadcrumb {
            display: flex;
            align-items: center;
            gap: 8px;
            font-size: 13px;
            color: var(--text-muted);
            margin-bottom: 16px;
        }

        .breadcrumb a {
            color: var(--text-secondary);
        }

        .breadcrumb a:hover {
            color: var(--accent);
        }

        .breadcrumb-sep {
            color: var(--text-muted);
        }

        /* Tables */
        .table-container {
            background: var(--bg-secondary);
            border: 1px solid var(--border);
            border-radius: 8px;
            overflow: hidden;
        }

        table {
            width: 100%;
            border-collapse: collapse;
        }

        th {
            text-align: left;
            padding: 12px 16px;
            background: var(--bg-tertiary);
            color: var(--text-secondary);
            font-size: 11px;
            font-weight: 500;
            text-transform: uppercase;
            letter-spacing: 0.5px;
            border-bottom: 1px solid var(--border);
        }

        td {
            padding: 12px 16px;
            border-bottom: 1px solid var(--border);
            font-size: 13px;
        }

        tr:last-child td {
            border-bottom: none;
        }

        tr:hover {
            background: rgba(88, 166, 255, 0.04);
        }

        .cell-primary {
            color: var(--text-primary);
            font-weight: 500;
        }

        .cell-secondary {
            color: var(--text-secondary);
        }

        .cell-muted {
            color: var(--text-muted);
        }

        .cell-truncate {
            max-width: 300px;
            overflow: hidden;
            text-overflow: ellipsis;
            white-space: nowrap;
        }

        /* Cards */
        .card {
            background: var(--bg-secondary);
            border: 1px solid var(--border);
            border-radius: 8px;
            padding: 20px;
            transition: border-color 0.15s ease;
        }

        .card:hover {
            border-color: var(--text-muted);
        }

        .card-grid {
            display: grid;
            gap: 16px;
            grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
        }

        .card-header {
            display: flex;
            align-items: center;
            justify-content: space-between;
            margin-bottom: 12px;
        }

        .card-title {
            font-size: 15px;
            font-weight: 600;
            color: var(--text-primary);
        }

        .card-meta {
            font-size: 12px;
            color: var(--text-muted);
        }

        /* Badges */
        .badge {
            display: inline-flex;
            align-items: center;
            gap: 4px;
            padding: 2px 8px;
            border-radius: 4px;
            font-size: 11px;
            font-weight: 500;
        }

        .badge-active {
            background: rgba(63, 185, 80, 0.15);
            color: var(--success);
        }

        .badge-inactive {
            background: rgba(110, 118, 129, 0.15);
            color: var(--text-muted);
        }

        .badge-agent {
            background: rgba(88, 166, 255, 0.15);
            color: var(--accent);
        }

        /* Year filter pills */
        .filter-bar {
            display: flex;
            align-items: center;
            gap: 8px;
            margin-bottom: 24px;
            flex-wrap: wrap;
        }

        .filter-label {
            font-size: 12px;
            color: var(--text-muted);
        }

        .filter-pill {
            padding: 6px 12px;
            border-radius: 6px;
            font-size: 12px;
            background: var(--bg-tertiary);
            color: var(--text-secondary);
            border: 1px solid transparent;
            transition: all 0.15s ease;
        }

        .filter-pill:hover {
            color: var(--text-primary);
            border-color: var(--border);
        }

        .filter-pill.active {
            background: var(--accent);
            color: var(--bg-primary);
        }

        /* Report detail layout */
        .report-layout {
            display: grid;
            gap: 24px;
            grid-template-columns: 280px 1fr;
        }

        @media (max-width: 900px) {
            .report-layout {
                grid-template-columns: 1fr;
            }
        }

        .report-sidebar {
            position: sticky;
            top: 80px;
            align-self: start;
        }

        .report-meta dt {
            font-size: 11px;
            color: var(--text-muted);
            text-transform: uppercase;
            letter-spacing: 0.5px;
            margin-bottom: 4px;
        }

        .report-meta dd {
            font-size: 13px;
            color: var(--text-primary);
            margin-bottom: 16px;
        }

        .report-meta dd:last-child {
            margin-bottom: 0;
        }

        /* Prose content */
        .prose {
            color: var(--text-secondary);
            line-height: 1.7;
        }

        .prose h1 {
            font-size: 18px;
            font-weight: 600;
            color: var(--text-primary);
            margin: 24px 0 12px 0;
            padding-bottom: 8px;
            border-bottom: 1px solid var(--border);
        }

        .prose h1:first-child {
            margin-top: 0;
        }

        .prose h2 {
            font-size: 15px;
            font-weight: 600;
            color: var(--text-primary);
            margin: 20px 0 10px 0;
        }

        .prose h3 {
            font-size: 14px;
            font-weight: 600;
            color: var(--text-primary);
            margin: 16px 0 8px 0;
        }

        .prose p {
            margin-bottom: 12px;
        }

        .prose ul, .prose ol {
            margin: 12px 0;
            padding-left: 24px;
        }

        .prose li {
            margin-bottom: 6px;
        }

        .prose li::marker {
            color: var(--text-muted);
        }

        .prose code {
            background: var(--bg-tertiary);
            padding: 2px 6px;
            border-radius: 4px;
            font-size: 12px;
            color: var(--accent);
        }

        .prose pre {
            background: var(--bg-primary);
            border: 1px solid var(--border);
            border-radius: 6px;
            padding: 16px;
            overflow-x: auto;
            margin: 16px 0;
        }

        .prose pre code {
            background: none;
            padding: 0;
            font-size: 12px;
            color: var(--text-secondary);
        }

        .prose strong {
            font-weight: 600;
            color: var(--text-primary);
        }

        .prose a {
            color: var(--accent);
            text-decoration: underline;
            text-underline-offset: 2px;
        }

        /* Commit count indicator */
        .commit-count {
            display: inline-flex;
            align-items: center;
            gap: 6px;
        }

        .commit-count::before {
            content: "";
            width: 8px;
            height: 8px;
            background: var(--commit-dot);
            border-radius: 50%;
        }

        /* Empty state */
        .empty-state {
            text-align: center;
            padding: 64px 24px;
            background: var(--bg-secondary);
            border: 1px dashed var(--border);
            border-radius: 8px;
        }

        .empty-state-icon {
            font-size: 32px;
            color: var(--text-muted);
            margin-bottom: 16px;
        }

        .empty-state-title {
            font-size: 15px;
            font-weight: 500;
            color: var(--text-primary);
            margin-bottom: 4px;
        }

        .empty-state-desc {
            font-size: 13px;
            color: var(--text-secondary);
        }

        /* Error state */
        .error-banner {
            background: rgba(248, 81, 73, 0.1);
            border: 1px solid rgba(248, 81, 73, 0.4);
            border-radius: 6px;
            padding: 12px 16px;
            margin-bottom: 24px;
            color: var(--error);
            font-size: 13px;
        }

        /* Footer */
        .footer {
            border-top: 1px solid var(--border);
            padding: 24px;
            margin-top: 64px;
        }

        .footer-inner {
            max-width: 1200px;
            margin: 0 auto;
            text-align: center;
            font-size: 12px;
            color: var(--text-muted);
        }

        /* URL display */
        .url-display {
            color: var(--text-muted);
            font-size: 12px;
            overflow: hidden;
            text-overflow: ellipsis;
            white-space: nowrap;
        }

        /* Stats row */
        .stats-row {
            display: flex;
            align-items: center;
            justify-content: space-between;
            font-size: 12px;
            color: var(--text-muted);
            margin-top: 12px;
            padding-top: 12px;
            border-top: 1px solid var(--border);
        }
    </style>
</head>
<body>
    <nav class="nav">
        <div class="nav-inner">
            <a href="/" class="nav-brand">activity</a>
            <div class="nav-links">
                <a href="/" class="nav-link {{if eq .ActiveNav "dashboard"}}active{{end}}">dashboard</a>
                <a href="/repos" class="nav-link {{if eq .ActiveNav "repos"}}active{{end}}">repos</a>
            </div>
        </div>
    </nav>

    <main class="main">
        {{if .Error}}
        <div class="error-banner">
            {{.Error}}
        </div>
        {{end}}
        {{template "content" .}}
    </main>

    <footer class="footer">
        <div class="footer-inner">
            activity // git repository change analyzer
        </div>
    </footer>
</body>
</html>`

const indexTemplate = `{{define "content"}}
<div class="page-header">
    <h1 class="page-title">Recent Reports</h1>
    <p class="page-subtitle">latest weekly activity summaries across all repositories</p>
</div>

{{with .Content}}
{{if .Reports}}
<div class="table-container">
    <table>
        <thead>
            <tr>
                <th>Repository</th>
                <th>Week</th>
                <th>Period</th>
                <th>Commits</th>
                <th>Preview</th>
            </tr>
        </thead>
        <tbody>
            {{range .Reports}}
            <tr>
                <td><a href="/repos/{{.RepoName}}" class="cell-primary">{{.RepoName}}</a></td>
                <td><a href="/reports/{{.ID}}">{{.WeekLabel}}</a></td>
                <td class="cell-secondary">{{.WeekStart}} - {{.WeekEnd}}</td>
                <td class="cell-secondary"><span class="commit-count">{{.CommitCount}}</span></td>
                <td class="cell-muted cell-truncate">{{.Preview}}</td>
            </tr>
            {{end}}
        </tbody>
    </table>
</div>
{{else}}
<div class="empty-state">
    <div class="empty-state-icon">[ ]</div>
    <div class="empty-state-title">No reports generated</div>
    <div class="empty-state-desc">Run 'activity report generate' to create weekly reports</div>
</div>
{{end}}
{{end}}
{{end}}`

const reposTemplate = `{{define "content"}}
<div class="page-header">
    <h1 class="page-title">Repositories</h1>
    <p class="page-subtitle">all tracked repositories</p>
</div>

{{with .Content}}
{{if .Repos}}
<div class="card-grid">
    {{range .Repos}}
    <div class="card">
        <div class="card-header">
            <a href="/repos/{{.Name}}" class="card-title">{{.Name}}</a>
            {{if .Active}}
            <span class="badge badge-active">active</span>
            {{else}}
            <span class="badge badge-inactive">inactive</span>
            {{end}}
        </div>
        <div class="url-display" title="{{.URL}}">{{.URL}}</div>
        <div class="stats-row">
            <span>{{.ReportCount}} reports</span>
            <span>{{.LastReport}}</span>
        </div>
    </div>
    {{end}}
</div>
{{else}}
<div class="empty-state">
    <div class="empty-state-icon">[ ]</div>
    <div class="empty-state-title">No repositories tracked</div>
    <div class="empty-state-desc">Run 'activity repo add' to start tracking repositories</div>
</div>
{{end}}
{{end}}
{{end}}`

const repoDetailTemplate = `{{define "content"}}
{{with .Content}}
<div class="breadcrumb">
    <a href="/repos">repos</a>
    <span class="breadcrumb-sep">/</span>
    <span>{{.Repo.Name}}</span>
</div>

<div class="page-header">
    <div style="display: flex; align-items: center; gap: 12px;">
        <h1 class="page-title">{{.Repo.Name}}</h1>
        {{if .Repo.Active}}
        <span class="badge badge-active">active</span>
        {{else}}
        <span class="badge badge-inactive">inactive</span>
        {{end}}
    </div>
    <p class="page-subtitle">{{.Repo.URL}}</p>
</div>

{{if .Years}}
<div class="filter-bar">
    <span class="filter-label">filter by year:</span>
    <a href="?year=" class="filter-pill {{if eq .CurrentYear 0}}active{{end}}">all</a>
    {{range .Years}}
    <a href="?year={{.}}" class="filter-pill {{if eq . $.Content.CurrentYear}}active{{end}}">{{.}}</a>
    {{end}}
</div>
{{end}}

{{if .Reports}}
<div class="table-container">
    <table>
        <thead>
            <tr>
                <th>Week</th>
                <th>Period</th>
                <th>Commits</th>
                <th>Generated</th>
                <th>Preview</th>
            </tr>
        </thead>
        <tbody>
            {{range .Reports}}
            <tr>
                <td><a href="/reports/{{.ID}}" class="cell-primary">{{.WeekLabel}}</a></td>
                <td class="cell-secondary">{{.WeekStart}} - {{.WeekEnd}}</td>
                <td class="cell-secondary"><span class="commit-count">{{.CommitCount}}</span></td>
                <td class="cell-muted">{{.CreatedAt}}</td>
                <td class="cell-muted cell-truncate">{{.Preview}}</td>
            </tr>
            {{end}}
        </tbody>
    </table>
</div>
{{else}}
<div class="empty-state">
    <div class="empty-state-icon">[ ]</div>
    <div class="empty-state-title">No reports for this repository</div>
    <div class="empty-state-desc">Run 'activity report generate {{.Repo.Name}}' to create reports</div>
</div>
{{end}}
{{end}}
{{end}}`

const reportTemplate = `{{define "content"}}
{{with .Content}}
<div class="breadcrumb">
    <a href="/repos">repos</a>
    <span class="breadcrumb-sep">/</span>
    <a href="/repos/{{.Report.RepoName}}">{{.Report.RepoName}}</a>
    <span class="breadcrumb-sep">/</span>
    <span>{{.Report.WeekLabel}}</span>
</div>

<div class="page-header">
    <h1 class="page-title">{{.Report.WeekLabel}}</h1>
    <p class="page-subtitle">weekly report for {{.Report.RepoName}}</p>
</div>

<div class="report-layout">
    <aside class="report-sidebar">
        <div class="card">
            <dl class="report-meta">
                <dt>Repository</dt>
                <dd><a href="/repos/{{.Report.RepoName}}">{{.Report.RepoName}}</a></dd>

                <dt>Week</dt>
                <dd>{{.Report.WeekLabel}}</dd>

                <dt>Period</dt>
                <dd>{{.Report.WeekStart}} - {{.Report.WeekEnd}}</dd>

                <dt>Commits</dt>
                <dd><span class="commit-count">{{.Report.CommitCount}}</span></dd>

                {{if .Report.Authors}}
                <dt>Authors</dt>
                <dd>{{range $i, $a := .Report.Authors}}{{if $i}}, {{end}}{{$a}}{{end}}</dd>
                {{end}}

                <dt>Analysis</dt>
                <dd>
                    {{if .Report.AgentMode}}
                    <span class="badge badge-agent">agent</span>
                    {{else}}
                    <span class="badge badge-inactive">simple</span>
                    {{end}}
                </dd>

                <dt>Generated</dt>
                <dd>{{.Report.CreatedAt}}</dd>
            </dl>
        </div>
    </aside>

    <article class="card">
        {{if .Report.SummaryHTML}}
        <div class="prose">
            {{.Report.SummaryHTML}}
        </div>
        {{else}}
        <div class="empty-state" style="border: none; padding: 32px;">
            <div class="empty-state-title">No summary available</div>
            <div class="empty-state-desc">This report has no generated summary</div>
        </div>
        {{end}}
    </article>
</div>
{{end}}
{{end}}`
