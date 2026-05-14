package db

import (
	"database/sql"
	"fmt"
	"time"
)

// CreateNewsletterSend records that an activity run was sent to a subscriber.
func (db *DB) CreateNewsletterSend(subscriberID, activityRunID int64, messageID string) (*NewsletterSend, error) {
	var msgID interface{}
	if messageID != "" {
		msgID = messageID
	}

	var id int64
	err := db.QueryRow(`
		INSERT INTO newsletter_sends (subscriber_id, activity_run_id, sendgrid_message_id)
		VALUES ($1, $2, $3)
		RETURNING id
	`, subscriberID, activityRunID, msgID).Scan(&id)
	if err != nil {
		return nil, fmt.Errorf("failed to create newsletter send: %w", err)
	}

	return db.GetNewsletterSend(id)
}

// GetNewsletterSend retrieves a newsletter send by ID.
func (db *DB) GetNewsletterSend(id int64) (*NewsletterSend, error) {
	ns := &NewsletterSend{}
	err := db.QueryRow(`
		SELECT id, subscriber_id, activity_run_id, sent_at, sendgrid_message_id
		FROM newsletter_sends
		WHERE id = $1
	`, id).Scan(&ns.ID, &ns.SubscriberID, &ns.ActivityRunID, &ns.SentAt, &ns.SendGridMessageID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("newsletter send not found: %w", ErrNotFound)
		}
		return nil, fmt.Errorf("failed to get newsletter send: %w", err)
	}
	return ns, nil
}

// HasNewsletterBeenSent checks if a specific activity run has been sent to a subscriber.
func (db *DB) HasNewsletterBeenSent(subscriberID, activityRunID int64) (bool, error) {
	var count int
	err := db.QueryRow(`
		SELECT COUNT(*) FROM newsletter_sends
		WHERE subscriber_id = $1 AND activity_run_id = $2
	`, subscriberID, activityRunID).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check newsletter send: %w", err)
	}
	return count > 0, nil
}

// GetUnsentActivityRuns retrieves activity runs that haven't been sent to a subscriber
// for the repositories they're subscribed to (or all repos if subscribe_all is true).
func (db *DB) GetUnsentActivityRuns(subscriberID int64, since time.Time) ([]*ActivityRun, error) {
	sub, err := db.GetSubscriber(subscriberID)
	if err != nil {
		return nil, err
	}

	var query string
	var args []interface{}

	if sub.SubscribeAll {
		query = `
			SELECT ar.id, ar.repo_id, ar.start_sha, ar.end_sha, ar.started_at, ar.completed_at,
			       ar.summary, ar.raw_data, COALESCE(ar.agent_mode, false), ar.tool_usage_stats
			FROM activity_runs ar
			WHERE ar.completed_at IS NOT NULL
			  AND ar.completed_at >= $1
			  AND ar.id NOT IN (
			      SELECT activity_run_id FROM newsletter_sends WHERE subscriber_id = $2
			  )
			ORDER BY ar.completed_at DESC
		`
		args = []interface{}{since, subscriberID}
	} else {
		query = `
			SELECT ar.id, ar.repo_id, ar.start_sha, ar.end_sha, ar.started_at, ar.completed_at,
			       ar.summary, ar.raw_data, COALESCE(ar.agent_mode, false), ar.tool_usage_stats
			FROM activity_runs ar
			INNER JOIN subscriptions s ON ar.repo_id = s.repo_id
			WHERE s.subscriber_id = $1
			  AND ar.completed_at IS NOT NULL
			  AND ar.completed_at >= $2
			  AND ar.id NOT IN (
			      SELECT activity_run_id FROM newsletter_sends WHERE subscriber_id = $3
			  )
			ORDER BY ar.completed_at DESC
		`
		args = []interface{}{subscriberID, since, subscriberID}
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get unsent activity runs: %w", err)
	}
	defer rows.Close()

	var runs []*ActivityRun
	for rows.Next() {
		run := &ActivityRun{}
		if err := rows.Scan(
			&run.ID, &run.RepoID, &run.StartSHA, &run.EndSHA,
			&run.StartedAt, &run.CompletedAt, &run.Summary, &run.RawData,
			&run.AgentMode, &run.ToolUsageStats,
		); err != nil {
			return nil, fmt.Errorf("failed to scan activity run: %w", err)
		}
		runs = append(runs, run)
	}

	return runs, nil
}

// GetReposForSubscriber returns the repositories a subscriber should receive updates for.
func (db *DB) GetReposForSubscriber(subscriberID int64) ([]*Repository, error) {
	sub, err := db.GetSubscriber(subscriberID)
	if err != nil {
		return nil, err
	}

	if sub.SubscribeAll {
		activeOnly := true
		return db.ListRepositories(&activeOnly)
	}

	rows, err := db.Query(`
		SELECT r.id, r.name, r.url, r.branch, r.active, COALESCE(r.private, false), COALESCE(r.external, false), r.description,
		       r.forge_type, r.forge_owner, r.forge_repo, r.created_at, r.updated_at, r.last_run_at, r.last_run_sha
		FROM repositories r
		INNER JOIN subscriptions s ON r.id = s.repo_id
		WHERE s.subscriber_id = $1
		ORDER BY r.name
	`, subscriberID)
	if err != nil {
		return nil, fmt.Errorf("failed to get repos for subscriber: %w", err)
	}
	defer rows.Close()

	var repos []*Repository
	for rows.Next() {
		repo := &Repository{}
		if err := rows.Scan(
			&repo.ID, &repo.Name, &repo.URL, &repo.Branch,
			&repo.Active, &repo.Private, &repo.External, &repo.Description,
			&repo.ForgeType, &repo.ForgeOwner, &repo.ForgeRepo,
			&repo.CreatedAt, &repo.UpdatedAt, &repo.LastRunAt, &repo.LastRunSHA,
		); err != nil {
			return nil, fmt.Errorf("failed to scan repository: %w", err)
		}
		repos = append(repos, repo)
	}

	return repos, nil
}
