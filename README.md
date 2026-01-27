# Activity

AI-powered git commit analyzer that generates human-readable summaries of repository activity. Uses intelligent agents to selectively fetch code diffs only when commit messages are unclear.

## Features

- **Intelligent Analysis**: Agent-based analyzer decides when to fetch diffs vs. using commit messages
- **Weekly Reports**: Generate week-indexed summaries with backfill support
- **Cost Controls**: Hard limits on diff fetching, diff size, and total tokens
- **Incremental Tracking**: Analyzes only new commits since last run
- **Multi-Repository**: Track and analyze multiple repositories
- **Cost Efficient**: ~$0.0005-0.01 per analysis depending on commit message quality

## Requirements

- Go 1.25.3+
- Google Gemini API key
- Git repositories to analyze

## Installation

```bash
go install github.com/perbu/activity@latest
```

Or build from source:

```bash
git clone https://github.com/perbu/activity
cd activity
go build .
```

## Docker

Docker images are built and pushed to GitHub Container Registry on tagged releases.

```bash
# Pull the latest image
docker pull ghcr.io/perbu/activity:latest

# Run with environment variables
docker run -v /path/to/data:/data \
  -e GOOGLE_API_KEY=your-api-key \
  ghcr.io/perbu/activity:latest \
  --data-dir /data \
  repo list

# For private GitHub repositories, include GitHub App credentials
docker run -v /path/to/data:/data \
  -e GOOGLE_API_KEY=your-api-key \
  -e GITHUB_APP_ID=123456 \
  -e GITHUB_INSTALLATION_ID=789012 \
  -e GITHUB_APP_PRIVATE_KEY="$(cat path/to/key.pem)" \
  ghcr.io/perbu/activity:latest \
  --data-dir /data \
  analyze myrepo
```

Available tags:
- `latest` - Latest release
- `1.0.0` - Specific version (from v1.0.0 tag)
- `1.0` - Major.minor version

## Quick Start

1. Set your API key:
```bash
export GOOGLE_API_KEY=your-api-key
```

2. Add a repository:
```bash
activity --data-dir ~/.local/share/activity repo add myproject https://github.com/user/repo
```

3. Analyze commits:
```bash
activity --data-dir ~/.local/share/activity analyze myproject --since '1 week ago'
```

4. Or update and analyze new commits:
```bash
activity --data-dir ~/.local/share/activity update myproject --analyze
```

Note: Flags can appear in any position, so these are equivalent:
```bash
activity --data-dir ~/.local/share/activity repo add myproject https://github.com/user/repo
activity repo add myproject https://github.com/user/repo --data-dir ~/.local/share/activity
```

## Configuration

Create `~/.config/activity/config.yaml`:

```yaml
data_dir: ~/.local/share/activity

llm:
  provider: gemini
  model: gemini-3.0-flash
  api_key_env: GOOGLE_API_KEY

  # Basic limits
  max_commits: 50
  max_message_length: 1000

  # Agent mode (default) - intelligent diff fetching
  use_agent: true        # Set to false for Phase 2 simple mode
  max_diff_fetches: 5    # Max diffs per analysis
  max_diff_size_kb: 10   # Max size per diff
  max_total_tokens: 100000  # ~$0.01 cost limit
  enable_tool_logs: true
```

See `config_example.yaml` for all options including custom prompts.

## How It Works

### Agent Mode (Default)

The intelligent agent:
1. Reviews all commit messages first
2. For **clear messages** (e.g., "Fix null pointer in user auth"): uses message only
3. For **vague messages** (e.g., "fix", "update"): fetches code diff
4. Respects hard limits to prevent cost overruns

**Cost**: ~$0.0005 for well-documented repos, up to ~$0.01 for poorly-documented repos (hard-capped)

### Phase 2 Mode (Fallback)

Simple mode sends only commit metadata (SHA, author, date, message) to LLM.

**Cost**: ~$0.0005 per analysis

Enable by setting `use_agent: false` in config.

## Commands

### Repository Management

```bash
# Add repository
activity repo add <name> <url> [--branch main]

# List repositories
activity list
activity repo list  # alias

# Remove repository
activity repo remove <name> [--keep-files]

# Show repository info
activity repo info <name>

# Activate/deactivate repository
activity repo activate <name>
activity repo deactivate <name>
```

### Analysis

```bash
# Analyze commits since date
activity analyze <name> --since '1 week ago'
activity analyze <name> --since 2024-01-01 --until 2024-01-31

# Analyze last N commits
activity analyze <name> -n 10

# Update repository and optionally analyze new commits
activity update <name>
activity update <name> --analyze
activity update --all  # Update all active repositories
```

### Weekly Reports

Generate week-indexed summaries for historical queries and web UI integration.

```bash
# Generate report for specific week
activity report generate <name> --week=2026-W03

# Backfill all weeks since a date
activity report generate <name> --since=2025-12-01

# Regenerate existing reports
activity report generate <name> --since=2025-12-01 --force

# Show latest report
activity report show <name> --latest

# Show report for specific week
activity report show <name> --week=2026-W03

# List all reports for a repository
activity report list <name>

# List all reports (all repos), filtered by year
activity report list --all --year=2026
```

### Prompts

```bash
# Show current LLM prompts (custom or default)
activity show-prompts

# Show default prompts even if custom ones are configured
activity show-prompts --defaults
```

## Cost Controls

The agent mode includes multiple safeguards:

- **max_diff_fetches**: Limits number of diffs per analysis (default: 5)
- **max_diff_size_kb**: Rejects diffs larger than limit (default: 10KB)
- **max_total_tokens**: Hard cap on total tokens (default: 100K ≈ $0.01)
- **Smart prompting**: Agent instructed to use diffs sparingly

## Database

All data stored in SQLite database at `<data_dir>/activity.db`. Migrations are managed by [goose](https://github.com/pressly/goose) and run automatically on startup.

Tables:
- `repositories`: Tracked repos with metadata
- `activity_runs`: Analysis results with summaries and cost tracking
- `weekly_reports`: Week-indexed summaries keyed by (repo, year, week)
- `subscribers`, `subscriptions`, `newsletter_sends`: Newsletter feature tables
- `admins`: Admin users for web authentication
- `goose_db_version`: Migration version tracking (managed by goose)

Query examples:
```sql
# View latest analysis run
sqlite3 ~/.local/share/activity/activity.db \
  "SELECT agent_mode, tool_usage_stats FROM activity_runs ORDER BY id DESC LIMIT 1;"

# View weekly reports
sqlite3 ~/.local/share/activity/activity.db \
  "SELECT year, week, commit_count, created_at FROM weekly_reports ORDER BY year DESC, week DESC;"

# Check migration version
sqlite3 ~/.local/share/activity/activity.db \
  "SELECT version_id FROM goose_db_version ORDER BY id DESC LIMIT 1;"
```

## Development

See `CLAUDE.md` for architecture overview and package descriptions.

### Project Structure

```
main.go               - CLI entry point (uses kong for parsing)
internal/
  analyzer/           - Analysis logic (Phase 2 + Phase 3)
  cli/                - Command structs and Run methods (kong-based)
  config/             - Configuration management
  db/                 - Database layer
    migrations/       - Goose SQL migrations (embedded)
  email/              - Email client for newsletters
  git/                - Git operations
  llm/                - LLM client abstraction
  newsletter/         - Newsletter composition and sending
```

## License

BSD 2-Clause License - see LICENSE file for details
