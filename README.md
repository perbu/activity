# Activity

AI-powered git commit analyzer that generates human-readable summaries of repository activity. Uses intelligent agents to selectively fetch code diffs only when commit messages are unclear.

## Features

- **Intelligent Analysis**: Agent-based analyzer decides when to fetch diffs vs. using commit messages
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
go install github.com/perbu/activity/cmd/activity@latest
```

Or build from source:

```bash
git clone https://github.com/perbu/activity
cd activity
go build -o activity ./cmd/activity
```

## Quick Start

1. Set your API key:
```bash
export GOOGLE_API_KEY=your-api-key
```

2. Initialize and add a repository:
```bash
activity --data-dir ~/.local/share/activity init
activity --data-dir ~/.local/share/activity add myproject https://github.com/user/repo
```

3. Analyze commits:
```bash
activity --data-dir ~/.local/share/activity update myproject
```

4. View summary:
```bash
activity --data-dir ~/.local/share/activity show myproject
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
activity add <name> <url> [--branch main]

# List repositories
activity list

# Remove repository
activity remove <name>
```

### Analysis

```bash
# Analyze new commits
activity update <name>

# Show latest summary
activity show <name>

# Force re-analyze from specific commit
activity update <name> --from-sha abc123
```

### Configuration

```bash
# Initialize data directory
activity init

# Show current configuration
activity config
```

## Cost Controls

The agent mode includes multiple safeguards:

- **max_diff_fetches**: Limits number of diffs per analysis (default: 5)
- **max_diff_size_kb**: Rejects diffs larger than limit (default: 10KB)
- **max_total_tokens**: Hard cap on total tokens (default: 100K ≈ $0.01)
- **Smart prompting**: Agent instructed to use diffs sparingly

## Database

All data stored in SQLite database at `<data_dir>/activity.db`:

- `repositories`: Tracked repos with metadata
- `activity_runs`: Analysis results with summaries and cost tracking

Query example:
```sql
sqlite3 ~/.local/share/activity/activity.db \
  "SELECT agent_mode, tool_usage_stats FROM activity_runs ORDER BY id DESC LIMIT 1;"
```

## Development

See `CLAUDE.md` for architecture overview and package descriptions.

### Project Structure

```
cmd/activity/          - CLI entry point
internal/
  analyzer/           - Analysis logic (Phase 2 + Phase 3)
  cli/                - Command implementations
  config/             - Configuration management
  db/                 - Database layer
  git/                - Git operations
  llm/                - LLM client abstraction
```

## License

BSD 2-Clause License - see LICENSE file for details
