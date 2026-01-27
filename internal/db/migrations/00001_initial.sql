-- +goose Up
-- Consolidated schema for PostgreSQL

CREATE TABLE repositories (
    id SERIAL PRIMARY KEY,
    name TEXT UNIQUE NOT NULL,
    url TEXT NOT NULL,
    branch TEXT NOT NULL DEFAULT 'main',
    active BOOLEAN NOT NULL DEFAULT true,
    private BOOLEAN NOT NULL DEFAULT false,
    description TEXT,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    last_run_at TIMESTAMP WITH TIME ZONE,
    last_run_sha TEXT
);

CREATE TABLE activity_runs (
    id SERIAL PRIMARY KEY,
    repo_id INTEGER NOT NULL,
    start_sha TEXT NOT NULL,
    end_sha TEXT NOT NULL,
    started_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMP WITH TIME ZONE,
    summary TEXT,
    raw_data TEXT,
    agent_mode BOOLEAN DEFAULT false,
    tool_usage_stats TEXT,
    FOREIGN KEY (repo_id) REFERENCES repositories(id) ON DELETE CASCADE
);

CREATE INDEX idx_activity_runs_repo_id ON activity_runs(repo_id);
CREATE INDEX idx_activity_runs_started_at ON activity_runs(started_at);
CREATE INDEX idx_activity_runs_agent_mode ON activity_runs(agent_mode);

CREATE TABLE subscribers (
    id SERIAL PRIMARY KEY,
    email TEXT UNIQUE NOT NULL,
    subscribe_all BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE TABLE subscriptions (
    id SERIAL PRIMARY KEY,
    subscriber_id INTEGER NOT NULL,
    repo_id INTEGER NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    FOREIGN KEY (subscriber_id) REFERENCES subscribers(id) ON DELETE CASCADE,
    FOREIGN KEY (repo_id) REFERENCES repositories(id) ON DELETE CASCADE,
    UNIQUE(subscriber_id, repo_id)
);

CREATE INDEX idx_subscriptions_subscriber_id ON subscriptions(subscriber_id);
CREATE INDEX idx_subscriptions_repo_id ON subscriptions(repo_id);

CREATE TABLE newsletter_sends (
    id SERIAL PRIMARY KEY,
    subscriber_id INTEGER NOT NULL,
    activity_run_id INTEGER NOT NULL,
    sent_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    sendgrid_message_id TEXT,
    FOREIGN KEY (subscriber_id) REFERENCES subscribers(id) ON DELETE CASCADE,
    FOREIGN KEY (activity_run_id) REFERENCES activity_runs(id) ON DELETE CASCADE,
    UNIQUE(subscriber_id, activity_run_id)
);

CREATE INDEX idx_newsletter_sends_subscriber_id ON newsletter_sends(subscriber_id);
CREATE INDEX idx_newsletter_sends_activity_run_id ON newsletter_sends(activity_run_id);

CREATE TABLE weekly_reports (
    id SERIAL PRIMARY KEY,
    repo_id INTEGER NOT NULL,
    year INTEGER NOT NULL,
    week INTEGER NOT NULL,
    week_start DATE NOT NULL,
    week_end DATE NOT NULL,
    summary TEXT,
    commit_count INTEGER NOT NULL DEFAULT 0,
    metadata TEXT,
    agent_mode BOOLEAN DEFAULT false,
    tool_usage_stats TEXT,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    source_run_id INTEGER,
    FOREIGN KEY (repo_id) REFERENCES repositories(id) ON DELETE CASCADE,
    FOREIGN KEY (source_run_id) REFERENCES activity_runs(id) ON DELETE SET NULL,
    UNIQUE(repo_id, year, week)
);

CREATE INDEX idx_weekly_reports_repo_id ON weekly_reports(repo_id);
CREATE INDEX idx_weekly_reports_year_week ON weekly_reports(year, week);

CREATE TABLE admins (
    id SERIAL PRIMARY KEY,
    email TEXT UNIQUE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    created_by TEXT
);

CREATE INDEX idx_admins_email ON admins(email);

-- +goose Down
DROP TABLE IF EXISTS admins;
DROP TABLE IF EXISTS weekly_reports;
DROP TABLE IF EXISTS newsletter_sends;
DROP TABLE IF EXISTS subscriptions;
DROP TABLE IF EXISTS subscribers;
DROP TABLE IF EXISTS activity_runs;
DROP TABLE IF EXISTS repositories;
