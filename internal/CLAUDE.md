# Internal Packages

## analyzer

Core analysis engine that processes git commits using AI. Supports two modes: a simple LLM mode that sends commit
metadata directly to Gemini, and an agent-based mode using Google's ADK framework that can intelligently fetch diffs
when commit messages are unclear. Includes cost tracking to limit API usage and provides tools (`GetCommitDiffTool`,
`GetFullCommitMessageTool`) for the agent to selectively retrieve additional context. The router decides which mode to
use based on configuration.

## config

Configuration management with YAML file support. Defines `Config`, `LLMConfig`, `WebConfig`, `NewsletterConfig`, and
`GitHubConfig` structs with sensible defaults. Handles API key resolution from both direct config values and environment
variables. `WebConfig` handles auth proxy settings (`auth_header`, `seed_admin`, `dev_mode`, `dev_user`).

## db

SQLite database layer using modernc.org/sqlite (pure Go). Manages schema migrations (version 7) and provides CRUD
operations for all models: repositories, activity_runs, weekly_reports, newsletter tables (subscribers, subscriptions,
newsletter_sends), and admins. The migration system tracks schema versions in a `migrations` table.

## email

SendGrid API client wrapper for sending emails. Provides a simple `Client` type with a `Send` method that takes email
content (HTML and text) and handles the SendGrid API interaction. Returns message IDs for tracking and handles error
responses from the API.

## git

Git operations wrapper using `os/exec` to shell out to the git CLI. Provides functions for cloning, pulling, getting
commit ranges, fetching diffs, and retrieving detailed commit information. Uses record separator delimiters to safely
parse git log output. Includes ISO week utilities (`ISOWeekBounds`, `GetCommitsForWeek`, `ParseISOWeek`, `WeeksInRange`)
for weekly report generation.

## github

GitHub App authentication using installation tokens. The `TokenProvider` type manages token lifecycle with automatic
caching and refresh (tokens cached for ~55 minutes, GitHub tokens valid for 1 hour). Provides `GetToken()` for
retrieving valid tokens and `GetAuthenticatedURL()` for constructing git URLs with embedded tokens for private
repository access.

## llm

LLM client abstraction for Google's Gemini API. Creates clients using the genai SDK and provides `GenerateText` for
simple prompts and `GetGeminiModel` for agent-based analysis via ADK. Handles API key retrieval from config or
environment variables and manages the underlying client lifecycle.

## newsletter

Newsletter composition and delivery system. The `Composer` builds email content from activity runs by gathering
repository summaries and formatting them using HTML templates. The `Sender` coordinates delivery via the email package,
tracking which newsletters have been sent to which subscribers in the database.

## service

Business logic layer extracted from former CLI commands. Provides reusable services for web handlers:
- `RepoService`: Repository management (Add, Remove, Activate, Deactivate, SetURL, Update, UpdateAll)
- `ReportService`: Report generation (GenerateForWeek, GenerateSince, GenerateLastWeek, ListReports)
- `NewsletterService`: Subscriber management (AddSubscriber, RemoveSubscriber, Subscribe, Unsubscribe, Send)
- `AdminService`: Admin user management (Add, Remove, IsAdmin, List, SeedIfNeeded, EnsureDevAdmin)

## web

HTTP server for the Activity web application. Uses Go's standard library `http.ServeMux` with embedded HTML templates.

**Public routes** (read-only):
- `/` - Dashboard with recent reports
- `/repos` - Repository list
- `/repos/{name}` - Per-repo reports
- `/reports/{id}` - Individual report view

**Admin routes** (protected by auth middleware):
- `/admin` - Admin dashboard
- `/admin/repos` - Repository management (add, remove, activate/deactivate)
- `/admin/subscribers` - Newsletter subscriber management
- `/admin/actions` - Manual triggers (update repos, generate reports, send newsletters)
- `/admin/admins` - Admin user management

Auth middleware extracts user email from configurable header (default: `oidc-email`) and checks admin status in database.
In dev mode, auth is bypassed and a configurable dev user is used.
