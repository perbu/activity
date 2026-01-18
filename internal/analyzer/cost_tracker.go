package analyzer

import (
	"fmt"
	"time"
)

// DiffFetchRecord records a single diff fetch operation
type DiffFetchRecord struct {
	CommitSHA string    `json:"commit_sha"`
	SizeBytes int       `json:"size_bytes"`
	Reason    string    `json:"reason"`
	Timestamp time.Time `json:"timestamp"`
}

// CostTracker tracks resource usage during agent analysis
type CostTracker struct {
	maxDiffFetches   int
	maxDiffSizeBytes int
	maxTotalTokens   int

	// Runtime tracking
	diffsFetched    int
	totalDiffBytes  int
	estimatedTokens int
	diffFetchLog    []DiffFetchRecord
}

// NewCostTracker creates a new cost tracker with specified limits
func NewCostTracker(maxDiffFetches, maxDiffSizeBytes, maxTotalTokens int) *CostTracker {
	return &CostTracker{
		maxDiffFetches:   maxDiffFetches,
		maxDiffSizeBytes: maxDiffSizeBytes,
		maxTotalTokens:   maxTotalTokens,
		diffFetchLog:     make([]DiffFetchRecord, 0),
	}
}

// CanFetchMore checks if another diff can be fetched within limits
func (ct *CostTracker) CanFetchMore() (bool, string) {
	if ct.diffsFetched >= ct.maxDiffFetches {
		return false, fmt.Sprintf("reached maximum diff fetches (%d)", ct.maxDiffFetches)
	}
	if ct.estimatedTokens >= ct.maxTotalTokens {
		return false, fmt.Sprintf("reached maximum estimated tokens (%d)", ct.maxTotalTokens)
	}
	return true, ""
}

// RecordDiffFetch records a diff fetch operation
func (ct *CostTracker) RecordDiffFetch(sha string, size int, reason string) {
	ct.diffsFetched++
	ct.totalDiffBytes += size
	// Estimate ~4 bytes per token for code (conservative)
	ct.estimatedTokens += size / 4
	ct.diffFetchLog = append(ct.diffFetchLog, DiffFetchRecord{
		CommitSHA: sha,
		SizeBytes: size,
		Reason:    reason,
		Timestamp: time.Now(),
	})
}

// GetMetadata returns metadata about cost tracking
func (ct *CostTracker) GetMetadata() map[string]interface{} {
	return map[string]interface{}{
		"diffs_fetched":    ct.diffsFetched,
		"total_diff_bytes": ct.totalDiffBytes,
		"estimated_tokens": ct.estimatedTokens,
		"fetch_log":        ct.diffFetchLog,
	}
}

// GetMaxDiffSizeBytes returns the maximum allowed diff size
func (ct *CostTracker) GetMaxDiffSizeBytes() int {
	return ct.maxDiffSizeBytes
}

// GetDiffsFetched returns the number of diffs fetched so far
func (ct *CostTracker) GetDiffsFetched() int {
	return ct.diffsFetched
}

// GetTotalDiffBytes returns the total bytes of diffs fetched
func (ct *CostTracker) GetTotalDiffBytes() int {
	return ct.totalDiffBytes
}

// GetEstimatedTokens returns the estimated total tokens used
func (ct *CostTracker) GetEstimatedTokens() int {
	return ct.estimatedTokens
}
