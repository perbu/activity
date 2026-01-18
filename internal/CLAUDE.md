# Internal Packages

## analyzer

Core analysis engine that processes git commits using AI. Supports two modes: a simple LLM mode that sends commit
metadata directly to Gemini, and an agent-based mode using Google's ADK framework that can intelligently fetch diffs
when commit messages are unclear. Includes cost tracking to limit API usage and provides tools (`GetCommitDiffTool`,
`GetFullCommitMessageTool`) for the agent to selectively retrieve additional context. The router decides which mode to
use based on configuration.

## cli

Command-line interface built with Kong using struct-tag based command definitions. Defines all CLI commands including
`repo` (add/remove/list repositories), `analyze` (run AI analysis on commits), `update` (pull and optionally analyze),
`report` (generate/show weekly reports), `newsletter` (subscriber management), and `serve` (start web server). Each
command struct has a `Run(ctx *Context) error` method that implements the command logic.

## config

Configuration management with YAML file support. Defines `Config`, `LLMConfig`, `NewsletterConfig`, and `GitHubConfig`
structs with sensible defaults. Handles API key resolution from both direct config values and environment variables.
Includes cost control parameters for agent mode like `max_diff_fetches`, `max_diff_size_kb`, and `max_total_tokens`.

## db

SQLite database layer using modernc.org/sqlite (pure Go). Manages schema migrations and provides CRUD operations for all
models: repositories (tracked git repos), activity_runs (analysis results), weekly_reports (week-indexed summaries), and
newsletter tables (subscribers, sent_newsletters). The migration system tracks schema versions in a `migrations` table.

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

## web

HTTP server for browsing weekly reports via a web interface. Uses Go's standard library `http.ServeMux` with embedded
HTML templates. Provides routes for listing repositories, viewing per-repo reports, and displaying individual report
content. Templates are parsed at startup using Go's `html/template` package.
