# Analyzer Package

This package implements AI-powered git commit analysis using Google's Gemini API and ADK, Agent Development Kit,
framework.

## Architecture Overview

The analyzer supports two modes of operation:

1. **Simple LLM Mode** (`use_agent: false`) - Sends commit metadata directly to Gemini for summarization
2. **Agent Mode** (`use_agent: true`, default) - Uses an ADK agent that can intelligently fetch additional context via
   tools

## Core Types

### Analyzer

The main service struct that orchestrates analysis:

```go
package analyzer

import (
	"github.com/perbu/activity/internal/config"
	"github.com/perbu/activity/internal/db"
	"github.com/perbu/activity/internal/llm"
)

type Analyzer struct {
	llmClient *llm.Client
	db        *db.DB
	config    *config.Config
}

```

Created via `New(llmClient, database, cfg)`. The entry points are:

- `AnalyzeCommits()` - Returns summary string only
- `AnalyzeAndSave()` - Analyzes and persists to database with metadata

## Agent Mode Implementation

### Creating an Agent

The agent is created in `createAnalyzerAgent()` using the ADK `llmagent` package:

```go
package analyzer

import (
	"google.golang.org/adk/agent/llmagent"
)
func createAnalyzerAgent() {
	// ..
	agentConfig := llmagent.Config{
		Name:        "git_analyzer",
		Description: "Analyzes git commits and provides summaries",
		Model:       geminiModel,      // *genai.GenerativeModel from llm.Client
		Instruction: systemPrompt,     // Agent's system instructions
		Tools:       []tool.Tool{...}, // Available tools
	}
	agt, err := llmagent.New(agentConfig)
	// ..
}

```

### Running the Agent

Agent execution uses the ADK runner with an in-memory session:

```go
package analyzer

func (a *Analyzer) analyzeWithAgent() {
	// Create session service
	sessionService := session.InMemoryService()

	// Create runner
	r, err := runner.New(runner.Config{
		AppName:        "activity-analyzer",
		Agent:          agt,
		SessionService: sessionService,
	})

	// Create session
	sessionService.Create(ctx, &session.CreateRequest{
		AppName:   "activity-analyzer",
		UserID:    "user1",
		SessionID: "session1",
	})

	// Create user message
	userMessage := genai.NewContentFromText(prompt, genai.RoleUser)

	// Run agent (returns iterator of events)
	for event, err := range r.Run(ctx, "user1", "session1", userMessage, agent.RunConfig{}) {
		// Process events - extract text from event.Content.Parts
	}
}
```

## Defining Tools

Tools implement the `tool.Tool` interface from ADK. Each tool needs:

### Required Methods

```go
package analyzer

import "google.golang.org/adk/tool"

type Tool interface {
	Name() string
	Description() string
	IsLongRunning() bool
	ProcessRequest(ctx tool.Context, req *model.LLMRequest) error
	Run(ctx tool.Context, args any) (map[string]any, error)
}

```

### Function Declaration

Tools also provide a `Declaration()` method returning `*genai.FunctionDeclaration`:

```go
package analyzer

import "google.golang.org/genai"

func (t *MyTool) Declaration() *genai.FunctionDeclaration {
	return &genai.FunctionDeclaration{
		Name:        t.Name(),
		Description: t.Description(),
		Parameters: &genai.Schema{
			Type: "object",
			Properties: map[string]*genai.Schema{
				"param_name": {
					Type:        "string",
					Description: "What this parameter does",
				},
			},
			Required: []string{"param_name"},
		},
	}
}

```

### ProcessRequest Implementation

Uses the helper function to add the tool to the LLM request:

```go
package analyzer

import "google.golang.org/adk/tool"

func (t *MyTool) ProcessRequest(ctx tool.Context, req *model.LLMRequest) error {
	return addFunctionTool(req, t)
}

```

### Run Implementation

Parses arguments and executes the tool logic:

```go
package analyzer

import (
	"encoding/json"
	"google.golang.org/adk/tool"
)

func (t *MyTool) Run(ctx tool.Context, args any) (map[string]any, error) {
	// Parse args - may be map[string]any or JSON string
	argsMap, ok := args.(map[string]any)
	if !ok {
		if argsStr, ok := args.(string); ok {
			json.Unmarshal([]byte(argsStr), &argsMap)
		}
	}

	// Extract parameters
	param := argsMap["param_name"].(string)

	// Do work...

	// Return result as map
	return map[string]any{
		"result": "value",
	}, nil
}

```

## Available Tools

### get_commit_diff

Fetches the code diff for a specific commit. Requires a `reason` parameter to encourage thoughtful usage. Subject to
cost tracking limits.

Parameters:

- `commit_sha` (required) - The commit SHA (8 or 40 chars)
- `reason` (required) - Why the diff is needed

### get_full_commit_message

Retrieves the full commit message including body (useful when messages are truncated in the prompt).

Parameters:

- `commit_sha` (required) - The commit SHA

### get_author_stats

Provides statistics about an author's contributions.

Parameters:

- `author_name` (required) - The author name as it appears in commits

## Cost Tracking

The `CostTracker` enforces limits on expensive operations:

```go
package analyzer

tracker := NewCostTracker(
maxDiffFetches,   // Max number of diff fetches
maxDiffSizeBytes, // Max size per individual diff
maxTotalTokens,   // Estimated token limit
)

```

Before fetching a diff, tools check:

```go
package analyzer

canFetch, msg := tracker.CanFetchMore()
if !canFetch {
return map[string]any{"error": msg}, nil
}
```

After fetching:

```go
tracker.RecordDiffFetch(sha, len(diff), reason)
```

The tracker estimates tokens at ~4 bytes per token and maintains a log of all fetches for debugging and metadata
storage.

## Configuration

Relevant config options (from `config.LLMConfig`):

| Option               | Description            | Default |
|----------------------|------------------------|---------|
| `use_agent`          | Enable agent mode      | `true`  |
| `max_diff_fetches`   | Max diffs per analysis | 5       |
| `max_diff_size_kb`   | Max size per diff      | 50      |
| `max_total_tokens`   | Token budget estimate  | 100000  |
| `max_commits`        | Max commits to include | 50      |
| `max_message_length` | Truncate messages at   | 1000    |

## Import Summary

```go
package analyzer

import (
	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/runner"
	"google.golang.org/adk/session"
	"google.golang.org/adk/tool"
	"google.golang.org/adk/model"
	"google.golang.org/genai"
)

```
