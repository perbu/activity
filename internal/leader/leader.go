// Package leader provides single-instance leader election backed by a
// PostgreSQL session-level advisory lock. Use it to gate work that must run on
// only one replica at a time (e.g. scheduled jobs) when multiple instances
// share the same database.
package leader

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"
)

// SchedulerLockKey is the advisory lock key used to gate the scheduled
// pipeline. The bytes spell "activity" in ASCII.
const SchedulerLockKey int64 = 0x6163746976697479

// Defaults for the election loop. Exposed as package vars so tests can lower
// them; production code should not mutate these.
var (
	HeartbeatInterval = 15 * time.Second
	RetryInterval     = 30 * time.Second
)

// Elector acquires and holds a session-level advisory lock on a dedicated
// connection. Run blocks until ctx is cancelled.
type Elector struct {
	db   *sql.DB
	key  int64
	name string

	leader atomic.Bool
}

// New constructs an Elector. name is used in log lines only.
func New(database *sql.DB, key int64, name string) *Elector {
	return &Elector{db: database, key: key, name: name}
}

// IsLeader reports whether this instance currently holds the lock. Safe to
// call from any goroutine.
func (e *Elector) IsLeader() bool {
	return e.leader.Load()
}

// Run drives the election loop. It returns when ctx is cancelled.
func (e *Elector) Run(ctx context.Context) {
	for {
		if err := ctx.Err(); err != nil {
			return
		}
		err := e.runSession(ctx)
		if err != nil && !errors.Is(err, context.Canceled) {
			slog.Warn("Leader election session ended", "name", e.name, "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(RetryInterval):
		}
	}
}

// runSession opens a dedicated connection, attempts to acquire the lock, and
// holds it (heartbeating) until ctx is cancelled or the connection breaks.
func (e *Elector) runSession(ctx context.Context) error {
	conn, err := e.db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("acquire conn: %w", err)
	}
	defer conn.Close()

	var acquired bool
	if err := conn.QueryRowContext(ctx, "SELECT pg_try_advisory_lock($1)", e.key).Scan(&acquired); err != nil {
		return fmt.Errorf("try advisory lock: %w", err)
	}
	if !acquired {
		slog.Debug("Leadership held by another instance", "name", e.name)
		return nil
	}

	slog.Info("Acquired leadership", "name", e.name)
	e.leader.Store(true)
	defer func() {
		e.leader.Store(false)
		e.releaseLock(conn)
		slog.Info("Released leadership", "name", e.name)
	}()

	ticker := time.NewTicker(HeartbeatInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := conn.PingContext(ctx); err != nil {
				return fmt.Errorf("heartbeat: %w", err)
			}
		}
	}
}

// releaseLock explicitly drops the advisory lock before the connection is
// returned to the pool. Without this, *sql.Conn.Close() recycles the
// underlying session into the pool with the lock still held — breaking
// failover. If the unlock query fails (e.g. because the connection is already
// bad), force the conn to be discarded instead of pooled.
func (e *Elector) releaseLock(conn *sql.Conn) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := conn.ExecContext(ctx, "SELECT pg_advisory_unlock($1)", e.key); err != nil {
		slog.Warn("Failed to release advisory lock; dropping connection", "name", e.name, "error", err)
		_ = conn.Raw(func(any) error { return driver.ErrBadConn })
	}
}
