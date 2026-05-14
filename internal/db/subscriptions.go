package db

import (
	"database/sql"
	"fmt"
)

// CreateSubscription creates a subscription between a subscriber and a repository.
func (db *DB) CreateSubscription(subscriberID, repoID int64) (*Subscription, error) {
	var id int64
	err := db.QueryRow(`
		INSERT INTO subscriptions (subscriber_id, repo_id)
		VALUES ($1, $2)
		RETURNING id
	`, subscriberID, repoID).Scan(&id)
	if err != nil {
		return nil, fmt.Errorf("failed to create subscription: %w", err)
	}

	return db.GetSubscription(id)
}

// GetSubscription retrieves a subscription by ID.
func (db *DB) GetSubscription(id int64) (*Subscription, error) {
	sub := &Subscription{}
	err := db.QueryRow(`
		SELECT id, subscriber_id, repo_id, created_at
		FROM subscriptions
		WHERE id = $1
	`, id).Scan(&sub.ID, &sub.SubscriberID, &sub.RepoID, &sub.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("subscription not found: %w", ErrNotFound)
		}
		return nil, fmt.Errorf("failed to get subscription: %w", err)
	}
	return sub, nil
}

// GetSubscriptionBySubscriberAndRepo retrieves a subscription by subscriber and repo.
func (db *DB) GetSubscriptionBySubscriberAndRepo(subscriberID, repoID int64) (*Subscription, error) {
	sub := &Subscription{}
	err := db.QueryRow(`
		SELECT id, subscriber_id, repo_id, created_at
		FROM subscriptions
		WHERE subscriber_id = $1 AND repo_id = $2
	`, subscriberID, repoID).Scan(&sub.ID, &sub.SubscriberID, &sub.RepoID, &sub.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("subscription not found: %w", ErrNotFound)
		}
		return nil, fmt.Errorf("failed to get subscription: %w", err)
	}
	return sub, nil
}

// ListSubscriptionsBySubscriber retrieves all subscriptions for a subscriber.
func (db *DB) ListSubscriptionsBySubscriber(subscriberID int64) ([]*Subscription, error) {
	rows, err := db.Query(`
		SELECT id, subscriber_id, repo_id, created_at
		FROM subscriptions
		WHERE subscriber_id = $1
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

// DeleteSubscription deletes a subscription by ID.
func (db *DB) DeleteSubscription(id int64) error {
	_, err := db.Exec("DELETE FROM subscriptions WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("failed to delete subscription: %w", err)
	}
	return nil
}

// DeleteSubscriptionBySubscriberAndRepo deletes a subscription by subscriber and repo.
func (db *DB) DeleteSubscriptionBySubscriberAndRepo(subscriberID, repoID int64) error {
	_, err := db.Exec("DELETE FROM subscriptions WHERE subscriber_id = $1 AND repo_id = $2", subscriberID, repoID)
	if err != nil {
		return fmt.Errorf("failed to delete subscription: %w", err)
	}
	return nil
}
