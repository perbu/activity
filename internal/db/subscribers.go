package db

import (
	"database/sql"
	"fmt"
)

// CreateSubscriber inserts a new subscriber into the database.
func (db *DB) CreateSubscriber(email string, subscribeAll bool) (*Subscriber, error) {
	var id int64
	err := db.QueryRow(`
		INSERT INTO subscribers (email, subscribe_all)
		VALUES ($1, $2)
		RETURNING id
	`, email, subscribeAll).Scan(&id)
	if err != nil {
		return nil, fmt.Errorf("failed to create subscriber: %w", err)
	}

	return db.GetSubscriber(id)
}

// GetSubscriber retrieves a subscriber by ID.
func (db *DB) GetSubscriber(id int64) (*Subscriber, error) {
	sub := &Subscriber{}
	err := db.QueryRow(`
		SELECT id, email, subscribe_all, created_at
		FROM subscribers
		WHERE id = $1
	`, id).Scan(&sub.ID, &sub.Email, &sub.SubscribeAll, &sub.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("subscriber not found: %w", ErrNotFound)
		}
		return nil, fmt.Errorf("failed to get subscriber: %w", err)
	}
	return sub, nil
}

// GetSubscriberByEmail retrieves a subscriber by email.
func (db *DB) GetSubscriberByEmail(email string) (*Subscriber, error) {
	sub := &Subscriber{}
	err := db.QueryRow(`
		SELECT id, email, subscribe_all, created_at
		FROM subscribers
		WHERE email = $1
	`, email).Scan(&sub.ID, &sub.Email, &sub.SubscribeAll, &sub.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("subscriber not found: %w", ErrNotFound)
		}
		return nil, fmt.Errorf("failed to get subscriber: %w", err)
	}
	return sub, nil
}

// ListSubscribers retrieves all subscribers.
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

// UpdateSubscriber updates a subscriber's fields.
func (db *DB) UpdateSubscriber(sub *Subscriber) error {
	_, err := db.Exec(`
		UPDATE subscribers
		SET email = $1, subscribe_all = $2
		WHERE id = $3
	`, sub.Email, sub.SubscribeAll, sub.ID)
	if err != nil {
		return fmt.Errorf("failed to update subscriber: %w", err)
	}
	return nil
}

// DeleteSubscriber deletes a subscriber by ID.
func (db *DB) DeleteSubscriber(id int64) error {
	_, err := db.Exec("DELETE FROM subscribers WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("failed to delete subscriber: %w", err)
	}
	return nil
}
