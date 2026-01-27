#!/bin/bash
set -e

SQLITE_DB="./data/activity.db"
PG_DB="activity"
TMPDIR=$(mktemp -d)

cleanup() {
    rm -rf "$TMPDIR"
}
trap cleanup EXIT

# Function to clean Go timestamps
# Removes monotonic clock suffix and timezone names (keeps numeric offset)
# "2026-01-26 21:20:22.055535 +0100 CET m=+42.322218501" -> "2026-01-26 21:20:22.055535 +0100"
clean_timestamps() {
    sed -E 's/ (CET|CEST|UTC|EST|EDT|PST|PDT|[A-Z]{2,4}) m=\+[0-9.]*//g; s/ (CET|CEST|UTC|EST|EDT|PST|PDT|[A-Z]{2,4})(")/\2/g; s/ (CET|CEST|UTC|EST|EDT|PST|PDT|[A-Z]{2,4})(,)/\2/g'
}

echo "=== SQLite to PostgreSQL Migration ==="
echo "Source: $SQLITE_DB"
echo "Target: PostgreSQL database '$PG_DB'"
echo ""

# Check SQLite database exists
if [ ! -f "$SQLITE_DB" ]; then
    echo "ERROR: SQLite database not found at $SQLITE_DB"
    exit 1
fi

# Check PostgreSQL is accessible
if ! psql -d "$PG_DB" -c "SELECT 1" > /dev/null 2>&1; then
    echo "ERROR: Cannot connect to PostgreSQL database '$PG_DB'"
    exit 1
fi

echo "Step 1: Clearing PostgreSQL tables..."
psql -d "$PG_DB" -q <<'EOF'
TRUNCATE TABLE newsletter_sends, subscriptions, subscribers, weekly_reports, activity_runs, admins, repositories RESTART IDENTITY CASCADE;
EOF
echo "  Done."

echo "Step 2: Exporting from SQLite..."

# Export repositories with boolean conversion
sqlite3 -csv "$SQLITE_DB" "SELECT id, name, url, branch, CASE WHEN active THEN 'true' ELSE 'false' END, CASE WHEN private THEN 'true' ELSE 'false' END, description, created_at, updated_at, last_run_at, last_run_sha FROM repositories;" | clean_timestamps > "$TMPDIR/repositories.csv"

# Export admins
sqlite3 -csv "$SQLITE_DB" "SELECT id, email, created_at, created_by FROM admins;" | clean_timestamps > "$TMPDIR/admins.csv"

# Export activity_runs with boolean conversion
sqlite3 -csv "$SQLITE_DB" "SELECT id, repo_id, start_sha, end_sha, started_at, completed_at, summary, raw_data, CASE WHEN agent_mode THEN 'true' ELSE 'false' END, tool_usage_stats FROM activity_runs;" | clean_timestamps > "$TMPDIR/activity_runs.csv"

# Export weekly_reports with boolean conversion
sqlite3 -csv "$SQLITE_DB" "SELECT id, repo_id, year, week, week_start, week_end, summary, commit_count, metadata, CASE WHEN agent_mode THEN 'true' ELSE 'false' END, tool_usage_stats, created_at, updated_at, source_run_id FROM weekly_reports;" | clean_timestamps > "$TMPDIR/weekly_reports.csv"

# Export subscribers with boolean conversion
sqlite3 -csv "$SQLITE_DB" "SELECT id, email, CASE WHEN subscribe_all THEN 'true' ELSE 'false' END, created_at FROM subscribers;" | clean_timestamps > "$TMPDIR/subscribers.csv"

# Export subscriptions
sqlite3 -csv "$SQLITE_DB" "SELECT id, subscriber_id, repo_id, created_at FROM subscriptions;" | clean_timestamps > "$TMPDIR/subscriptions.csv"

# Export newsletter_sends
sqlite3 -csv "$SQLITE_DB" "SELECT id, subscriber_id, activity_run_id, sent_at, sendgrid_message_id FROM newsletter_sends;" | clean_timestamps > "$TMPDIR/newsletter_sends.csv"

echo "  Done."

echo "Step 3: Importing to PostgreSQL..."

# Import repositories
if [ -s "$TMPDIR/repositories.csv" ]; then
    psql -d "$PG_DB" -q -c "\COPY repositories(id, name, url, branch, active, private, description, created_at, updated_at, last_run_at, last_run_sha) FROM '$TMPDIR/repositories.csv' WITH CSV"
fi
COUNT=$(psql -d "$PG_DB" -tAc "SELECT COUNT(*) FROM repositories")
echo "  repositories: $COUNT"

# Import admins
if [ -s "$TMPDIR/admins.csv" ]; then
    psql -d "$PG_DB" -q -c "\COPY admins(id, email, created_at, created_by) FROM '$TMPDIR/admins.csv' WITH CSV"
fi
COUNT=$(psql -d "$PG_DB" -tAc "SELECT COUNT(*) FROM admins")
echo "  admins: $COUNT"

# Import activity_runs
if [ -s "$TMPDIR/activity_runs.csv" ]; then
    psql -d "$PG_DB" -q -c "\COPY activity_runs(id, repo_id, start_sha, end_sha, started_at, completed_at, summary, raw_data, agent_mode, tool_usage_stats) FROM '$TMPDIR/activity_runs.csv' WITH CSV"
fi
COUNT=$(psql -d "$PG_DB" -tAc "SELECT COUNT(*) FROM activity_runs")
echo "  activity_runs: $COUNT"

# Import weekly_reports
if [ -s "$TMPDIR/weekly_reports.csv" ]; then
    psql -d "$PG_DB" -q -c "\COPY weekly_reports(id, repo_id, year, week, week_start, week_end, summary, commit_count, metadata, agent_mode, tool_usage_stats, created_at, updated_at, source_run_id) FROM '$TMPDIR/weekly_reports.csv' WITH CSV"
fi
COUNT=$(psql -d "$PG_DB" -tAc "SELECT COUNT(*) FROM weekly_reports")
echo "  weekly_reports: $COUNT"

# Import subscribers
if [ -s "$TMPDIR/subscribers.csv" ]; then
    psql -d "$PG_DB" -q -c "\COPY subscribers(id, email, subscribe_all, created_at) FROM '$TMPDIR/subscribers.csv' WITH CSV"
fi
COUNT=$(psql -d "$PG_DB" -tAc "SELECT COUNT(*) FROM subscribers")
echo "  subscribers: $COUNT"

# Import subscriptions
if [ -s "$TMPDIR/subscriptions.csv" ]; then
    psql -d "$PG_DB" -q -c "\COPY subscriptions(id, subscriber_id, repo_id, created_at) FROM '$TMPDIR/subscriptions.csv' WITH CSV"
fi
COUNT=$(psql -d "$PG_DB" -tAc "SELECT COUNT(*) FROM subscriptions")
echo "  subscriptions: $COUNT"

# Import newsletter_sends
if [ -s "$TMPDIR/newsletter_sends.csv" ]; then
    psql -d "$PG_DB" -q -c "\COPY newsletter_sends(id, subscriber_id, activity_run_id, sent_at, sendgrid_message_id) FROM '$TMPDIR/newsletter_sends.csv' WITH CSV"
fi
COUNT=$(psql -d "$PG_DB" -tAc "SELECT COUNT(*) FROM newsletter_sends")
echo "  newsletter_sends: $COUNT"

echo ""
echo "Step 4: Resetting sequences..."
psql -d "$PG_DB" -q <<'EOF'
SELECT setval('repositories_id_seq', COALESCE((SELECT MAX(id) FROM repositories), 0) + 1, false);
SELECT setval('admins_id_seq', COALESCE((SELECT MAX(id) FROM admins), 0) + 1, false);
SELECT setval('activity_runs_id_seq', COALESCE((SELECT MAX(id) FROM activity_runs), 0) + 1, false);
SELECT setval('weekly_reports_id_seq', COALESCE((SELECT MAX(id) FROM weekly_reports), 0) + 1, false);
SELECT setval('subscribers_id_seq', COALESCE((SELECT MAX(id) FROM subscribers), 0) + 1, false);
SELECT setval('subscriptions_id_seq', COALESCE((SELECT MAX(id) FROM subscriptions), 0) + 1, false);
SELECT setval('newsletter_sends_id_seq', COALESCE((SELECT MAX(id) FROM newsletter_sends), 0) + 1, false);
EOF
echo "  Done."

echo ""
echo "=== Migration Complete ==="
