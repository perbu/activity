package progress

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
)

func TestProgressHandler_ForwardsToSink(t *testing.T) {
	var buf bytes.Buffer
	base := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})
	handler := NewHandler(base)
	logger := slog.New(handler)
	slog.SetDefault(logger)

	var captured []string
	sink := func(msg string) {
		captured = append(captured, msg)
	}

	ctx := WithProgressSink(context.Background(), sink)

	// Log with progress tag - should be captured
	slog.InfoContext(ctx, "test message", "progress", true)

	if len(captured) != 1 {
		t.Fatalf("expected 1 captured message, got %d", len(captured))
	}
	if captured[0] != "test message" {
		t.Errorf("expected 'test message', got %q", captured[0])
	}

	// Also verify it went to the base handler
	if !strings.Contains(buf.String(), "test message") {
		t.Errorf("expected message in base handler output, got %q", buf.String())
	}
}

func TestProgressHandler_IncludesAttributes(t *testing.T) {
	var buf bytes.Buffer
	base := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})
	handler := NewHandler(base)
	logger := slog.New(handler)
	slog.SetDefault(logger)

	var captured []string
	sink := func(msg string) {
		captured = append(captured, msg)
	}

	ctx := WithProgressSink(context.Background(), sink)

	// Log with progress tag and extra attributes
	slog.InfoContext(ctx, "Updating repository", "name", "myrepo", "commits", 5, "progress", true)

	if len(captured) != 1 {
		t.Fatalf("expected 1 captured message, got %d", len(captured))
	}

	// Should include message and attributes (but not progress tag)
	if !strings.Contains(captured[0], "Updating repository") {
		t.Errorf("expected message in output, got %q", captured[0])
	}
	if !strings.Contains(captured[0], "name=myrepo") {
		t.Errorf("expected name=myrepo in output, got %q", captured[0])
	}
	if !strings.Contains(captured[0], "commits=5") {
		t.Errorf("expected commits=5 in output, got %q", captured[0])
	}
	if strings.Contains(captured[0], "progress=") {
		t.Errorf("progress tag should be excluded, got %q", captured[0])
	}
}

func TestProgressHandler_IgnoresWithoutTag(t *testing.T) {
	var buf bytes.Buffer
	base := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})
	handler := NewHandler(base)
	logger := slog.New(handler)
	slog.SetDefault(logger)

	var captured []string
	sink := func(msg string) {
		captured = append(captured, msg)
	}

	ctx := WithProgressSink(context.Background(), sink)

	// Log without progress tag - should NOT be captured
	slog.InfoContext(ctx, "regular message")

	if len(captured) != 0 {
		t.Fatalf("expected 0 captured messages, got %d", len(captured))
	}

	// But should still go to base handler
	if !strings.Contains(buf.String(), "regular message") {
		t.Errorf("expected message in base handler output, got %q", buf.String())
	}
}

func TestProgressHandler_IgnoresWithoutSink(t *testing.T) {
	var buf bytes.Buffer
	base := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})
	handler := NewHandler(base)
	logger := slog.New(handler)
	slog.SetDefault(logger)

	// No sink attached
	ctx := context.Background()

	// Log with progress tag but no sink - should not panic
	slog.InfoContext(ctx, "test message", "progress", true)

	// Should still go to base handler
	if !strings.Contains(buf.String(), "test message") {
		t.Errorf("expected message in base handler output, got %q", buf.String())
	}
}

func TestLog_AddsProgressTag(t *testing.T) {
	var buf bytes.Buffer
	base := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})
	handler := NewHandler(base)
	logger := slog.New(handler)
	slog.SetDefault(logger)

	var captured []string
	sink := func(msg string) {
		captured = append(captured, msg)
	}

	ctx := WithProgressSink(context.Background(), sink)

	// Use the Log convenience function
	Log(ctx, "progress message", "extra", "attr")

	if len(captured) != 1 {
		t.Fatalf("expected 1 captured message, got %d", len(captured))
	}
	if !strings.Contains(captured[0], "progress message") {
		t.Errorf("expected 'progress message' in output, got %q", captured[0])
	}
	if !strings.Contains(captured[0], "extra=attr") {
		t.Errorf("expected 'extra=attr' in output, got %q", captured[0])
	}
}

func TestProgressHandler_IsolatesContexts(t *testing.T) {
	var buf bytes.Buffer
	base := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})
	handler := NewHandler(base)
	logger := slog.New(handler)
	slog.SetDefault(logger)

	var captured1, captured2 []string
	sink1 := func(msg string) { captured1 = append(captured1, msg) }
	sink2 := func(msg string) { captured2 = append(captured2, msg) }

	ctx1 := WithProgressSink(context.Background(), sink1)
	ctx2 := WithProgressSink(context.Background(), sink2)

	Log(ctx1, "message for sink1")
	Log(ctx2, "message for sink2")

	if len(captured1) != 1 || !strings.Contains(captured1[0], "message for sink1") {
		t.Errorf("sink1 captured wrong messages: %v", captured1)
	}
	if len(captured2) != 1 || !strings.Contains(captured2[0], "message for sink2") {
		t.Errorf("sink2 captured wrong messages: %v", captured2)
	}
}
