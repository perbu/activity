package db

import "fmt"

// SiteStats aggregates high-level statistics for the admin dashboard.
type SiteStats struct {
	NewslettersSent int
	TotalPageViews  int
	UniqueVisitors  int
}

// RecordPageView inserts a page view record. userHash should be the hex-encoded
// SHA-256 of the user's email, or empty for anonymous visitors.
func (db *DB) RecordPageView(userHash, path string) error {
	var nullableHash any
	if userHash != "" {
		nullableHash = userHash
	}
	_, err := db.Exec(`
		INSERT INTO page_views (user_hash, path) VALUES ($1, $2)
	`, nullableHash, path)
	if err != nil {
		return fmt.Errorf("record page view: %w", err)
	}
	return nil
}

// GetSiteStats returns aggregate counts for the admin dashboard.
func (db *DB) GetSiteStats() (*SiteStats, error) {
	s := &SiteStats{}

	if err := db.QueryRow(`SELECT COUNT(*) FROM newsletter_sends`).Scan(&s.NewslettersSent); err != nil {
		return nil, fmt.Errorf("count newsletter_sends: %w", err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM page_views`).Scan(&s.TotalPageViews); err != nil {
		return nil, fmt.Errorf("count page_views: %w", err)
	}
	if err := db.QueryRow(`SELECT COUNT(DISTINCT user_hash) FROM page_views WHERE user_hash IS NOT NULL`).Scan(&s.UniqueVisitors); err != nil {
		return nil, fmt.Errorf("count unique visitors: %w", err)
	}
	return s, nil
}
