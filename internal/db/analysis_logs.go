package db

import (
	"database/sql"
	"fmt"
)

// CreateAnalysisLog inserts a new analysis log entry into the database.
func (db *DB) CreateAnalysisLog(activityRunID int64, logType, toolName, content string, sequence int) (*AnalysisLog, error) {
	var toolNameVal interface{}
	if toolName != "" {
		toolNameVal = toolName
	}

	var id int64
	err := db.QueryRow(`
		INSERT INTO analysis_logs (activity_run_id, log_type, tool_name, content, sequence)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id
	`, activityRunID, logType, toolNameVal, content, sequence).Scan(&id)
	if err != nil {
		return nil, fmt.Errorf("failed to create analysis log: %w", err)
	}

	return db.GetAnalysisLog(id)
}

// GetAnalysisLog retrieves an analysis log by ID.
func (db *DB) GetAnalysisLog(id int64) (*AnalysisLog, error) {
	log := &AnalysisLog{}
	err := db.QueryRow(`
		SELECT id, activity_run_id, log_type, tool_name, content, sequence, created_at
		FROM analysis_logs
		WHERE id = $1
	`, id).Scan(&log.ID, &log.ActivityRunID, &log.LogType, &log.ToolName, &log.Content, &log.Sequence, &log.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("analysis log not found: %w", ErrNotFound)
		}
		return nil, fmt.Errorf("failed to get analysis log: %w", err)
	}
	return log, nil
}

// GetAnalysisLogsByActivityRun retrieves all analysis logs for a specific activity run.
func (db *DB) GetAnalysisLogsByActivityRun(activityRunID int64) ([]*AnalysisLog, error) {
	rows, err := db.Query(`
		SELECT id, activity_run_id, log_type, tool_name, content, sequence, created_at
		FROM analysis_logs
		WHERE activity_run_id = $1
		ORDER BY sequence
	`, activityRunID)
	if err != nil {
		return nil, fmt.Errorf("failed to list analysis logs: %w", err)
	}
	defer rows.Close()

	var logs []*AnalysisLog
	for rows.Next() {
		log := &AnalysisLog{}
		if err := rows.Scan(&log.ID, &log.ActivityRunID, &log.LogType, &log.ToolName, &log.Content, &log.Sequence, &log.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan analysis log: %w", err)
		}
		logs = append(logs, log)
	}

	return logs, nil
}
