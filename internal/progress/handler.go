package progress

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
)

// contextKey is a custom type for context keys to avoid collisions
type contextKey int

const sinkKey contextKey = iota

// Handler wraps a base slog.Handler and forwards tagged messages to a context sink.
// Messages tagged with "progress", true are forwarded to the sink function
// stored in the context, enabling real-time progress streaming to clients.
type Handler struct {
	base slog.Handler
}

// NewHandler creates a new Handler wrapping the base handler
func NewHandler(base slog.Handler) *Handler {
	return &Handler{base: base}
}

// Enabled returns whether the handler is enabled for the given level
func (h *Handler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.base.Enabled(ctx, level)
}

// Handle processes the log record, forwarding to base and optionally to context sink
func (h *Handler) Handle(ctx context.Context, r slog.Record) error {
	// Always delegate to base handler
	if err := h.base.Handle(ctx, r); err != nil {
		return err
	}

	// Check for progress tag and forward to sink if present
	if hasProgressTag(r) {
		if sink, ok := ctx.Value(sinkKey).(func(string)); ok && sink != nil {
			sink(formatRecord(r))
		}
	}

	return nil
}

// formatRecord formats a log record as "message key=value key=value ..."
// excluding the "progress" tag itself
func formatRecord(r slog.Record) string {
	var parts []string
	parts = append(parts, r.Message)

	r.Attrs(func(a slog.Attr) bool {
		// Skip the progress tag
		if a.Key == "progress" {
			return true
		}
		parts = append(parts, fmt.Sprintf("%s=%v", a.Key, a.Value.Any()))
		return true
	})

	return strings.Join(parts, " ")
}

// WithAttrs returns a new handler with the given attributes
func (h *Handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &Handler{base: h.base.WithAttrs(attrs)}
}

// WithGroup returns a new handler with the given group name
func (h *Handler) WithGroup(name string) slog.Handler {
	return &Handler{base: h.base.WithGroup(name)}
}

// hasProgressTag checks if the record has a "progress" attribute set to true
func hasProgressTag(r slog.Record) bool {
	var found bool
	r.Attrs(func(a slog.Attr) bool {
		if a.Key == "progress" && a.Value.Bool() {
			found = true
			return false // stop iteration
		}
		return true
	})
	return found
}

// WithProgressSink attaches a sink function to the context.
// The sink will receive log messages tagged with "progress", true.
func WithProgressSink(ctx context.Context, sink func(string)) context.Context {
	return context.WithValue(ctx, sinkKey, sink)
}

// Log logs a message with the progress tag, forwarding it to both the
// standard logger and any context-attached sink.
func Log(ctx context.Context, msg string, args ...any) {
	// Append progress tag to args
	args = append(args, "progress", true)
	slog.InfoContext(ctx, msg, args...)
}
