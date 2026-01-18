package newsletter

import (
	"bytes"
	"html/template"
	"strings"

	"github.com/yuin/goldmark"
)

// RepoSection represents a section of the newsletter for a single repository
type RepoSection struct {
	RepoName    string
	Summary     string
	SummaryHTML template.HTML
	CommitRange string
	AnalyzedAt  string
}

// NewsletterData holds all data needed to render a newsletter
type NewsletterData struct {
	Sections      []RepoSection
	TotalRepos    int
	SubjectPrefix string
}

// Subject generates the email subject line
func (n *NewsletterData) Subject() string {
	if n.TotalRepos == 1 {
		return n.SubjectPrefix + " Activity update for " + n.Sections[0].RepoName
	}
	return n.SubjectPrefix + " Activity digest"
}

var htmlTemplate = template.Must(template.New("html").Parse(`<!DOCTYPE html>
<html>
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Activity Digest</title>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, sans-serif;
            line-height: 1.6;
            color: #333;
            max-width: 700px;
            margin: 0 auto;
            padding: 20px;
        }
        h1 {
            color: #2c3e50;
            border-bottom: 2px solid #3498db;
            padding-bottom: 10px;
        }
        h2 {
            color: #2980b9;
            margin-top: 30px;
        }
        .repo-section {
            background: #f8f9fa;
            border-left: 4px solid #3498db;
            padding: 15px 20px;
            margin: 20px 0;
        }
        .meta {
            color: #666;
            font-size: 0.9em;
            margin-bottom: 15px;
        }
        .summary {
            margin-top: 10px;
        }
        .summary h1, .summary h2, .summary h3 {
            margin-top: 15px;
            margin-bottom: 10px;
        }
        .summary ul, .summary ol {
            margin-left: 20px;
        }
        .footer {
            margin-top: 40px;
            padding-top: 20px;
            border-top: 1px solid #ddd;
            color: #666;
            font-size: 0.85em;
        }
    </style>
</head>
<body>
    <h1>Activity Digest</h1>
    {{range .Sections}}
    <div class="repo-section">
        <h2>{{.RepoName}}</h2>
        <div class="meta">
            Commits: {{.CommitRange}}<br>
            Analyzed: {{.AnalyzedAt}}
        </div>
        <div class="summary">
            {{.SummaryHTML}}
        </div>
    </div>
    {{end}}
    <div class="footer">
        <p>This email was sent by Activity - Git Repository Change Analyzer</p>
    </div>
</body>
</html>`))

var textTemplate = template.Must(template.New("text").Parse(`ACTIVITY DIGEST
===============

{{range .Sections}}
## {{.RepoName}}

Commits: {{.CommitRange}}
Analyzed: {{.AnalyzedAt}}

{{.Summary}}

---
{{end}}

This email was sent by Activity - Git Repository Change Analyzer
`))

// RenderHTML renders the newsletter as HTML
func RenderHTML(data *NewsletterData) (string, error) {
	var buf bytes.Buffer
	if err := htmlTemplate.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// RenderText renders the newsletter as plain text
func RenderText(data *NewsletterData) (string, error) {
	var buf bytes.Buffer
	if err := textTemplate.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// MarkdownToHTML converts markdown text to HTML
func MarkdownToHTML(markdown string) (template.HTML, error) {
	var buf bytes.Buffer
	if err := goldmark.Convert([]byte(markdown), &buf); err != nil {
		return "", err
	}
	return template.HTML(buf.String()), nil
}

// StripMarkdown attempts to convert markdown to plain text
// This is a simple approach that removes common markdown syntax
func StripMarkdown(markdown string) string {
	// Remove headers
	lines := strings.Split(markdown, "\n")
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") {
			// Remove # prefix from headers
			line = strings.TrimLeft(line, "# ")
			lines[i] = line
		}
	}

	result := strings.Join(lines, "\n")

	// Remove bold/italic markers
	result = strings.ReplaceAll(result, "**", "")
	result = strings.ReplaceAll(result, "__", "")
	result = strings.ReplaceAll(result, "*", "")
	result = strings.ReplaceAll(result, "_", "")

	// Remove inline code markers
	result = strings.ReplaceAll(result, "`", "")

	return result
}
