# Activity - Git Repository Analysis Tool

## Statement of Intent

This tool analyzes git commit history using AI to generate human-readable activity summaries. It tracks multiple repositories, analyzes commit ranges incrementally, and uses intelligent agents to selectively fetch code diffs only when commit messages are unclear. The implementation emphasizes cost control through configurable limits and phased AI integration (simple LLM → agent-based with tools).

## Architecture

Activity is a **pure web application** with admin functionality. Public routes (dashboard, repos, reports) are read-only. Admin operations (repository management, newsletters, analysis triggers, user management) require authentication via an auth proxy that provides user email in a configurable header.

**Auth Model:** Auth proxy provides user email via configurable header (default: `oidc-email`). Admins are listed in SQLite `admins` table.

**Dev Mode:** When `dev_mode: true` in config, auth is bypassed and `dev_user` email is used (default: `dev@localhost`), treated as admin.

## Code Overview

### `main.go`

Main entry point. Uses standard library `flag` for CLI arguments (port, host, config, data-dir, debug). Initializes database, services, and starts the web server.

### `internal/config`

Configuration management with YAML support. Defines `Config`, `LLMConfig`, `WebConfig`, `NewsletterConfig`, and `GitHubConfig` structs. WebConfig handles auth settings (`auth_header`, `seed_admin`, `dev_mode`, `dev_user`).

### `internal/db`

SQLite database layer with migrations (version 7). Tables: `repositories`, `activity_runs`, `weekly_reports`, newsletter tables (`subscribers`, `subscriptions`, `newsletter_sends`), and `admins`. Includes CRUD operations for all models.

### `internal/service`

Business logic layer extracted from former CLI commands:
- `RepoService`: Add, Remove, Activate, Deactivate, SetURL, Update, UpdateAll
- `ReportService`: GenerateForWeek, GenerateSince, GenerateLastWeek, ListReports
- `NewsletterService`: AddSubscriber, RemoveSubscriber, Subscribe, Unsubscribe, Send
- `AdminService`: Add, Remove, IsAdmin, List, SeedIfNeeded, EnsureDevAdmin

### `internal/web`

HTTP server with public and admin routes:
- **Public**: `/` (dashboard), `/repos`, `/repos/{name}`, `/reports/{id}`
- **Admin**: `/admin`, `/admin/repos`, `/admin/subscribers`, `/admin/actions`, `/admin/admins`

Auth middleware extracts user from header (or uses dev user in dev mode) and checks admin status. `RequireAdmin` middleware protects admin routes.

### `internal/git`

Git operations wrapper using `exec.Command`. Provides functions for cloning, pulling, retrieving commit ranges, fetching diffs. Includes ISO week utilities for weekly report generation.

### `internal/llm`

LLM client abstraction supporting Gemini API (Phase 2 simple and Phase 3 agent modes).

### `internal/analyzer`

Core analysis logic with cost tracking. Agent mode uses ADK tools (`GetCommitDiffTool`, `GetFullCommitMessageTool`) for selective diff fetching.

## Running

```bash
# Development mode (no auth required)
./activity --data-dir ./data --port 8080

# With config file
./activity --config /path/to/config.yaml

# Production (behind auth proxy)
./activity --data-dir /var/lib/activity --port 8080 --host 0.0.0.0
```

## Configuration

```yaml
data_dir: /var/lib/activity
web:
  auth_header: oidc-email    # Header containing user email
  seed_admin: admin@example.com  # First admin on empty DB
  dev_mode: false            # Set true for local development
  dev_user: dev@localhost    # Email used in dev mode
llm:
  use_agent: true            # Agent mode (default)
  max_diff_fetches: 5        # Cost control
newsletter:
  enabled: true
  sendgrid_api_key_env: SENDGRID_API_KEY
```

## Phase Architecture

**Phase 1 (Complete)**: Foundation - git operations, database, web structure
**Phase 2 (Complete)**: Simple AI - commit metadata sent to LLM (~$0.0005/analysis)
**Phase 3 (Complete, Default)**: Intelligent agents - ADK-based with selective diff fetching (~$0.0005-0.01/analysis, hard-capped)
