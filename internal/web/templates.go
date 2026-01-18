package web

import (
	"embed"
	"html/template"
	"io/fs"
)

//go:embed templates/*.html
var templateFS embed.FS

//go:embed static/*
var staticFS embed.FS

// Templates holds all parsed templates
type Templates struct {
	index      *template.Template
	repos      *template.Template
	repoDetail *template.Template
	report     *template.Template
}

// StaticFS returns the embedded static files filesystem
func StaticFS() fs.FS {
	sub, _ := fs.Sub(staticFS, "static")
	return sub
}

// ParseTemplates parses all templates and returns a Templates struct
func ParseTemplates() (*Templates, error) {
	funcs := template.FuncMap{
		"safe": func(s string) template.HTML {
			return template.HTML(s)
		},
	}

	// Parse base template
	base, err := template.New("base.html").Funcs(funcs).ParseFS(templateFS, "templates/base.html")
	if err != nil {
		return nil, err
	}

	// Parse each page template by cloning base and adding the page
	index, err := template.Must(base.Clone()).ParseFS(templateFS, "templates/index.html")
	if err != nil {
		return nil, err
	}

	repos, err := template.Must(base.Clone()).ParseFS(templateFS, "templates/repos.html")
	if err != nil {
		return nil, err
	}

	repoDetail, err := template.Must(base.Clone()).ParseFS(templateFS, "templates/repo_detail.html")
	if err != nil {
		return nil, err
	}

	report, err := template.Must(base.Clone()).ParseFS(templateFS, "templates/report.html")
	if err != nil {
		return nil, err
	}

	return &Templates{
		index:      index,
		repos:      repos,
		repoDetail: repoDetail,
		report:     report,
	}, nil
}
