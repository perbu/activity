-- +goose Up
CREATE TABLE analysis_logs (
    id SERIAL PRIMARY KEY,
    activity_run_id INTEGER NOT NULL,
    log_type TEXT NOT NULL,
    tool_name TEXT,
    content TEXT NOT NULL,
    sequence INTEGER NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    FOREIGN KEY (activity_run_id) REFERENCES activity_runs(id) ON DELETE CASCADE
);
CREATE INDEX idx_analysis_logs_activity_run_id ON analysis_logs(activity_run_id);

-- +goose Down
DROP INDEX IF EXISTS idx_analysis_logs_activity_run_id;
DROP TABLE IF EXISTS analysis_logs;
