package forge

import (
	"context"
	"time"
)

// Forge represents a code hosting platform (GitHub, Forgejo, etc.)
type Forge interface {
	// ListMergedPRs returns PRs merged in the given time range, with their reviews
	ListMergedPRs(ctx context.Context, since, until time.Time) ([]PRWithReviews, error)

	// Type returns the forge type identifier ("github", "forgejo", etc.)
	Type() string
}
