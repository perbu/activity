# Activity - Git Repository Analysis Tool

## Statement of Intent

This tool analyzes git commit history using AI to generate human-readable activity summaries. It tracks multiple repositories, analyzes commit ranges incrementally, and uses intelligent agents to selectively fetch code diffs only when commit messages are unclear. The implementation emphasizes cost control through configurable limits and phased AI integration (simple LLM → agent-based with tools).

## Code Overview (~2000 LOC)

### `main.go`

Main entry point at the root for simple `go install`. Uses kong for CLI parsing with struct-tag based command definitions. Wires together configuration, database, git operations, LLM client, and analyzer components. Handles initialization of the data directory and database schema.

### `internal/config`

Configuration management with YAML support. Defines `Config` and `LLMConfig` structs with defaults. Supports both Phase 2 (simple LLM) and Phase 3 (agent-based) settings including cost control parameters (`max_diff_fetches`, `max_diff_size_kb`, `max_total_tokens`). Handles path expansion and environment variable resolution.

### `internal/db`

SQLite database layer with migrations. Manages tables: `repositories` (tracked repos), `activity_runs` (analysis results), `weekly_reports` (week-indexed summaries), and newsletter tables. Includes CRUD operations for all models. Migration version 4 adds `weekly_reports` table for week-indexed analysis storage.

### `internal/git`

Git operations wrapper using `exec.Command`. Provides functions for cloning, pulling, retrieving commit ranges, fetching diffs, and getting detailed commit info. Uses record separator delimiters to safely parse git output. Includes ISO week utilities (`ISOWeekBounds`, `GetCommitsForWeek`, `ParseISOWeek`, `WeeksInRange`) for weekly report generation.

### `internal/llm`

LLM client abstraction supporting both genai (Phase 2) and ADK (Phase 3). Creates Gemini API clients and provides `GenerateText` for simple prompts and `GetGeminiModel` for agent-based analysis. Handles API key management and model configuration.

### `internal/analyzer`

Core analysis logic with three modes: Phase 2 (simple LLM), Phase 3 (agent with tools), and routing between them. Contains cost tracker for limiting diff fetches, two ADK tools (`GetCommitDiffTool`, `GetFullCommitMessageTool`), and agent orchestration with in-memory sessions. Builds prompts from commit metadata and stores results with cost tracking metadata.

### `internal/cli`

CLI command implementations using kong's struct-tag based approach. Command structs are defined in `commands.go` with kong tags for flags, args, and help text. Each command struct has a `Run(ctx *Context) error` method. Commands include: `repo` (add/remove/list repos), `analyze` (analyze commits), `update` (pull and analyze), `report` (generate/show/list weekly reports), and `newsletter` (subscriber management). Flags can appear in any position relative to commands.

## Phase Architecture

**Phase 1 (Complete)**: Foundation - git operations, database, CLI structure
**Phase 2 (Complete)**: Simple AI - commit metadata sent to LLM (~$0.0005/analysis)
**Phase 3 (Complete, Default)**: Intelligent agents - ADK-based with selective diff fetching (~$0.0005-0.01/analysis, hard-capped)

Agent mode is default. For well-documented repos with clear commit messages, the agent skips diff fetching and costs match Phase 2. Set `use_agent: false` in config to use Phase 2 simple mode.
