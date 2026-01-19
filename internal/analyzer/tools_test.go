package analyzer

import (
	"testing"
)

func TestGetCommitDiffTool_Metadata(t *testing.T) {
	ct := NewCostTracker(5, 10, 100000)
	tool := NewGetCommitDiffTool("/fake/path", ct)

	if tool.Name() != "get_commit_diff" {
		t.Errorf("Name() = %q, want %q", tool.Name(), "get_commit_diff")
	}

	if tool.Description() == "" {
		t.Error("Description() should not be empty")
	}

	if tool.IsLongRunning() {
		t.Error("IsLongRunning() should be false")
	}

	decl := tool.Declaration()
	if decl == nil {
		t.Fatal("Declaration() returned nil")
	}
	if decl.Name != "get_commit_diff" {
		t.Errorf("Declaration().Name = %q, want %q", decl.Name, "get_commit_diff")
	}
	if decl.Parameters == nil {
		t.Error("Declaration().Parameters should not be nil")
	}
	if len(decl.Parameters.Required) != 2 {
		t.Errorf("Declaration() should require 2 parameters, got %d", len(decl.Parameters.Required))
	}
}

func TestGetCommitDiffTool_RunInvalidArgs(t *testing.T) {
	ct := NewCostTracker(5, 10, 100000)
	tool := NewGetCommitDiffTool("/fake/path", ct)

	tests := []struct {
		name string
		args any
	}{
		{"nil args", nil},
		{"int args", 123},
		{"missing commit_sha", map[string]any{"reason": "test"}},
		{"missing reason", map[string]any{"commit_sha": "abc123"}},
		{"wrong type commit_sha", map[string]any{"commit_sha": 123, "reason": "test"}},
		{"wrong type reason", map[string]any{"commit_sha": "abc123", "reason": 456}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tool.Run(nil, tt.args)
			if err != nil {
				t.Errorf("Run() returned unexpected error: %v", err)
			}
			// Should return error in the result map
			if result == nil {
				t.Error("Run() returned nil result")
				return
			}
			if _, hasError := result["error"]; !hasError {
				t.Error("Run() with invalid args should return error in result")
			}
		})
	}
}

func TestGetCommitDiffTool_RunDeniedByTracker(t *testing.T) {
	// Create a tracker that's already at its limit
	ct := NewCostTracker(0, 10, 100000) // 0 max fetches
	tool := NewGetCommitDiffTool("/fake/path", ct)

	result, err := tool.Run(nil, map[string]any{
		"commit_sha": "abc123",
		"reason":     "test",
	})

	if err != nil {
		t.Errorf("Run() returned unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("Run() returned nil result")
	}
	if _, hasError := result["error"]; !hasError {
		t.Error("Run() when tracker denies fetch should return error")
	}
}

func TestGetCommitDiffFullTool_Metadata(t *testing.T) {
	ct := NewCostTracker(5, 10, 100000)
	tool := NewGetCommitDiffFullTool("/fake/path", ct)

	if tool.Name() != "get_commit_diff_full" {
		t.Errorf("Name() = %q, want %q", tool.Name(), "get_commit_diff_full")
	}

	if tool.Description() == "" {
		t.Error("Description() should not be empty")
	}

	if tool.IsLongRunning() {
		t.Error("IsLongRunning() should be false")
	}

	decl := tool.Declaration()
	if decl == nil {
		t.Fatal("Declaration() returned nil")
	}
	if decl.Name != "get_commit_diff_full" {
		t.Errorf("Declaration().Name = %q, want %q", decl.Name, "get_commit_diff_full")
	}
}

func TestGetCommitDiffFullTool_RunInvalidArgs(t *testing.T) {
	ct := NewCostTracker(5, 10, 100000)
	tool := NewGetCommitDiffFullTool("/fake/path", ct)

	tests := []struct {
		name string
		args any
	}{
		{"nil args", nil},
		{"missing commit_sha", map[string]any{"reason": "test"}},
		{"missing reason", map[string]any{"commit_sha": "abc123"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tool.Run(nil, tt.args)
			if err != nil {
				t.Errorf("Run() returned unexpected error: %v", err)
			}
			if result == nil {
				t.Error("Run() returned nil result")
				return
			}
			if _, hasError := result["error"]; !hasError {
				t.Error("Run() with invalid args should return error in result")
			}
		})
	}
}

func TestGetFullCommitMessageTool_Metadata(t *testing.T) {
	tool := NewGetFullCommitMessageTool("/fake/path")

	if tool.Name() != "get_full_commit_message" {
		t.Errorf("Name() = %q, want %q", tool.Name(), "get_full_commit_message")
	}

	if tool.Description() == "" {
		t.Error("Description() should not be empty")
	}

	if tool.IsLongRunning() {
		t.Error("IsLongRunning() should be false")
	}

	decl := tool.Declaration()
	if decl == nil {
		t.Fatal("Declaration() returned nil")
	}
	if decl.Name != "get_full_commit_message" {
		t.Errorf("Declaration().Name = %q, want %q", decl.Name, "get_full_commit_message")
	}
	if len(decl.Parameters.Required) != 1 {
		t.Errorf("Declaration() should require 1 parameter, got %d", len(decl.Parameters.Required))
	}
}

func TestGetFullCommitMessageTool_RunInvalidArgs(t *testing.T) {
	tool := NewGetFullCommitMessageTool("/fake/path")

	tests := []struct {
		name string
		args any
	}{
		{"nil args", nil},
		{"empty map", map[string]any{}},
		{"wrong type commit_sha", map[string]any{"commit_sha": 123}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tool.Run(nil, tt.args)
			if err != nil {
				t.Errorf("Run() returned unexpected error: %v", err)
			}
			if result == nil {
				t.Error("Run() returned nil result")
				return
			}
			if _, hasError := result["error"]; !hasError {
				t.Error("Run() with invalid args should return error in result")
			}
		})
	}
}

func TestGetAuthorStatsTool_Metadata(t *testing.T) {
	tool := NewGetAuthorStatsTool("/fake/path")

	if tool.Name() != "get_author_stats" {
		t.Errorf("Name() = %q, want %q", tool.Name(), "get_author_stats")
	}

	if tool.Description() == "" {
		t.Error("Description() should not be empty")
	}

	if tool.IsLongRunning() {
		t.Error("IsLongRunning() should be false")
	}

	decl := tool.Declaration()
	if decl == nil {
		t.Fatal("Declaration() returned nil")
	}
	if decl.Name != "get_author_stats" {
		t.Errorf("Declaration().Name = %q, want %q", decl.Name, "get_author_stats")
	}
	if len(decl.Parameters.Required) != 1 {
		t.Errorf("Declaration() should require 1 parameter, got %d", len(decl.Parameters.Required))
	}
}

func TestGetAuthorStatsTool_RunInvalidArgs(t *testing.T) {
	tool := NewGetAuthorStatsTool("/fake/path")

	tests := []struct {
		name string
		args any
	}{
		{"nil args", nil},
		{"empty map", map[string]any{}},
		{"wrong type author_name", map[string]any{"author_name": 123}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tool.Run(nil, tt.args)
			if err != nil {
				t.Errorf("Run() returned unexpected error: %v", err)
			}
			if result == nil {
				t.Error("Run() returned nil result")
				return
			}
			if _, hasError := result["error"]; !hasError {
				t.Error("Run() with invalid args should return error in result")
			}
		})
	}
}

func TestToolJSONArgs(t *testing.T) {
	ct := NewCostTracker(0, 10, 100000) // 0 max to ensure we get the "denied" error
	tool := NewGetCommitDiffTool("/fake/path", ct)

	// Test with JSON string args
	jsonArgs := `{"commit_sha": "abc123", "reason": "test reason"}`
	result, err := tool.Run(nil, jsonArgs)

	if err != nil {
		t.Errorf("Run() with JSON args returned error: %v", err)
	}
	if result == nil {
		t.Error("Run() returned nil result")
	}
	// Should get denied by tracker but args should parse correctly
}
