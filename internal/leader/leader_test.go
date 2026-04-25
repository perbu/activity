package leader

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/perbu/activity/internal/testutil"
)

var testDSN string

func TestMain(m *testing.M) {
	flag.Parse()
	if testing.Short() {
		os.Exit(0)
	}

	dsn, terminate, err := testutil.StartPostgresContainer(context.Background())
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
	testDSN = dsn

	code := m.Run()

	terminate()
	os.Exit(code)
}

// openDB returns a sql.DB pointed at the shared test container. Each call gets
// its own pool so the two electors in TestFailover don't share connections.
func openDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("postgres", testDSN)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	db.SetMaxOpenConns(5)
	if err := db.Ping(); err != nil {
		db.Close()
		t.Fatalf("ping: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// withFastIntervals shortens the package-level intervals for the duration of a
// test so failover happens in seconds, not minutes.
func withFastIntervals(t *testing.T) {
	t.Helper()
	origH, origR := HeartbeatInterval, RetryInterval
	HeartbeatInterval = 50 * time.Millisecond
	RetryInterval = 100 * time.Millisecond
	t.Cleanup(func() {
		HeartbeatInterval = origH
		RetryInterval = origR
	})
}

// waitFor polls cond up to timeout, returning whether it became true.
func waitFor(timeout time.Duration, cond func() bool) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return cond()
}

// uniqueKey produces a per-test lock key so tests run against the shared
// container don't collide.
func uniqueKey(t *testing.T) int64 {
	t.Helper()
	const offset = 1469598103934665603
	const prime = 1099511628211
	h := uint64(offset)
	for _, b := range []byte(t.Name()) {
		h ^= uint64(b)
		h *= prime
	}
	return int64(h >> 1)
}

func TestElector_AcquiresLeadership(t *testing.T) {
	withFastIntervals(t)
	db := openDB(t)
	key := uniqueKey(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	e := New(db, key, "test")
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		e.Run(ctx)
	}()

	if !waitFor(2*time.Second, e.IsLeader) {
		t.Fatal("elector did not become leader within 2s")
	}

	cancel()
	wg.Wait()

	if e.IsLeader() {
		t.Fatal("elector still reports leader after Run returned")
	}
}

func TestElector_SecondInstanceIsNotLeader(t *testing.T) {
	withFastIntervals(t)
	key := uniqueKey(t)

	dbA := openDB(t)
	dbB := openDB(t)

	ctxA, cancelA := context.WithCancel(context.Background())
	defer cancelA()
	ctxB, cancelB := context.WithCancel(context.Background())
	defer cancelB()

	a := New(dbA, key, "A")
	b := New(dbB, key, "B")

	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); a.Run(ctxA) }()

	// Let A win the race.
	if !waitFor(2*time.Second, a.IsLeader) {
		t.Fatal("A did not become leader within 2s")
	}

	go func() { defer wg.Done(); b.Run(ctxB) }()

	// Give B several retry cycles to attempt and fail.
	time.Sleep(10 * RetryInterval)
	if b.IsLeader() {
		t.Fatal("B became leader while A still held the lock")
	}
	if !a.IsLeader() {
		t.Fatal("A unexpectedly lost leadership")
	}

	cancelA()
	cancelB()
	wg.Wait()
}

func TestElector_FailoverWhenLeaderExits(t *testing.T) {
	withFastIntervals(t)
	key := uniqueKey(t)

	dbA := openDB(t)
	dbB := openDB(t)

	ctxA, cancelA := context.WithCancel(context.Background())
	ctxB, cancelB := context.WithCancel(context.Background())
	defer cancelB()

	a := New(dbA, key, "A")
	b := New(dbB, key, "B")

	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); a.Run(ctxA) }()

	if !waitFor(2*time.Second, a.IsLeader) {
		t.Fatal("A did not become leader within 2s")
	}

	go func() { defer wg.Done(); b.Run(ctxB) }()

	// B should be polling and stuck without leadership.
	time.Sleep(5 * RetryInterval)
	if b.IsLeader() {
		t.Fatal("B became leader before A relinquished")
	}

	// Drop A; B should pick up leadership on its next retry.
	cancelA()

	if !waitFor(3*time.Second, b.IsLeader) {
		t.Fatal("B did not take over leadership within 3s of A exiting")
	}

	cancelB()
	wg.Wait()
}

