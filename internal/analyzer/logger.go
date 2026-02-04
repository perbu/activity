package analyzer

import (
	"encoding/json"
	"log/slog"
	"sync"

	"github.com/perbu/activity/internal/db"
)

// AnalysisLogger records LLM interactions to the database for transparency
type AnalysisLogger struct {
	db            *db.DB
	activityRunID int64
	sequence      int
	mu            sync.Mutex
}

// NewAnalysisLogger creates a new logger for recording analysis interactions
func NewAnalysisLogger(database *db.DB, activityRunID int64) *AnalysisLogger {
	return &AnalysisLogger{
		db:            database,
		activityRunID: activityRunID,
		sequence:      0,
	}
}

// nextSequence returns the next sequence number (thread-safe)
func (l *AnalysisLogger) nextSequence() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.sequence++
	return l.sequence
}

// LogPrompt records a prompt sent to the LLM
func (l *AnalysisLogger) LogPrompt(content string) {
	seq := l.nextSequence()
	_, err := l.db.CreateAnalysisLog(l.activityRunID, "prompt", "", content, seq)
	if err != nil {
		slog.Warn("failed to log prompt", "error", err, "run_id", l.activityRunID)
	}
}

// LogResponse records a response received from the LLM
func (l *AnalysisLogger) LogResponse(content string) {
	seq := l.nextSequence()
	_, err := l.db.CreateAnalysisLog(l.activityRunID, "response", "", content, seq)
	if err != nil {
		slog.Warn("failed to log response", "error", err, "run_id", l.activityRunID)
	}
}

// LogToolCall records a tool call made by the agent
func (l *AnalysisLogger) LogToolCall(toolName string, args any) {
	seq := l.nextSequence()
	content := formatToolArgs(args)
	_, err := l.db.CreateAnalysisLog(l.activityRunID, "tool_call", toolName, content, seq)
	if err != nil {
		slog.Warn("failed to log tool call", "error", err, "run_id", l.activityRunID, "tool", toolName)
	}
}

// LogToolResult records the result of a tool call
func (l *AnalysisLogger) LogToolResult(toolName string, result any) {
	seq := l.nextSequence()
	content := formatToolArgs(result)
	_, err := l.db.CreateAnalysisLog(l.activityRunID, "tool_result", toolName, content, seq)
	if err != nil {
		slog.Warn("failed to log tool result", "error", err, "run_id", l.activityRunID, "tool", toolName)
	}
}

// formatToolArgs converts tool arguments/results to a readable string
func formatToolArgs(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "<failed to serialize>"
	}
	return string(b)
}
