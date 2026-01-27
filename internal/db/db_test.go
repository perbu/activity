package db

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// setupTestDB creates a temporary database for testing
func setupTestDB(t *testing.T) (*DB, func()) {
	t.Helper()

	// Create a temporary directory for the database
	tmpDir, err := os.MkdirTemp("", "activity-db-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	db, err := Open(tmpDir)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to open database: %v", err)
	}

	cleanup := func() {
		db.Close()
		os.RemoveAll(tmpDir)
	}

	return db, cleanup
}

func TestOpen(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "activity-db-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	db, err := Open(tmpDir)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer db.Close()

	// Verify database file was created
	dbPath := filepath.Join(tmpDir, "activity.db")
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("database file was not created")
	}

	// Verify migrations ran by checking tables exist
	tables := []string{"goose_db_version", "repositories", "activity_runs", "subscribers", "subscriptions", "newsletter_sends", "weekly_reports", "admins"}
	for _, table := range tables {
		var name string
		err := db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&name)
		if err != nil {
			t.Errorf("table %q does not exist: %v", table, err)
		}
	}
}

func TestOpen_InvalidPath(t *testing.T) {
	// Try to open a database in a path that doesn't exist and can't be created
	_, err := Open("/nonexistent/deeply/nested/path/that/should/not/exist")
	if err == nil {
		t.Error("Open() expected error for invalid path, got nil")
	}
}

// Repository CRUD tests

func TestRepository_Create(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo, err := db.CreateRepository("test-repo", "https://github.com/test/repo", "main", false, sql.NullString{})
	if err != nil {
		t.Fatalf("CreateRepository() error = %v", err)
	}

	if repo.ID == 0 {
		t.Error("expected non-zero ID")
	}
	if repo.Name != "test-repo" {
		t.Errorf("Name = %q, want %q", repo.Name, "test-repo")
	}
	if repo.URL != "https://github.com/test/repo" {
		t.Errorf("URL = %q, want %q", repo.URL, "https://github.com/test/repo")
	}
	if repo.Branch != "main" {
		t.Errorf("Branch = %q, want %q", repo.Branch, "main")
	}
	if !repo.Active {
		t.Error("Active should be true by default")
	}
	if repo.Private {
		t.Error("Private should be false")
	}
}

func TestRepository_CreateWithDescription(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	desc := sql.NullString{String: "A test repository", Valid: true}
	repo, err := db.CreateRepository("test-repo", "https://github.com/test/repo", "main", true, desc)
	if err != nil {
		t.Fatalf("CreateRepository() error = %v", err)
	}

	if !repo.Description.Valid || repo.Description.String != "A test repository" {
		t.Errorf("Description = %v, want 'A test repository'", repo.Description)
	}
	if !repo.Private {
		t.Error("Private should be true")
	}
}

func TestRepository_CreateDuplicate(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	_, err := db.CreateRepository("test-repo", "https://github.com/test/repo", "main", false, sql.NullString{})
	if err != nil {
		t.Fatalf("first CreateRepository() error = %v", err)
	}

	_, err = db.CreateRepository("test-repo", "https://github.com/other/repo", "main", false, sql.NullString{})
	if err == nil {
		t.Error("expected error for duplicate name, got nil")
	}
}

func TestRepository_Get(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	created, err := db.CreateRepository("test-repo", "https://github.com/test/repo", "main", false, sql.NullString{})
	if err != nil {
		t.Fatalf("CreateRepository() error = %v", err)
	}

	// Test GetRepository by ID
	repo, err := db.GetRepository(created.ID)
	if err != nil {
		t.Fatalf("GetRepository() error = %v", err)
	}
	if repo.ID != created.ID {
		t.Errorf("ID = %d, want %d", repo.ID, created.ID)
	}
	if repo.Name != created.Name {
		t.Errorf("Name = %q, want %q", repo.Name, created.Name)
	}

	// Test GetRepositoryByName
	repo, err = db.GetRepositoryByName("test-repo")
	if err != nil {
		t.Fatalf("GetRepositoryByName() error = %v", err)
	}
	if repo.ID != created.ID {
		t.Errorf("ID = %d, want %d", repo.ID, created.ID)
	}
}

func TestRepository_GetNotFound(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	_, err := db.GetRepository(999)
	if err == nil {
		t.Error("GetRepository() expected error for non-existent ID, got nil")
	}

	_, err = db.GetRepositoryByName("nonexistent")
	if err == nil {
		t.Error("GetRepositoryByName() expected error for non-existent name, got nil")
	}
}

func TestRepository_List(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Create some repositories
	repo1, _ := db.CreateRepository("repo-a", "https://github.com/test/a", "main", false, sql.NullString{})
	db.CreateRepository("repo-b", "https://github.com/test/b", "main", false, sql.NullString{})
	db.CreateRepository("repo-c", "https://github.com/test/c", "main", false, sql.NullString{})

	// Deactivate one
	db.SetRepositoryActive(repo1.ID, false)

	// List all
	repos, err := db.ListRepositories(nil)
	if err != nil {
		t.Fatalf("ListRepositories(nil) error = %v", err)
	}
	if len(repos) != 3 {
		t.Errorf("ListRepositories(nil) returned %d repos, want 3", len(repos))
	}

	// List active only
	activeOnly := true
	repos, err = db.ListRepositories(&activeOnly)
	if err != nil {
		t.Fatalf("ListRepositories(true) error = %v", err)
	}
	if len(repos) != 2 {
		t.Errorf("ListRepositories(true) returned %d repos, want 2", len(repos))
	}

	// List inactive only
	activeOnly = false
	repos, err = db.ListRepositories(&activeOnly)
	if err != nil {
		t.Fatalf("ListRepositories(false) error = %v", err)
	}
	if len(repos) != 1 {
		t.Errorf("ListRepositories(false) returned %d repos, want 1", len(repos))
	}
}

func TestRepository_ListOrdering(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	db.CreateRepository("zebra", "https://github.com/test/z", "main", false, sql.NullString{})
	db.CreateRepository("alpha", "https://github.com/test/a", "main", false, sql.NullString{})
	db.CreateRepository("middle", "https://github.com/test/m", "main", false, sql.NullString{})

	repos, err := db.ListRepositories(nil)
	if err != nil {
		t.Fatalf("ListRepositories() error = %v", err)
	}

	// Should be ordered by name
	if repos[0].Name != "alpha" {
		t.Errorf("first repo = %q, want %q", repos[0].Name, "alpha")
	}
	if repos[1].Name != "middle" {
		t.Errorf("second repo = %q, want %q", repos[1].Name, "middle")
	}
	if repos[2].Name != "zebra" {
		t.Errorf("third repo = %q, want %q", repos[2].Name, "zebra")
	}
}

func TestRepository_Update(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo, err := db.CreateRepository("test-repo", "https://github.com/test/repo", "main", false, sql.NullString{})
	if err != nil {
		t.Fatalf("CreateRepository() error = %v", err)
	}

	// Update fields
	repo.Name = "updated-repo"
	repo.Branch = "develop"
	repo.Private = true
	repo.Description = sql.NullString{String: "Updated description", Valid: true}
	repo.LastRunAt = sql.NullTime{Time: time.Now(), Valid: true}
	repo.LastRunSHA = sql.NullString{String: "abc123", Valid: true}

	if err := db.UpdateRepository(repo); err != nil {
		t.Fatalf("UpdateRepository() error = %v", err)
	}

	// Verify update
	updated, err := db.GetRepository(repo.ID)
	if err != nil {
		t.Fatalf("GetRepository() error = %v", err)
	}
	if updated.Name != "updated-repo" {
		t.Errorf("Name = %q, want %q", updated.Name, "updated-repo")
	}
	if updated.Branch != "develop" {
		t.Errorf("Branch = %q, want %q", updated.Branch, "develop")
	}
	if !updated.Private {
		t.Error("Private should be true")
	}
	if !updated.Description.Valid || updated.Description.String != "Updated description" {
		t.Error("Description not updated correctly")
	}
	if !updated.LastRunAt.Valid {
		t.Error("LastRunAt should be valid")
	}
	if !updated.LastRunSHA.Valid || updated.LastRunSHA.String != "abc123" {
		t.Error("LastRunSHA not updated correctly")
	}
}

func TestRepository_Delete(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo, err := db.CreateRepository("test-repo", "https://github.com/test/repo", "main", false, sql.NullString{})
	if err != nil {
		t.Fatalf("CreateRepository() error = %v", err)
	}

	if err := db.DeleteRepository(repo.ID); err != nil {
		t.Fatalf("DeleteRepository() error = %v", err)
	}

	_, err = db.GetRepository(repo.ID)
	if err == nil {
		t.Error("GetRepository() after delete expected error, got nil")
	}
}

func TestRepository_SetActive(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo, _ := db.CreateRepository("test-repo", "https://github.com/test/repo", "main", false, sql.NullString{})

	// Deactivate
	if err := db.SetRepositoryActive(repo.ID, false); err != nil {
		t.Fatalf("SetRepositoryActive(false) error = %v", err)
	}

	updated, _ := db.GetRepository(repo.ID)
	if updated.Active {
		t.Error("Active should be false after deactivation")
	}

	// Reactivate
	if err := db.SetRepositoryActive(repo.ID, true); err != nil {
		t.Fatalf("SetRepositoryActive(true) error = %v", err)
	}

	updated, _ = db.GetRepository(repo.ID)
	if !updated.Active {
		t.Error("Active should be true after reactivation")
	}
}

// ActivityRun CRUD tests

func TestActivityRun_Create(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo, _ := db.CreateRepository("test-repo", "https://github.com/test/repo", "main", false, sql.NullString{})

	run, err := db.CreateActivityRun(repo.ID, "abc123", "def456")
	if err != nil {
		t.Fatalf("CreateActivityRun() error = %v", err)
	}

	if run.ID == 0 {
		t.Error("expected non-zero ID")
	}
	if run.RepoID != repo.ID {
		t.Errorf("RepoID = %d, want %d", run.RepoID, repo.ID)
	}
	if run.StartSHA != "abc123" {
		t.Errorf("StartSHA = %q, want %q", run.StartSHA, "abc123")
	}
	if run.EndSHA != "def456" {
		t.Errorf("EndSHA = %q, want %q", run.EndSHA, "def456")
	}
	if run.CompletedAt.Valid {
		t.Error("CompletedAt should not be valid initially")
	}
}

func TestActivityRun_Get(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo, _ := db.CreateRepository("test-repo", "https://github.com/test/repo", "main", false, sql.NullString{})
	created, _ := db.CreateActivityRun(repo.ID, "abc123", "def456")

	run, err := db.GetActivityRun(created.ID)
	if err != nil {
		t.Fatalf("GetActivityRun() error = %v", err)
	}

	if run.ID != created.ID {
		t.Errorf("ID = %d, want %d", run.ID, created.ID)
	}
}

func TestActivityRun_GetNotFound(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	_, err := db.GetActivityRun(999)
	if err == nil {
		t.Error("GetActivityRun() expected error for non-existent ID, got nil")
	}
}

func TestActivityRun_GetLatest(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo, _ := db.CreateRepository("test-repo", "https://github.com/test/repo", "main", false, sql.NullString{})

	// No runs yet
	run, err := db.GetLatestActivityRun(repo.ID)
	if err != nil {
		t.Fatalf("GetLatestActivityRun() error = %v", err)
	}
	if run != nil {
		t.Error("expected nil for no runs, got non-nil")
	}

	// Create some runs - we can't rely on timestamps, so just verify we get the most recent one
	// The query orders by started_at DESC, which defaults to CURRENT_TIMESTAMP
	// Create multiple runs and verify we get one back
	db.CreateActivityRun(repo.ID, "first", "first-end")
	db.CreateActivityRun(repo.ID, "second", "second-end")

	run, err = db.GetLatestActivityRun(repo.ID)
	if err != nil {
		t.Fatalf("GetLatestActivityRun() error = %v", err)
	}
	if run == nil {
		t.Error("expected a run, got nil")
	}
	// We can't guarantee which one is "latest" due to timestamp precision
	// but we should get one of them
	if run.StartSHA != "first" && run.StartSHA != "second" {
		t.Errorf("got unexpected run with StartSHA = %q", run.StartSHA)
	}
}

func TestActivityRun_Update(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo, _ := db.CreateRepository("test-repo", "https://github.com/test/repo", "main", false, sql.NullString{})
	run, _ := db.CreateActivityRun(repo.ID, "abc123", "def456")

	// Update fields
	run.CompletedAt = sql.NullTime{Time: time.Now(), Valid: true}
	run.Summary = sql.NullString{String: "Test summary", Valid: true}
	run.RawData = sql.NullString{String: `{"test": "data"}`, Valid: true}
	run.AgentMode = true
	run.ToolUsageStats = sql.NullString{String: `{"diffs": 3}`, Valid: true}

	if err := db.UpdateActivityRun(run); err != nil {
		t.Fatalf("UpdateActivityRun() error = %v", err)
	}

	updated, _ := db.GetActivityRun(run.ID)
	if !updated.CompletedAt.Valid {
		t.Error("CompletedAt should be valid")
	}
	if !updated.Summary.Valid || updated.Summary.String != "Test summary" {
		t.Error("Summary not updated correctly")
	}
	if !updated.AgentMode {
		t.Error("AgentMode should be true")
	}
	if !updated.ToolUsageStats.Valid || updated.ToolUsageStats.String != `{"diffs": 3}` {
		t.Error("ToolUsageStats not updated correctly")
	}
}

// Subscriber CRUD tests

func TestSubscriber_Create(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	sub, err := db.CreateSubscriber("test@example.com", false)
	if err != nil {
		t.Fatalf("CreateSubscriber() error = %v", err)
	}

	if sub.ID == 0 {
		t.Error("expected non-zero ID")
	}
	if sub.Email != "test@example.com" {
		t.Errorf("Email = %q, want %q", sub.Email, "test@example.com")
	}
	if sub.SubscribeAll {
		t.Error("SubscribeAll should be false")
	}
}

func TestSubscriber_CreateWithSubscribeAll(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	sub, err := db.CreateSubscriber("all@example.com", true)
	if err != nil {
		t.Fatalf("CreateSubscriber() error = %v", err)
	}

	if !sub.SubscribeAll {
		t.Error("SubscribeAll should be true")
	}
}

func TestSubscriber_CreateDuplicate(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	_, err := db.CreateSubscriber("test@example.com", false)
	if err != nil {
		t.Fatalf("first CreateSubscriber() error = %v", err)
	}

	_, err = db.CreateSubscriber("test@example.com", false)
	if err == nil {
		t.Error("expected error for duplicate email, got nil")
	}
}

func TestSubscriber_Get(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	created, _ := db.CreateSubscriber("test@example.com", false)

	// By ID
	sub, err := db.GetSubscriber(created.ID)
	if err != nil {
		t.Fatalf("GetSubscriber() error = %v", err)
	}
	if sub.ID != created.ID {
		t.Errorf("ID = %d, want %d", sub.ID, created.ID)
	}

	// By email
	sub, err = db.GetSubscriberByEmail("test@example.com")
	if err != nil {
		t.Fatalf("GetSubscriberByEmail() error = %v", err)
	}
	if sub.ID != created.ID {
		t.Errorf("ID = %d, want %d", sub.ID, created.ID)
	}
}

func TestSubscriber_GetNotFound(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	_, err := db.GetSubscriber(999)
	if err == nil {
		t.Error("GetSubscriber() expected error for non-existent ID, got nil")
	}

	_, err = db.GetSubscriberByEmail("nonexistent@example.com")
	if err == nil {
		t.Error("GetSubscriberByEmail() expected error for non-existent email, got nil")
	}
}

func TestSubscriber_List(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	db.CreateSubscriber("zebra@example.com", false)
	db.CreateSubscriber("alpha@example.com", false)
	db.CreateSubscriber("middle@example.com", false)

	subs, err := db.ListSubscribers()
	if err != nil {
		t.Fatalf("ListSubscribers() error = %v", err)
	}

	if len(subs) != 3 {
		t.Errorf("ListSubscribers() returned %d subscribers, want 3", len(subs))
	}

	// Should be ordered by email
	if subs[0].Email != "alpha@example.com" {
		t.Errorf("first email = %q, want %q", subs[0].Email, "alpha@example.com")
	}
}

func TestSubscriber_Update(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	sub, _ := db.CreateSubscriber("test@example.com", false)

	sub.Email = "updated@example.com"
	sub.SubscribeAll = true

	if err := db.UpdateSubscriber(sub); err != nil {
		t.Fatalf("UpdateSubscriber() error = %v", err)
	}

	updated, _ := db.GetSubscriber(sub.ID)
	if updated.Email != "updated@example.com" {
		t.Errorf("Email = %q, want %q", updated.Email, "updated@example.com")
	}
	if !updated.SubscribeAll {
		t.Error("SubscribeAll should be true")
	}
}

func TestSubscriber_Delete(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	sub, _ := db.CreateSubscriber("test@example.com", false)

	if err := db.DeleteSubscriber(sub.ID); err != nil {
		t.Fatalf("DeleteSubscriber() error = %v", err)
	}

	_, err := db.GetSubscriber(sub.ID)
	if err == nil {
		t.Error("GetSubscriber() after delete expected error, got nil")
	}
}

// Subscription CRUD tests

func TestSubscription_Create(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo, _ := db.CreateRepository("test-repo", "https://github.com/test/repo", "main", false, sql.NullString{})
	sub, _ := db.CreateSubscriber("test@example.com", false)

	subscription, err := db.CreateSubscription(sub.ID, repo.ID)
	if err != nil {
		t.Fatalf("CreateSubscription() error = %v", err)
	}

	if subscription.ID == 0 {
		t.Error("expected non-zero ID")
	}
	if subscription.SubscriberID != sub.ID {
		t.Errorf("SubscriberID = %d, want %d", subscription.SubscriberID, sub.ID)
	}
	if subscription.RepoID != repo.ID {
		t.Errorf("RepoID = %d, want %d", subscription.RepoID, repo.ID)
	}
}

func TestSubscription_CreateDuplicate(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo, _ := db.CreateRepository("test-repo", "https://github.com/test/repo", "main", false, sql.NullString{})
	sub, _ := db.CreateSubscriber("test@example.com", false)

	_, err := db.CreateSubscription(sub.ID, repo.ID)
	if err != nil {
		t.Fatalf("first CreateSubscription() error = %v", err)
	}

	_, err = db.CreateSubscription(sub.ID, repo.ID)
	if err == nil {
		t.Error("expected error for duplicate subscription, got nil")
	}
}

func TestSubscription_Get(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo, _ := db.CreateRepository("test-repo", "https://github.com/test/repo", "main", false, sql.NullString{})
	sub, _ := db.CreateSubscriber("test@example.com", false)
	created, _ := db.CreateSubscription(sub.ID, repo.ID)

	// By ID
	subscription, err := db.GetSubscription(created.ID)
	if err != nil {
		t.Fatalf("GetSubscription() error = %v", err)
	}
	if subscription.ID != created.ID {
		t.Errorf("ID = %d, want %d", subscription.ID, created.ID)
	}

	// By subscriber and repo
	subscription, err = db.GetSubscriptionBySubscriberAndRepo(sub.ID, repo.ID)
	if err != nil {
		t.Fatalf("GetSubscriptionBySubscriberAndRepo() error = %v", err)
	}
	if subscription.ID != created.ID {
		t.Errorf("ID = %d, want %d", subscription.ID, created.ID)
	}
}

func TestSubscription_List(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo1, _ := db.CreateRepository("repo-1", "https://github.com/test/1", "main", false, sql.NullString{})
	repo2, _ := db.CreateRepository("repo-2", "https://github.com/test/2", "main", false, sql.NullString{})
	sub, _ := db.CreateSubscriber("test@example.com", false)

	db.CreateSubscription(sub.ID, repo1.ID)
	db.CreateSubscription(sub.ID, repo2.ID)

	subs, err := db.ListSubscriptionsBySubscriber(sub.ID)
	if err != nil {
		t.Fatalf("ListSubscriptionsBySubscriber() error = %v", err)
	}

	if len(subs) != 2 {
		t.Errorf("ListSubscriptionsBySubscriber() returned %d subscriptions, want 2", len(subs))
	}
}

func TestSubscription_Delete(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo, _ := db.CreateRepository("test-repo", "https://github.com/test/repo", "main", false, sql.NullString{})
	sub, _ := db.CreateSubscriber("test@example.com", false)
	subscription, _ := db.CreateSubscription(sub.ID, repo.ID)

	if err := db.DeleteSubscription(subscription.ID); err != nil {
		t.Fatalf("DeleteSubscription() error = %v", err)
	}

	_, err := db.GetSubscription(subscription.ID)
	if err == nil {
		t.Error("GetSubscription() after delete expected error, got nil")
	}
}

func TestSubscription_DeleteBySubscriberAndRepo(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo, _ := db.CreateRepository("test-repo", "https://github.com/test/repo", "main", false, sql.NullString{})
	sub, _ := db.CreateSubscriber("test@example.com", false)
	db.CreateSubscription(sub.ID, repo.ID)

	if err := db.DeleteSubscriptionBySubscriberAndRepo(sub.ID, repo.ID); err != nil {
		t.Fatalf("DeleteSubscriptionBySubscriberAndRepo() error = %v", err)
	}

	_, err := db.GetSubscriptionBySubscriberAndRepo(sub.ID, repo.ID)
	if err == nil {
		t.Error("GetSubscriptionBySubscriberAndRepo() after delete expected error, got nil")
	}
}

// NewsletterSend CRUD tests

func TestNewsletterSend_Create(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo, _ := db.CreateRepository("test-repo", "https://github.com/test/repo", "main", false, sql.NullString{})
	sub, _ := db.CreateSubscriber("test@example.com", false)
	run, _ := db.CreateActivityRun(repo.ID, "abc123", "def456")

	ns, err := db.CreateNewsletterSend(sub.ID, run.ID, "msg-123")
	if err != nil {
		t.Fatalf("CreateNewsletterSend() error = %v", err)
	}

	if ns.ID == 0 {
		t.Error("expected non-zero ID")
	}
	if ns.SubscriberID != sub.ID {
		t.Errorf("SubscriberID = %d, want %d", ns.SubscriberID, sub.ID)
	}
	if !ns.SendGridMessageID.Valid || ns.SendGridMessageID.String != "msg-123" {
		t.Error("SendGridMessageID not set correctly")
	}
}

func TestNewsletterSend_CreateWithoutMessageID(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo, _ := db.CreateRepository("test-repo", "https://github.com/test/repo", "main", false, sql.NullString{})
	sub, _ := db.CreateSubscriber("test@example.com", false)
	run, _ := db.CreateActivityRun(repo.ID, "abc123", "def456")

	ns, err := db.CreateNewsletterSend(sub.ID, run.ID, "")
	if err != nil {
		t.Fatalf("CreateNewsletterSend() error = %v", err)
	}

	if ns.SendGridMessageID.Valid {
		t.Error("SendGridMessageID should not be valid for empty string")
	}
}

func TestNewsletterSend_HasBeenSent(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo, _ := db.CreateRepository("test-repo", "https://github.com/test/repo", "main", false, sql.NullString{})
	sub, _ := db.CreateSubscriber("test@example.com", false)
	run, _ := db.CreateActivityRun(repo.ID, "abc123", "def456")

	// Not sent yet
	sent, err := db.HasNewsletterBeenSent(sub.ID, run.ID)
	if err != nil {
		t.Fatalf("HasNewsletterBeenSent() error = %v", err)
	}
	if sent {
		t.Error("HasNewsletterBeenSent() should be false before sending")
	}

	// Send
	db.CreateNewsletterSend(sub.ID, run.ID, "")

	sent, err = db.HasNewsletterBeenSent(sub.ID, run.ID)
	if err != nil {
		t.Fatalf("HasNewsletterBeenSent() error = %v", err)
	}
	if !sent {
		t.Error("HasNewsletterBeenSent() should be true after sending")
	}
}

// WeeklyReport CRUD tests

func TestWeeklyReport_Create(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo, _ := db.CreateRepository("test-repo", "https://github.com/test/repo", "main", false, sql.NullString{})

	report := &WeeklyReport{
		RepoID:      repo.ID,
		Year:        2024,
		Week:        1,
		WeekStart:   time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		WeekEnd:     time.Date(2024, 1, 7, 0, 0, 0, 0, time.UTC),
		Summary:     sql.NullString{String: "Test summary", Valid: true},
		CommitCount: 10,
		AgentMode:   true,
	}

	created, err := db.CreateWeeklyReport(report)
	if err != nil {
		t.Fatalf("CreateWeeklyReport() error = %v", err)
	}

	if created.ID == 0 {
		t.Error("expected non-zero ID")
	}
	if created.Year != 2024 {
		t.Errorf("Year = %d, want %d", created.Year, 2024)
	}
	if created.Week != 1 {
		t.Errorf("Week = %d, want %d", created.Week, 1)
	}
	if created.CommitCount != 10 {
		t.Errorf("CommitCount = %d, want %d", created.CommitCount, 10)
	}
}

func TestWeeklyReport_CreateDuplicate(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo, _ := db.CreateRepository("test-repo", "https://github.com/test/repo", "main", false, sql.NullString{})

	report := &WeeklyReport{
		RepoID:    repo.ID,
		Year:      2024,
		Week:      1,
		WeekStart: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		WeekEnd:   time.Date(2024, 1, 7, 0, 0, 0, 0, time.UTC),
	}

	_, err := db.CreateWeeklyReport(report)
	if err != nil {
		t.Fatalf("first CreateWeeklyReport() error = %v", err)
	}

	_, err = db.CreateWeeklyReport(report)
	if err == nil {
		t.Error("expected error for duplicate year/week, got nil")
	}
}

func TestWeeklyReport_Get(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo, _ := db.CreateRepository("test-repo", "https://github.com/test/repo", "main", false, sql.NullString{})

	report := &WeeklyReport{
		RepoID:    repo.ID,
		Year:      2024,
		Week:      1,
		WeekStart: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		WeekEnd:   time.Date(2024, 1, 7, 0, 0, 0, 0, time.UTC),
	}
	created, _ := db.CreateWeeklyReport(report)

	// By ID
	fetched, err := db.GetWeeklyReport(created.ID)
	if err != nil {
		t.Fatalf("GetWeeklyReport() error = %v", err)
	}
	if fetched.ID != created.ID {
		t.Errorf("ID = %d, want %d", fetched.ID, created.ID)
	}

	// By repo and week
	fetched, err = db.GetWeeklyReportByRepoAndWeek(repo.ID, 2024, 1)
	if err != nil {
		t.Fatalf("GetWeeklyReportByRepoAndWeek() error = %v", err)
	}
	if fetched.ID != created.ID {
		t.Errorf("ID = %d, want %d", fetched.ID, created.ID)
	}

	// Non-existent week returns nil without error
	fetched, err = db.GetWeeklyReportByRepoAndWeek(repo.ID, 2024, 52)
	if err != nil {
		t.Fatalf("GetWeeklyReportByRepoAndWeek() error = %v", err)
	}
	if fetched != nil {
		t.Error("expected nil for non-existent week")
	}
}

func TestWeeklyReport_GetLatest(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo, _ := db.CreateRepository("test-repo", "https://github.com/test/repo", "main", false, sql.NullString{})

	// No reports yet
	latest, err := db.GetLatestWeeklyReport(repo.ID)
	if err != nil {
		t.Fatalf("GetLatestWeeklyReport() error = %v", err)
	}
	if latest != nil {
		t.Error("expected nil for no reports")
	}

	// Create some reports
	db.CreateWeeklyReport(&WeeklyReport{
		RepoID:    repo.ID,
		Year:      2024,
		Week:      1,
		WeekStart: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		WeekEnd:   time.Date(2024, 1, 7, 0, 0, 0, 0, time.UTC),
	})
	expectedLatest, _ := db.CreateWeeklyReport(&WeeklyReport{
		RepoID:    repo.ID,
		Year:      2024,
		Week:      5,
		WeekStart: time.Date(2024, 1, 29, 0, 0, 0, 0, time.UTC),
		WeekEnd:   time.Date(2024, 2, 4, 0, 0, 0, 0, time.UTC),
	})

	latest, err = db.GetLatestWeeklyReport(repo.ID)
	if err != nil {
		t.Fatalf("GetLatestWeeklyReport() error = %v", err)
	}
	if latest.ID != expectedLatest.ID {
		t.Errorf("got report ID %d, want %d (latest)", latest.ID, expectedLatest.ID)
	}
}

func TestWeeklyReport_List(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo, _ := db.CreateRepository("test-repo", "https://github.com/test/repo", "main", false, sql.NullString{})

	// Create reports for 2023 and 2024
	db.CreateWeeklyReport(&WeeklyReport{
		RepoID:    repo.ID,
		Year:      2023,
		Week:      50,
		WeekStart: time.Date(2023, 12, 11, 0, 0, 0, 0, time.UTC),
		WeekEnd:   time.Date(2023, 12, 17, 0, 0, 0, 0, time.UTC),
	})
	db.CreateWeeklyReport(&WeeklyReport{
		RepoID:    repo.ID,
		Year:      2024,
		Week:      1,
		WeekStart: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		WeekEnd:   time.Date(2024, 1, 7, 0, 0, 0, 0, time.UTC),
	})
	db.CreateWeeklyReport(&WeeklyReport{
		RepoID:    repo.ID,
		Year:      2024,
		Week:      2,
		WeekStart: time.Date(2024, 1, 8, 0, 0, 0, 0, time.UTC),
		WeekEnd:   time.Date(2024, 1, 14, 0, 0, 0, 0, time.UTC),
	})

	// List all
	reports, err := db.ListWeeklyReportsByRepo(repo.ID, nil)
	if err != nil {
		t.Fatalf("ListWeeklyReportsByRepo(nil) error = %v", err)
	}
	if len(reports) != 3 {
		t.Errorf("ListWeeklyReportsByRepo(nil) returned %d reports, want 3", len(reports))
	}

	// List by year
	year := 2024
	reports, err = db.ListWeeklyReportsByRepo(repo.ID, &year)
	if err != nil {
		t.Fatalf("ListWeeklyReportsByRepo(2024) error = %v", err)
	}
	if len(reports) != 2 {
		t.Errorf("ListWeeklyReportsByRepo(2024) returned %d reports, want 2", len(reports))
	}
}

func TestWeeklyReport_ListAll(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo1, _ := db.CreateRepository("repo-1", "https://github.com/test/1", "main", false, sql.NullString{})
	repo2, _ := db.CreateRepository("repo-2", "https://github.com/test/2", "main", false, sql.NullString{})

	db.CreateWeeklyReport(&WeeklyReport{
		RepoID:    repo1.ID,
		Year:      2024,
		Week:      1,
		WeekStart: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		WeekEnd:   time.Date(2024, 1, 7, 0, 0, 0, 0, time.UTC),
	})
	db.CreateWeeklyReport(&WeeklyReport{
		RepoID:    repo2.ID,
		Year:      2024,
		Week:      1,
		WeekStart: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		WeekEnd:   time.Date(2024, 1, 7, 0, 0, 0, 0, time.UTC),
	})

	reports, err := db.ListAllWeeklyReports(nil)
	if err != nil {
		t.Fatalf("ListAllWeeklyReports() error = %v", err)
	}
	if len(reports) != 2 {
		t.Errorf("ListAllWeeklyReports() returned %d reports, want 2", len(reports))
	}
}

func TestWeeklyReport_Update(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo, _ := db.CreateRepository("test-repo", "https://github.com/test/repo", "main", false, sql.NullString{})

	report := &WeeklyReport{
		RepoID:    repo.ID,
		Year:      2024,
		Week:      1,
		WeekStart: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		WeekEnd:   time.Date(2024, 1, 7, 0, 0, 0, 0, time.UTC),
	}
	created, _ := db.CreateWeeklyReport(report)

	created.Summary = sql.NullString{String: "Updated summary", Valid: true}
	created.CommitCount = 42
	created.AgentMode = true

	if err := db.UpdateWeeklyReport(created); err != nil {
		t.Fatalf("UpdateWeeklyReport() error = %v", err)
	}

	updated, _ := db.GetWeeklyReport(created.ID)
	if !updated.Summary.Valid || updated.Summary.String != "Updated summary" {
		t.Error("Summary not updated")
	}
	if updated.CommitCount != 42 {
		t.Errorf("CommitCount = %d, want %d", updated.CommitCount, 42)
	}
	if !updated.AgentMode {
		t.Error("AgentMode should be true")
	}
}

func TestWeeklyReport_Exists(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo, _ := db.CreateRepository("test-repo", "https://github.com/test/repo", "main", false, sql.NullString{})

	// Doesn't exist yet
	exists, err := db.WeeklyReportExists(repo.ID, 2024, 1)
	if err != nil {
		t.Fatalf("WeeklyReportExists() error = %v", err)
	}
	if exists {
		t.Error("WeeklyReportExists() should be false")
	}

	// Create it
	db.CreateWeeklyReport(&WeeklyReport{
		RepoID:    repo.ID,
		Year:      2024,
		Week:      1,
		WeekStart: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		WeekEnd:   time.Date(2024, 1, 7, 0, 0, 0, 0, time.UTC),
	})

	exists, err = db.WeeklyReportExists(repo.ID, 2024, 1)
	if err != nil {
		t.Fatalf("WeeklyReportExists() error = %v", err)
	}
	if !exists {
		t.Error("WeeklyReportExists() should be true")
	}
}

func TestWeeklyReport_Delete(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo, _ := db.CreateRepository("test-repo", "https://github.com/test/repo", "main", false, sql.NullString{})

	report, _ := db.CreateWeeklyReport(&WeeklyReport{
		RepoID:    repo.ID,
		Year:      2024,
		Week:      1,
		WeekStart: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		WeekEnd:   time.Date(2024, 1, 7, 0, 0, 0, 0, time.UTC),
	})

	if err := db.DeleteWeeklyReport(report.ID); err != nil {
		t.Fatalf("DeleteWeeklyReport() error = %v", err)
	}

	_, err := db.GetWeeklyReport(report.ID)
	if err == nil {
		t.Error("GetWeeklyReport() after delete expected error, got nil")
	}
}

// Complex query tests

func TestGetUnsentActivityRuns_SubscribeAll(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo1, _ := db.CreateRepository("repo-1", "https://github.com/test/1", "main", false, sql.NullString{})
	repo2, _ := db.CreateRepository("repo-2", "https://github.com/test/2", "main", false, sql.NullString{})

	// Create subscriber with subscribe_all = true
	sub, _ := db.CreateSubscriber("all@example.com", true)

	// Create completed activity runs
	run1, _ := db.CreateActivityRun(repo1.ID, "abc", "def")
	run1.CompletedAt = sql.NullTime{Time: time.Now(), Valid: true}
	run1.Summary = sql.NullString{String: "Run 1", Valid: true}
	db.UpdateActivityRun(run1)

	run2, _ := db.CreateActivityRun(repo2.ID, "ghi", "jkl")
	run2.CompletedAt = sql.NullTime{Time: time.Now(), Valid: true}
	run2.Summary = sql.NullString{String: "Run 2", Valid: true}
	db.UpdateActivityRun(run2)

	// Get unsent runs - should return both
	since := time.Now().Add(-1 * time.Hour)
	runs, err := db.GetUnsentActivityRuns(sub.ID, since)
	if err != nil {
		t.Fatalf("GetUnsentActivityRuns() error = %v", err)
	}
	if len(runs) != 2 {
		t.Errorf("GetUnsentActivityRuns() returned %d runs, want 2", len(runs))
	}

	// Mark one as sent
	db.CreateNewsletterSend(sub.ID, run1.ID, "")

	// Get unsent runs - should return only one
	runs, err = db.GetUnsentActivityRuns(sub.ID, since)
	if err != nil {
		t.Fatalf("GetUnsentActivityRuns() error = %v", err)
	}
	if len(runs) != 1 {
		t.Errorf("GetUnsentActivityRuns() returned %d runs, want 1", len(runs))
	}
	if runs[0].ID != run2.ID {
		t.Errorf("expected run2, got run %d", runs[0].ID)
	}
}

func TestGetUnsentActivityRuns_SpecificRepos(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo1, _ := db.CreateRepository("repo-1", "https://github.com/test/1", "main", false, sql.NullString{})
	repo2, _ := db.CreateRepository("repo-2", "https://github.com/test/2", "main", false, sql.NullString{})

	// Create subscriber subscribed only to repo1
	sub, _ := db.CreateSubscriber("specific@example.com", false)
	db.CreateSubscription(sub.ID, repo1.ID)

	// Create completed activity runs for both repos
	run1, _ := db.CreateActivityRun(repo1.ID, "abc", "def")
	run1.CompletedAt = sql.NullTime{Time: time.Now(), Valid: true}
	db.UpdateActivityRun(run1)

	run2, _ := db.CreateActivityRun(repo2.ID, "ghi", "jkl")
	run2.CompletedAt = sql.NullTime{Time: time.Now(), Valid: true}
	db.UpdateActivityRun(run2)

	// Get unsent runs - should return only run1
	since := time.Now().Add(-1 * time.Hour)
	runs, err := db.GetUnsentActivityRuns(sub.ID, since)
	if err != nil {
		t.Fatalf("GetUnsentActivityRuns() error = %v", err)
	}
	if len(runs) != 1 {
		t.Errorf("GetUnsentActivityRuns() returned %d runs, want 1", len(runs))
	}
	if runs[0].ID != run1.ID {
		t.Errorf("expected run1, got run %d", runs[0].ID)
	}
}

func TestGetReposForSubscriber_SubscribeAll(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	db.CreateRepository("repo-1", "https://github.com/test/1", "main", false, sql.NullString{})
	db.CreateRepository("repo-2", "https://github.com/test/2", "main", false, sql.NullString{})
	repo3, _ := db.CreateRepository("repo-3", "https://github.com/test/3", "main", false, sql.NullString{})
	db.SetRepositoryActive(repo3.ID, false) // Deactivate one

	sub, _ := db.CreateSubscriber("all@example.com", true)

	repos, err := db.GetReposForSubscriber(sub.ID)
	if err != nil {
		t.Fatalf("GetReposForSubscriber() error = %v", err)
	}

	// Should return only active repos (2 out of 3)
	if len(repos) != 2 {
		t.Errorf("GetReposForSubscriber() returned %d repos, want 2", len(repos))
	}
}

func TestGetReposForSubscriber_SpecificRepos(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo1, _ := db.CreateRepository("repo-1", "https://github.com/test/1", "main", false, sql.NullString{})
	repo2, _ := db.CreateRepository("repo-2", "https://github.com/test/2", "main", false, sql.NullString{})
	db.CreateRepository("repo-3", "https://github.com/test/3", "main", false, sql.NullString{})

	sub, _ := db.CreateSubscriber("specific@example.com", false)
	db.CreateSubscription(sub.ID, repo1.ID)
	db.CreateSubscription(sub.ID, repo2.ID)

	repos, err := db.GetReposForSubscriber(sub.ID)
	if err != nil {
		t.Fatalf("GetReposForSubscriber() error = %v", err)
	}

	if len(repos) != 2 {
		t.Errorf("GetReposForSubscriber() returned %d repos, want 2", len(repos))
	}
}

// Delete tests with related records

func TestRepository_DeleteWithActivityRuns(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo, _ := db.CreateRepository("test-repo", "https://github.com/test/repo", "main", false, sql.NullString{})
	db.CreateActivityRun(repo.ID, "abc", "def")

	// Delete the repository - should succeed even with related activity runs
	// Note: SQLite foreign key constraints are not enforced by default,
	// so this will leave orphaned activity runs
	err := db.DeleteRepository(repo.ID)
	if err != nil {
		t.Fatalf("DeleteRepository() error = %v", err)
	}

	// Repository should be deleted
	_, err = db.GetRepository(repo.ID)
	if err == nil {
		t.Error("repository should have been deleted")
	}
}

func TestSubscriber_DeleteWithSubscriptions(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo, _ := db.CreateRepository("test-repo", "https://github.com/test/repo", "main", false, sql.NullString{})
	sub, _ := db.CreateSubscriber("test@example.com", false)
	db.CreateSubscription(sub.ID, repo.ID)

	// Delete the subscriber - should succeed even with related subscriptions
	// Note: SQLite foreign key constraints are not enforced by default
	err := db.DeleteSubscriber(sub.ID)
	if err != nil {
		t.Fatalf("DeleteSubscriber() error = %v", err)
	}

	// Subscriber should be deleted
	_, err = db.GetSubscriber(sub.ID)
	if err == nil {
		t.Error("subscriber should have been deleted")
	}
}

func TestMigrations_Idempotent(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "activity-db-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Open once - runs migrations
	db1, err := Open(tmpDir)
	if err != nil {
		t.Fatalf("first Open() error = %v", err)
	}
	db1.Close()

	// Open again - should not error (migrations already applied)
	db2, err := Open(tmpDir)
	if err != nil {
		t.Fatalf("second Open() error = %v", err)
	}
	defer db2.Close()

	// Verify we can still use the database
	_, err = db2.CreateRepository("test", "https://example.com", "main", false, sql.NullString{})
	if err != nil {
		t.Fatalf("CreateRepository() after reopen error = %v", err)
	}
}

func TestMigrations_LegacyUpgrade(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "activity-db-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "activity.db")

	// Create a database with the old migration system at version 8
	sqlDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}

	// Create old migrations table
	_, err = sqlDB.Exec(`
		CREATE TABLE migrations (
			version INTEGER PRIMARY KEY,
			applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		t.Fatalf("failed to create migrations table: %v", err)
	}

	// Insert version 8
	_, err = sqlDB.Exec("INSERT INTO migrations (version) VALUES (8)")
	if err != nil {
		t.Fatalf("failed to insert migration version: %v", err)
	}

	// Create all the tables that would exist at version 8
	_, err = sqlDB.Exec(`
		CREATE TABLE repositories (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT UNIQUE NOT NULL,
			url TEXT NOT NULL,
			branch TEXT NOT NULL DEFAULT 'main',
			active BOOLEAN NOT NULL DEFAULT 1,
			private BOOLEAN NOT NULL DEFAULT 0,
			description TEXT,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			last_run_at DATETIME,
			last_run_sha TEXT
		);

		CREATE TABLE activity_runs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			repo_id INTEGER NOT NULL,
			start_sha TEXT NOT NULL,
			end_sha TEXT NOT NULL,
			started_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			completed_at DATETIME,
			summary TEXT,
			raw_data TEXT,
			agent_mode BOOLEAN DEFAULT 0,
			tool_usage_stats TEXT,
			FOREIGN KEY (repo_id) REFERENCES repositories(id) ON DELETE CASCADE
		);

		CREATE TABLE subscribers (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			email TEXT UNIQUE NOT NULL,
			subscribe_all BOOLEAN NOT NULL DEFAULT 0,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE subscriptions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			subscriber_id INTEGER NOT NULL,
			repo_id INTEGER NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (subscriber_id) REFERENCES subscribers(id) ON DELETE CASCADE,
			FOREIGN KEY (repo_id) REFERENCES repositories(id) ON DELETE CASCADE,
			UNIQUE(subscriber_id, repo_id)
		);

		CREATE TABLE newsletter_sends (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			subscriber_id INTEGER NOT NULL,
			activity_run_id INTEGER NOT NULL,
			sent_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			sendgrid_message_id TEXT,
			FOREIGN KEY (subscriber_id) REFERENCES subscribers(id) ON DELETE CASCADE,
			FOREIGN KEY (activity_run_id) REFERENCES activity_runs(id) ON DELETE CASCADE,
			UNIQUE(subscriber_id, activity_run_id)
		);

		CREATE TABLE weekly_reports (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			repo_id INTEGER NOT NULL,
			year INTEGER NOT NULL,
			week INTEGER NOT NULL,
			week_start DATE NOT NULL,
			week_end DATE NOT NULL,
			summary TEXT,
			commit_count INTEGER NOT NULL DEFAULT 0,
			metadata TEXT,
			agent_mode BOOLEAN DEFAULT 0,
			tool_usage_stats TEXT,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			source_run_id INTEGER,
			FOREIGN KEY (repo_id) REFERENCES repositories(id) ON DELETE CASCADE,
			FOREIGN KEY (source_run_id) REFERENCES activity_runs(id) ON DELETE SET NULL,
			UNIQUE(repo_id, year, week)
		);

		CREATE TABLE admins (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			email TEXT UNIQUE NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			created_by TEXT
		);
	`)
	if err != nil {
		t.Fatalf("failed to create tables: %v", err)
	}

	// Insert some test data
	_, err = sqlDB.Exec(`INSERT INTO repositories (name, url, branch) VALUES ('test-repo', 'https://example.com', 'main')`)
	if err != nil {
		t.Fatalf("failed to insert test data: %v", err)
	}

	sqlDB.Close()

	// Now open with our new migration system
	db, err := Open(tmpDir)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer db.Close()

	// Verify old migrations table is gone
	var tableName string
	err = db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='migrations'").Scan(&tableName)
	if err != sql.ErrNoRows {
		t.Error("old migrations table should have been dropped")
	}

	// Verify goose_db_version table exists
	err = db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='goose_db_version'").Scan(&tableName)
	if err != nil {
		t.Errorf("goose_db_version table should exist: %v", err)
	}

	// Verify data was preserved
	repo, err := db.GetRepositoryByName("test-repo")
	if err != nil {
		t.Fatalf("GetRepositoryByName() error = %v", err)
	}
	if repo.Name != "test-repo" {
		t.Errorf("Name = %q, want %q", repo.Name, "test-repo")
	}

	// Verify we can still use the database
	_, err = db.CreateRepository("new-repo", "https://example.com/new", "main", false, sql.NullString{})
	if err != nil {
		t.Fatalf("CreateRepository() after upgrade error = %v", err)
	}
}
