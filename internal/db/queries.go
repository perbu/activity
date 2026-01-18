package db

import (
	"database/sql"
	"fmt"
	"time"
)

// Repository CRUD operations

// CreateRepository inserts a new repository into the database
func (db *DB) CreateRepository(name, url, branch, localPath string) (*Repository, error) {
	result, err := db.Exec(`
		INSERT INTO repositories (name, url, branch, local_path, active)
		VALUES (?, ?, ?, ?, 1)
	`, name, url, branch, localPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create repository: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get repository ID: %w", err)
	}

	return db.GetRepository(id)
}

// GetRepository retrieves a repository by ID
func (db *DB) GetRepository(id int64) (*Repository, error) {
	repo := &Repository{}
	err := db.QueryRow(`
		SELECT id, name, url, branch, local_path, active, created_at, updated_at, last_run_at, last_run_sha
		FROM repositories
		WHERE id = ?
	`, id).Scan(
		&repo.ID, &repo.Name, &repo.URL, &repo.Branch, &repo.LocalPath,
		&repo.Active, &repo.CreatedAt, &repo.UpdatedAt, &repo.LastRunAt, &repo.LastRunSHA,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("repository not found")
		}
		return nil, fmt.Errorf("failed to get repository: %w", err)
	}
	return repo, nil
}

// GetRepositoryByName retrieves a repository by name
func (db *DB) GetRepositoryByName(name string) (*Repository, error) {
	repo := &Repository{}
	err := db.QueryRow(`
		SELECT id, name, url, branch, local_path, active, created_at, updated_at, last_run_at, last_run_sha
		FROM repositories
		WHERE name = ?
	`, name).Scan(
		&repo.ID, &repo.Name, &repo.URL, &repo.Branch, &repo.LocalPath,
		&repo.Active, &repo.CreatedAt, &repo.UpdatedAt, &repo.LastRunAt, &repo.LastRunSHA,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("repository not found")
		}
		return nil, fmt.Errorf("failed to get repository: %w", err)
	}
	return repo, nil
}

// ListRepositories retrieves all repositories, optionally filtered by active status
func (db *DB) ListRepositories(activeOnly *bool) ([]*Repository, error) {
	query := `
		SELECT id, name, url, branch, local_path, active, created_at, updated_at, last_run_at, last_run_sha
		FROM repositories
	`
	var args []interface{}

	if activeOnly != nil {
		query += " WHERE active = ?"
		args = append(args, *activeOnly)
	}

	query += " ORDER BY name"

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list repositories: %w", err)
	}
	defer rows.Close()

	var repos []*Repository
	for rows.Next() {
		repo := &Repository{}
		err := rows.Scan(
			&repo.ID, &repo.Name, &repo.URL, &repo.Branch, &repo.LocalPath,
			&repo.Active, &repo.CreatedAt, &repo.UpdatedAt, &repo.LastRunAt, &repo.LastRunSHA,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan repository: %w", err)
		}
		repos = append(repos, repo)
	}

	return repos, nil
}

// UpdateRepository updates a repository's fields
func (db *DB) UpdateRepository(repo *Repository) error {
	repo.UpdatedAt = time.Now()
	_, err := db.Exec(`
		UPDATE repositories
		SET name = ?, url = ?, branch = ?, local_path = ?, active = ?, updated_at = ?, last_run_at = ?, last_run_sha = ?
		WHERE id = ?
	`, repo.Name, repo.URL, repo.Branch, repo.LocalPath, repo.Active, repo.UpdatedAt, repo.LastRunAt, repo.LastRunSHA, repo.ID)
	if err != nil {
		return fmt.Errorf("failed to update repository: %w", err)
	}
	return nil
}

// DeleteRepository deletes a repository by ID
func (db *DB) DeleteRepository(id int64) error {
	_, err := db.Exec("DELETE FROM repositories WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("failed to delete repository: %w", err)
	}
	return nil
}

// SetRepositoryActive sets the active status of a repository
func (db *DB) SetRepositoryActive(id int64, active bool) error {
	_, err := db.Exec(`
		UPDATE repositories
		SET active = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, active, id)
	if err != nil {
		return fmt.Errorf("failed to set repository active status: %w", err)
	}
	return nil
}

// ActivityRun CRUD operations

// CreateActivityRun inserts a new activity run into the database
func (db *DB) CreateActivityRun(repoID int64, startSHA, endSHA string) (*ActivityRun, error) {
	result, err := db.Exec(`
		INSERT INTO activity_runs (repo_id, start_sha, end_sha)
		VALUES (?, ?, ?)
	`, repoID, startSHA, endSHA)
	if err != nil {
		return nil, fmt.Errorf("failed to create activity run: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get activity run ID: %w", err)
	}

	return db.GetActivityRun(id)
}

// GetActivityRun retrieves an activity run by ID
func (db *DB) GetActivityRun(id int64) (*ActivityRun, error) {
	run := &ActivityRun{}
	err := db.QueryRow(`
		SELECT id, repo_id, start_sha, end_sha, started_at, completed_at, summary, raw_data,
		       COALESCE(agent_mode, 0), tool_usage_stats
		FROM activity_runs
		WHERE id = ?
	`, id).Scan(
		&run.ID, &run.RepoID, &run.StartSHA, &run.EndSHA,
		&run.StartedAt, &run.CompletedAt, &run.Summary, &run.RawData,
		&run.AgentMode, &run.ToolUsageStats,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("activity run not found")
		}
		return nil, fmt.Errorf("failed to get activity run: %w", err)
	}
	return run, nil
}

// GetLatestActivityRun retrieves the most recent activity run for a repository
func (db *DB) GetLatestActivityRun(repoID int64) (*ActivityRun, error) {
	run := &ActivityRun{}
	err := db.QueryRow(`
		SELECT id, repo_id, start_sha, end_sha, started_at, completed_at, summary, raw_data,
		       COALESCE(agent_mode, 0), tool_usage_stats
		FROM activity_runs
		WHERE repo_id = ?
		ORDER BY started_at DESC
		LIMIT 1
	`, repoID).Scan(
		&run.ID, &run.RepoID, &run.StartSHA, &run.EndSHA,
		&run.StartedAt, &run.CompletedAt, &run.Summary, &run.RawData,
		&run.AgentMode, &run.ToolUsageStats,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // No runs yet
		}
		return nil, fmt.Errorf("failed to get latest activity run: %w", err)
	}
	return run, nil
}

// UpdateActivityRun updates an activity run
func (db *DB) UpdateActivityRun(run *ActivityRun) error {
	_, err := db.Exec(`
		UPDATE activity_runs
		SET completed_at = ?, summary = ?, raw_data = ?, agent_mode = ?, tool_usage_stats = ?
		WHERE id = ?
	`, run.CompletedAt, run.Summary, run.RawData, run.AgentMode, run.ToolUsageStats, run.ID)
	if err != nil {
		return fmt.Errorf("failed to update activity run: %w", err)
	}
	return nil
}

// Subscriber CRUD operations

// CreateSubscriber inserts a new subscriber into the database
func (db *DB) CreateSubscriber(email string, subscribeAll bool) (*Subscriber, error) {
	result, err := db.Exec(`
		INSERT INTO subscribers (email, subscribe_all)
		VALUES (?, ?)
	`, email, subscribeAll)
	if err != nil {
		return nil, fmt.Errorf("failed to create subscriber: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get subscriber ID: %w", err)
	}

	return db.GetSubscriber(id)
}

// GetSubscriber retrieves a subscriber by ID
func (db *DB) GetSubscriber(id int64) (*Subscriber, error) {
	sub := &Subscriber{}
	err := db.QueryRow(`
		SELECT id, email, subscribe_all, created_at
		FROM subscribers
		WHERE id = ?
	`, id).Scan(&sub.ID, &sub.Email, &sub.SubscribeAll, &sub.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("subscriber not found")
		}
		return nil, fmt.Errorf("failed to get subscriber: %w", err)
	}
	return sub, nil
}

// GetSubscriberByEmail retrieves a subscriber by email
func (db *DB) GetSubscriberByEmail(email string) (*Subscriber, error) {
	sub := &Subscriber{}
	err := db.QueryRow(`
		SELECT id, email, subscribe_all, created_at
		FROM subscribers
		WHERE email = ?
	`, email).Scan(&sub.ID, &sub.Email, &sub.SubscribeAll, &sub.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("subscriber not found")
		}
		return nil, fmt.Errorf("failed to get subscriber: %w", err)
	}
	return sub, nil
}

// ListSubscribers retrieves all subscribers
func (db *DB) ListSubscribers() ([]*Subscriber, error) {
	rows, err := db.Query(`
		SELECT id, email, subscribe_all, created_at
		FROM subscribers
		ORDER BY email
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to list subscribers: %w", err)
	}
	defer rows.Close()

	var subs []*Subscriber
	for rows.Next() {
		sub := &Subscriber{}
		if err := rows.Scan(&sub.ID, &sub.Email, &sub.SubscribeAll, &sub.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan subscriber: %w", err)
		}
		subs = append(subs, sub)
	}

	return subs, nil
}

// UpdateSubscriber updates a subscriber's fields
func (db *DB) UpdateSubscriber(sub *Subscriber) error {
	_, err := db.Exec(`
		UPDATE subscribers
		SET email = ?, subscribe_all = ?
		WHERE id = ?
	`, sub.Email, sub.SubscribeAll, sub.ID)
	if err != nil {
		return fmt.Errorf("failed to update subscriber: %w", err)
	}
	return nil
}

// DeleteSubscriber deletes a subscriber by ID
func (db *DB) DeleteSubscriber(id int64) error {
	_, err := db.Exec("DELETE FROM subscribers WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("failed to delete subscriber: %w", err)
	}
	return nil
}

// Subscription CRUD operations

// CreateSubscription creates a subscription between a subscriber and a repository
func (db *DB) CreateSubscription(subscriberID, repoID int64) (*Subscription, error) {
	result, err := db.Exec(`
		INSERT INTO subscriptions (subscriber_id, repo_id)
		VALUES (?, ?)
	`, subscriberID, repoID)
	if err != nil {
		return nil, fmt.Errorf("failed to create subscription: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get subscription ID: %w", err)
	}

	return db.GetSubscription(id)
}

// GetSubscription retrieves a subscription by ID
func (db *DB) GetSubscription(id int64) (*Subscription, error) {
	sub := &Subscription{}
	err := db.QueryRow(`
		SELECT id, subscriber_id, repo_id, created_at
		FROM subscriptions
		WHERE id = ?
	`, id).Scan(&sub.ID, &sub.SubscriberID, &sub.RepoID, &sub.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("subscription not found")
		}
		return nil, fmt.Errorf("failed to get subscription: %w", err)
	}
	return sub, nil
}

// GetSubscriptionBySubscriberAndRepo retrieves a subscription by subscriber and repo
func (db *DB) GetSubscriptionBySubscriberAndRepo(subscriberID, repoID int64) (*Subscription, error) {
	sub := &Subscription{}
	err := db.QueryRow(`
		SELECT id, subscriber_id, repo_id, created_at
		FROM subscriptions
		WHERE subscriber_id = ? AND repo_id = ?
	`, subscriberID, repoID).Scan(&sub.ID, &sub.SubscriberID, &sub.RepoID, &sub.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("subscription not found")
		}
		return nil, fmt.Errorf("failed to get subscription: %w", err)
	}
	return sub, nil
}

// ListSubscriptionsBySubscriber retrieves all subscriptions for a subscriber
func (db *DB) ListSubscriptionsBySubscriber(subscriberID int64) ([]*Subscription, error) {
	rows, err := db.Query(`
		SELECT id, subscriber_id, repo_id, created_at
		FROM subscriptions
		WHERE subscriber_id = ?
		ORDER BY created_at
	`, subscriberID)
	if err != nil {
		return nil, fmt.Errorf("failed to list subscriptions: %w", err)
	}
	defer rows.Close()

	var subs []*Subscription
	for rows.Next() {
		sub := &Subscription{}
		if err := rows.Scan(&sub.ID, &sub.SubscriberID, &sub.RepoID, &sub.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan subscription: %w", err)
		}
		subs = append(subs, sub)
	}

	return subs, nil
}

// DeleteSubscription deletes a subscription by ID
func (db *DB) DeleteSubscription(id int64) error {
	_, err := db.Exec("DELETE FROM subscriptions WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("failed to delete subscription: %w", err)
	}
	return nil
}

// DeleteSubscriptionBySubscriberAndRepo deletes a subscription by subscriber and repo
func (db *DB) DeleteSubscriptionBySubscriberAndRepo(subscriberID, repoID int64) error {
	_, err := db.Exec("DELETE FROM subscriptions WHERE subscriber_id = ? AND repo_id = ?", subscriberID, repoID)
	if err != nil {
		return fmt.Errorf("failed to delete subscription: %w", err)
	}
	return nil
}

// NewsletterSend CRUD operations

// CreateNewsletterSend records that an activity run was sent to a subscriber
func (db *DB) CreateNewsletterSend(subscriberID, activityRunID int64, messageID string) (*NewsletterSend, error) {
	var msgID interface{}
	if messageID != "" {
		msgID = messageID
	}

	result, err := db.Exec(`
		INSERT INTO newsletter_sends (subscriber_id, activity_run_id, sendgrid_message_id)
		VALUES (?, ?, ?)
	`, subscriberID, activityRunID, msgID)
	if err != nil {
		return nil, fmt.Errorf("failed to create newsletter send: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get newsletter send ID: %w", err)
	}

	return db.GetNewsletterSend(id)
}

// GetNewsletterSend retrieves a newsletter send by ID
func (db *DB) GetNewsletterSend(id int64) (*NewsletterSend, error) {
	ns := &NewsletterSend{}
	err := db.QueryRow(`
		SELECT id, subscriber_id, activity_run_id, sent_at, sendgrid_message_id
		FROM newsletter_sends
		WHERE id = ?
	`, id).Scan(&ns.ID, &ns.SubscriberID, &ns.ActivityRunID, &ns.SentAt, &ns.SendGridMessageID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("newsletter send not found")
		}
		return nil, fmt.Errorf("failed to get newsletter send: %w", err)
	}
	return ns, nil
}

// HasNewsletterBeenSent checks if a specific activity run has been sent to a subscriber
func (db *DB) HasNewsletterBeenSent(subscriberID, activityRunID int64) (bool, error) {
	var count int
	err := db.QueryRow(`
		SELECT COUNT(*) FROM newsletter_sends
		WHERE subscriber_id = ? AND activity_run_id = ?
	`, subscriberID, activityRunID).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check newsletter send: %w", err)
	}
	return count > 0, nil
}

// GetUnsentActivityRuns retrieves activity runs that haven't been sent to a subscriber
// for the repositories they're subscribed to (or all repos if subscribe_all is true)
func (db *DB) GetUnsentActivityRuns(subscriberID int64, since time.Time) ([]*ActivityRun, error) {
	// Get the subscriber to check subscribe_all flag
	sub, err := db.GetSubscriber(subscriberID)
	if err != nil {
		return nil, err
	}

	var query string
	var args []interface{}

	if sub.SubscribeAll {
		// Get all completed activity runs since the given time that haven't been sent
		query = `
			SELECT ar.id, ar.repo_id, ar.start_sha, ar.end_sha, ar.started_at, ar.completed_at,
			       ar.summary, ar.raw_data, COALESCE(ar.agent_mode, 0), ar.tool_usage_stats
			FROM activity_runs ar
			WHERE ar.completed_at IS NOT NULL
			  AND ar.completed_at >= ?
			  AND ar.id NOT IN (
			      SELECT activity_run_id FROM newsletter_sends WHERE subscriber_id = ?
			  )
			ORDER BY ar.completed_at DESC
		`
		args = []interface{}{since, subscriberID}
	} else {
		// Get activity runs for subscribed repos only
		query = `
			SELECT ar.id, ar.repo_id, ar.start_sha, ar.end_sha, ar.started_at, ar.completed_at,
			       ar.summary, ar.raw_data, COALESCE(ar.agent_mode, 0), ar.tool_usage_stats
			FROM activity_runs ar
			INNER JOIN subscriptions s ON ar.repo_id = s.repo_id
			WHERE s.subscriber_id = ?
			  AND ar.completed_at IS NOT NULL
			  AND ar.completed_at >= ?
			  AND ar.id NOT IN (
			      SELECT activity_run_id FROM newsletter_sends WHERE subscriber_id = ?
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

// GetReposForSubscriber returns the repositories a subscriber should receive updates for
func (db *DB) GetReposForSubscriber(subscriberID int64) ([]*Repository, error) {
	sub, err := db.GetSubscriber(subscriberID)
	if err != nil {
		return nil, err
	}

	if sub.SubscribeAll {
		// Return all active repos
		activeOnly := true
		return db.ListRepositories(&activeOnly)
	}

	// Return only subscribed repos
	rows, err := db.Query(`
		SELECT r.id, r.name, r.url, r.branch, r.local_path, r.active, r.created_at, r.updated_at, r.last_run_at, r.last_run_sha
		FROM repositories r
		INNER JOIN subscriptions s ON r.id = s.repo_id
		WHERE s.subscriber_id = ?
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
			&repo.ID, &repo.Name, &repo.URL, &repo.Branch, &repo.LocalPath,
			&repo.Active, &repo.CreatedAt, &repo.UpdatedAt, &repo.LastRunAt, &repo.LastRunSHA,
		); err != nil {
			return nil, fmt.Errorf("failed to scan repository: %w", err)
		}
		repos = append(repos, repo)
	}

	return repos, nil
}

// WeeklyReport CRUD operations

// CreateWeeklyReport inserts a new weekly report into the database
func (db *DB) CreateWeeklyReport(report *WeeklyReport) (*WeeklyReport, error) {
	result, err := db.Exec(`
		INSERT INTO weekly_reports (repo_id, year, week, week_start, week_end, summary, commit_count, metadata, agent_mode, tool_usage_stats, source_run_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, report.RepoID, report.Year, report.Week, report.WeekStart, report.WeekEnd,
		report.Summary, report.CommitCount, report.Metadata, report.AgentMode,
		report.ToolUsageStats, report.SourceRunID)
	if err != nil {
		return nil, fmt.Errorf("failed to create weekly report: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get weekly report ID: %w", err)
	}

	return db.GetWeeklyReport(id)
}

// GetWeeklyReport retrieves a weekly report by ID
func (db *DB) GetWeeklyReport(id int64) (*WeeklyReport, error) {
	report := &WeeklyReport{}
	err := db.QueryRow(`
		SELECT id, repo_id, year, week, week_start, week_end, summary, commit_count,
		       metadata, COALESCE(agent_mode, 0), tool_usage_stats, created_at, updated_at, source_run_id
		FROM weekly_reports
		WHERE id = ?
	`, id).Scan(
		&report.ID, &report.RepoID, &report.Year, &report.Week,
		&report.WeekStart, &report.WeekEnd, &report.Summary, &report.CommitCount,
		&report.Metadata, &report.AgentMode, &report.ToolUsageStats,
		&report.CreatedAt, &report.UpdatedAt, &report.SourceRunID,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("weekly report not found")
		}
		return nil, fmt.Errorf("failed to get weekly report: %w", err)
	}
	return report, nil
}

// GetWeeklyReportByRepoAndWeek retrieves a weekly report by repository, year, and week
func (db *DB) GetWeeklyReportByRepoAndWeek(repoID int64, year, week int) (*WeeklyReport, error) {
	report := &WeeklyReport{}
	err := db.QueryRow(`
		SELECT id, repo_id, year, week, week_start, week_end, summary, commit_count,
		       metadata, COALESCE(agent_mode, 0), tool_usage_stats, created_at, updated_at, source_run_id
		FROM weekly_reports
		WHERE repo_id = ? AND year = ? AND week = ?
	`, repoID, year, week).Scan(
		&report.ID, &report.RepoID, &report.Year, &report.Week,
		&report.WeekStart, &report.WeekEnd, &report.Summary, &report.CommitCount,
		&report.Metadata, &report.AgentMode, &report.ToolUsageStats,
		&report.CreatedAt, &report.UpdatedAt, &report.SourceRunID,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Not found, return nil without error
		}
		return nil, fmt.Errorf("failed to get weekly report: %w", err)
	}
	return report, nil
}

// GetLatestWeeklyReport retrieves the most recent weekly report for a repository
func (db *DB) GetLatestWeeklyReport(repoID int64) (*WeeklyReport, error) {
	report := &WeeklyReport{}
	err := db.QueryRow(`
		SELECT id, repo_id, year, week, week_start, week_end, summary, commit_count,
		       metadata, COALESCE(agent_mode, 0), tool_usage_stats, created_at, updated_at, source_run_id
		FROM weekly_reports
		WHERE repo_id = ?
		ORDER BY year DESC, week DESC
		LIMIT 1
	`, repoID).Scan(
		&report.ID, &report.RepoID, &report.Year, &report.Week,
		&report.WeekStart, &report.WeekEnd, &report.Summary, &report.CommitCount,
		&report.Metadata, &report.AgentMode, &report.ToolUsageStats,
		&report.CreatedAt, &report.UpdatedAt, &report.SourceRunID,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // No reports yet
		}
		return nil, fmt.Errorf("failed to get latest weekly report: %w", err)
	}
	return report, nil
}

// ListWeeklyReportsByRepo retrieves all weekly reports for a repository, optionally filtered by year
func (db *DB) ListWeeklyReportsByRepo(repoID int64, year *int) ([]*WeeklyReport, error) {
	var query string
	var args []interface{}

	if year != nil {
		query = `
			SELECT id, repo_id, year, week, week_start, week_end, summary, commit_count,
			       metadata, COALESCE(agent_mode, 0), tool_usage_stats, created_at, updated_at, source_run_id
			FROM weekly_reports
			WHERE repo_id = ? AND year = ?
			ORDER BY year DESC, week DESC
		`
		args = []interface{}{repoID, *year}
	} else {
		query = `
			SELECT id, repo_id, year, week, week_start, week_end, summary, commit_count,
			       metadata, COALESCE(agent_mode, 0), tool_usage_stats, created_at, updated_at, source_run_id
			FROM weekly_reports
			WHERE repo_id = ?
			ORDER BY year DESC, week DESC
		`
		args = []interface{}{repoID}
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list weekly reports: %w", err)
	}
	defer rows.Close()

	var reports []*WeeklyReport
	for rows.Next() {
		report := &WeeklyReport{}
		if err := rows.Scan(
			&report.ID, &report.RepoID, &report.Year, &report.Week,
			&report.WeekStart, &report.WeekEnd, &report.Summary, &report.CommitCount,
			&report.Metadata, &report.AgentMode, &report.ToolUsageStats,
			&report.CreatedAt, &report.UpdatedAt, &report.SourceRunID,
		); err != nil {
			return nil, fmt.Errorf("failed to scan weekly report: %w", err)
		}
		reports = append(reports, report)
	}

	return reports, nil
}

// ListAllWeeklyReports retrieves all weekly reports, optionally filtered by year
func (db *DB) ListAllWeeklyReports(year *int) ([]*WeeklyReport, error) {
	var query string
	var args []interface{}

	if year != nil {
		query = `
			SELECT id, repo_id, year, week, week_start, week_end, summary, commit_count,
			       metadata, COALESCE(agent_mode, 0), tool_usage_stats, created_at, updated_at, source_run_id
			FROM weekly_reports
			WHERE year = ?
			ORDER BY year DESC, week DESC, repo_id
		`
		args = []interface{}{*year}
	} else {
		query = `
			SELECT id, repo_id, year, week, week_start, week_end, summary, commit_count,
			       metadata, COALESCE(agent_mode, 0), tool_usage_stats, created_at, updated_at, source_run_id
			FROM weekly_reports
			ORDER BY year DESC, week DESC, repo_id
		`
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list weekly reports: %w", err)
	}
	defer rows.Close()

	var reports []*WeeklyReport
	for rows.Next() {
		report := &WeeklyReport{}
		if err := rows.Scan(
			&report.ID, &report.RepoID, &report.Year, &report.Week,
			&report.WeekStart, &report.WeekEnd, &report.Summary, &report.CommitCount,
			&report.Metadata, &report.AgentMode, &report.ToolUsageStats,
			&report.CreatedAt, &report.UpdatedAt, &report.SourceRunID,
		); err != nil {
			return nil, fmt.Errorf("failed to scan weekly report: %w", err)
		}
		reports = append(reports, report)
	}

	return reports, nil
}

// UpdateWeeklyReport updates an existing weekly report
func (db *DB) UpdateWeeklyReport(report *WeeklyReport) error {
	report.UpdatedAt = time.Now()
	_, err := db.Exec(`
		UPDATE weekly_reports
		SET summary = ?, commit_count = ?, metadata = ?, agent_mode = ?,
		    tool_usage_stats = ?, updated_at = ?, source_run_id = ?
		WHERE id = ?
	`, report.Summary, report.CommitCount, report.Metadata, report.AgentMode,
		report.ToolUsageStats, report.UpdatedAt, report.SourceRunID, report.ID)
	if err != nil {
		return fmt.Errorf("failed to update weekly report: %w", err)
	}
	return nil
}

// WeeklyReportExists checks if a weekly report exists for the given repo, year, and week
func (db *DB) WeeklyReportExists(repoID int64, year, week int) (bool, error) {
	var count int
	err := db.QueryRow(`
		SELECT COUNT(*) FROM weekly_reports
		WHERE repo_id = ? AND year = ? AND week = ?
	`, repoID, year, week).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check weekly report existence: %w", err)
	}
	return count > 0, nil
}

// DeleteWeeklyReport deletes a weekly report by ID
func (db *DB) DeleteWeeklyReport(id int64) error {
	_, err := db.Exec("DELETE FROM weekly_reports WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("failed to delete weekly report: %w", err)
	}
	return nil
}
