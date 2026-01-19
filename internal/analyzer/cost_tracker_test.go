package analyzer

import (
	"testing"
)

func TestNewCostTracker(t *testing.T) {
	tests := []struct {
		name             string
		maxDiffFetches   int
		maxDiffSizeBytes int
		maxTotalTokens   int
	}{
		{"default values", 5, 10240, 100000},
		{"zero limits", 0, 0, 0},
		{"high limits", 100, 1048576, 1000000},
		{"single fetch allowed", 1, 1024, 1000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ct := NewCostTracker(tt.maxDiffFetches, tt.maxDiffSizeBytes, tt.maxTotalTokens)

			if ct == nil {
				t.Fatal("NewCostTracker returned nil")
			}

			if ct.GetMaxDiffSizeBytes() != tt.maxDiffSizeBytes {
				t.Errorf("GetMaxDiffSizeBytes() = %d, want %d",
					ct.GetMaxDiffSizeBytes(), tt.maxDiffSizeBytes)
			}

			// New tracker should have zero counts
			if ct.GetDiffsFetched() != 0 {
				t.Errorf("new tracker GetDiffsFetched() = %d, want 0", ct.GetDiffsFetched())
			}
			if ct.GetTotalDiffBytes() != 0 {
				t.Errorf("new tracker GetTotalDiffBytes() = %d, want 0", ct.GetTotalDiffBytes())
			}
			if ct.GetEstimatedTokens() != 0 {
				t.Errorf("new tracker GetEstimatedTokens() = %d, want 0", ct.GetEstimatedTokens())
			}
		})
	}
}

func TestCanFetchMore(t *testing.T) {
	tests := []struct {
		name           string
		maxDiffs       int
		maxTokens      int
		fetchCount     int
		fetchSizes     []int
		wantCanFetch   bool
		wantMsgContain string
	}{
		{
			name:         "under limits",
			maxDiffs:     5,
			maxTokens:    100000,
			fetchCount:   2,
			fetchSizes:   []int{1000, 1000},
			wantCanFetch: true,
		},
		{
			name:           "at diff limit",
			maxDiffs:       3,
			maxTokens:      100000,
			fetchCount:     3,
			fetchSizes:     []int{100, 100, 100},
			wantCanFetch:   false,
			wantMsgContain: "maximum diff fetches",
		},
		{
			name:           "over diff limit",
			maxDiffs:       2,
			maxTokens:      100000,
			fetchCount:     3,
			fetchSizes:     []int{100, 100, 100},
			wantCanFetch:   false,
			wantMsgContain: "maximum diff fetches",
		},
		{
			name:           "at token limit",
			maxDiffs:       100,
			maxTokens:      1000,
			fetchCount:     1,
			fetchSizes:     []int{4000}, // 4000 bytes / 4 = 1000 tokens
			wantCanFetch:   false,
			wantMsgContain: "maximum estimated tokens",
		},
		{
			name:           "over token limit",
			maxDiffs:       100,
			maxTokens:      1000,
			fetchCount:     1,
			fetchSizes:     []int{8000}, // 8000 bytes / 4 = 2000 tokens
			wantCanFetch:   false,
			wantMsgContain: "maximum estimated tokens",
		},
		{
			name:           "zero diff limit",
			maxDiffs:       0,
			maxTokens:      100000,
			fetchCount:     0,
			fetchSizes:     nil,
			wantCanFetch:   false,
			wantMsgContain: "maximum diff fetches",
		},
		{
			name:           "zero token limit",
			maxDiffs:       100,
			maxTokens:      0,
			fetchCount:     0,
			fetchSizes:     nil,
			wantCanFetch:   false,
			wantMsgContain: "maximum estimated tokens",
		},
		{
			name:         "one fetch remaining",
			maxDiffs:     5,
			maxTokens:    100000,
			fetchCount:   4,
			fetchSizes:   []int{100, 100, 100, 100},
			wantCanFetch: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ct := NewCostTracker(tt.maxDiffs, 10240, tt.maxTokens)

			// Record fetches
			for i := 0; i < tt.fetchCount && i < len(tt.fetchSizes); i++ {
				ct.RecordDiffFetch("sha"+string(rune('a'+i)), tt.fetchSizes[i], "test")
			}

			canFetch, msg := ct.CanFetchMore()

			if canFetch != tt.wantCanFetch {
				t.Errorf("CanFetchMore() = %v, want %v (msg: %q)", canFetch, tt.wantCanFetch, msg)
			}

			if !tt.wantCanFetch && tt.wantMsgContain != "" {
				if msg == "" {
					t.Errorf("expected message containing %q, got empty string", tt.wantMsgContain)
				} else if !contains(msg, tt.wantMsgContain) {
					t.Errorf("message %q does not contain %q", msg, tt.wantMsgContain)
				}
			}
		})
	}
}

func TestRecordDiffFetch(t *testing.T) {
	ct := NewCostTracker(10, 10240, 100000)

	// Record first fetch
	ct.RecordDiffFetch("abc123", 1000, "commit message was vague")

	if ct.GetDiffsFetched() != 1 {
		t.Errorf("after 1 fetch, GetDiffsFetched() = %d, want 1", ct.GetDiffsFetched())
	}
	if ct.GetTotalDiffBytes() != 1000 {
		t.Errorf("after 1 fetch, GetTotalDiffBytes() = %d, want 1000", ct.GetTotalDiffBytes())
	}
	// Token estimation: 1000 bytes / 4 = 250 tokens
	if ct.GetEstimatedTokens() != 250 {
		t.Errorf("after 1 fetch, GetEstimatedTokens() = %d, want 250", ct.GetEstimatedTokens())
	}

	// Record second fetch
	ct.RecordDiffFetch("def456", 2000, "needed scope verification")

	if ct.GetDiffsFetched() != 2 {
		t.Errorf("after 2 fetches, GetDiffsFetched() = %d, want 2", ct.GetDiffsFetched())
	}
	if ct.GetTotalDiffBytes() != 3000 {
		t.Errorf("after 2 fetches, GetTotalDiffBytes() = %d, want 3000", ct.GetTotalDiffBytes())
	}
	// Token estimation: 3000 bytes / 4 = 750 tokens
	if ct.GetEstimatedTokens() != 750 {
		t.Errorf("after 2 fetches, GetEstimatedTokens() = %d, want 750", ct.GetEstimatedTokens())
	}
}

func TestRecordDiffFetchLog(t *testing.T) {
	ct := NewCostTracker(10, 10240, 100000)

	ct.RecordDiffFetch("sha1", 100, "reason1")
	ct.RecordDiffFetch("sha2", 200, "reason2")

	metadata := ct.GetMetadata()

	fetchLog, ok := metadata["fetch_log"].([]DiffFetchRecord)
	if !ok {
		t.Fatal("GetMetadata() fetch_log is not []DiffFetchRecord")
	}

	if len(fetchLog) != 2 {
		t.Errorf("fetch_log length = %d, want 2", len(fetchLog))
	}

	// Check first record
	if fetchLog[0].CommitSHA != "sha1" {
		t.Errorf("first record SHA = %q, want %q", fetchLog[0].CommitSHA, "sha1")
	}
	if fetchLog[0].SizeBytes != 100 {
		t.Errorf("first record SizeBytes = %d, want 100", fetchLog[0].SizeBytes)
	}
	if fetchLog[0].Reason != "reason1" {
		t.Errorf("first record Reason = %q, want %q", fetchLog[0].Reason, "reason1")
	}
	if fetchLog[0].Timestamp.IsZero() {
		t.Error("first record Timestamp should not be zero")
	}

	// Check second record
	if fetchLog[1].CommitSHA != "sha2" {
		t.Errorf("second record SHA = %q, want %q", fetchLog[1].CommitSHA, "sha2")
	}
	if fetchLog[1].SizeBytes != 200 {
		t.Errorf("second record SizeBytes = %d, want 200", fetchLog[1].SizeBytes)
	}
}

func TestGetMetadata(t *testing.T) {
	ct := NewCostTracker(5, 10240, 100000)

	ct.RecordDiffFetch("sha1", 400, "test")
	ct.RecordDiffFetch("sha2", 600, "test")

	metadata := ct.GetMetadata()

	if metadata["diffs_fetched"].(int) != 2 {
		t.Errorf("metadata diffs_fetched = %v, want 2", metadata["diffs_fetched"])
	}
	if metadata["total_diff_bytes"].(int) != 1000 {
		t.Errorf("metadata total_diff_bytes = %v, want 1000", metadata["total_diff_bytes"])
	}
	if metadata["estimated_tokens"].(int) != 250 {
		t.Errorf("metadata estimated_tokens = %v, want 250", metadata["estimated_tokens"])
	}
}

func TestTokenEstimation(t *testing.T) {
	// Verify the ~4 bytes per token estimation
	tests := []struct {
		sizeBytes      int
		expectedTokens int
	}{
		{4, 1},
		{8, 2},
		{100, 25},
		{1000, 250},
		{4000, 1000},
		{3, 0}, // Integer division rounds down
		{7, 1},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			ct := NewCostTracker(100, 10240, 100000)
			ct.RecordDiffFetch("sha", tt.sizeBytes, "test")

			if ct.GetEstimatedTokens() != tt.expectedTokens {
				t.Errorf("for %d bytes, GetEstimatedTokens() = %d, want %d",
					tt.sizeBytes, ct.GetEstimatedTokens(), tt.expectedTokens)
			}
		})
	}
}

func TestGetterMethods(t *testing.T) {
	maxDiffFetches := 5
	maxDiffSizeBytes := 10240
	maxTotalTokens := 100000

	ct := NewCostTracker(maxDiffFetches, maxDiffSizeBytes, maxTotalTokens)

	// Test GetMaxDiffSizeBytes
	if got := ct.GetMaxDiffSizeBytes(); got != maxDiffSizeBytes {
		t.Errorf("GetMaxDiffSizeBytes() = %d, want %d", got, maxDiffSizeBytes)
	}

	// Record some data
	ct.RecordDiffFetch("sha1", 500, "test")
	ct.RecordDiffFetch("sha2", 300, "test")

	// Test GetDiffsFetched
	if got := ct.GetDiffsFetched(); got != 2 {
		t.Errorf("GetDiffsFetched() = %d, want 2", got)
	}

	// Test GetTotalDiffBytes
	if got := ct.GetTotalDiffBytes(); got != 800 {
		t.Errorf("GetTotalDiffBytes() = %d, want 800", got)
	}

	// Test GetEstimatedTokens (800 / 4 = 200)
	if got := ct.GetEstimatedTokens(); got != 200 {
		t.Errorf("GetEstimatedTokens() = %d, want 200", got)
	}
}

// contains checks if substr is in s (simple helper to avoid importing strings)
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
