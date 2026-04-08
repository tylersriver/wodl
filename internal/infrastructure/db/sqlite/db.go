package sqlite

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

const migrationSQL = `
CREATE TABLE IF NOT EXISTS users (
    id TEXT PRIMARY KEY,
    email TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    display_name TEXT NOT NULL,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS lifts (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id),
    name TEXT NOT NULL,
    category TEXT NOT NULL DEFAULT 'custom',
    one_rep_max REAL,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    deleted_at DATETIME
);

CREATE TABLE IF NOT EXISTS lift_logs (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id),
    lift_id TEXT NOT NULL REFERENCES lifts(id),
    weight REAL NOT NULL,
    reps INTEGER NOT NULL,
    sets INTEGER NOT NULL DEFAULT 1,
    rpe REAL,
    estimated_1rm REAL,
    percent_of_1rm REAL,
    notes TEXT,
    logged_at DATETIME NOT NULL,
    created_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS workouts (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id),
    name TEXT NOT NULL,
    type TEXT NOT NULL,
    description TEXT,
    time_cap INTEGER,
    rounds INTEGER,
    interval_seconds INTEGER,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    deleted_at DATETIME
);

CREATE TABLE IF NOT EXISTS workout_results (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id),
    workout_id TEXT NOT NULL REFERENCES workouts(id),
    score TEXT NOT NULL,
    score_type TEXT NOT NULL,
    rx BOOLEAN NOT NULL DEFAULT 0,
    notes TEXT,
    logged_at DATETIME NOT NULL,
    created_at DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_lifts_user_id ON lifts(user_id);
CREATE INDEX IF NOT EXISTS idx_lift_logs_lift_id ON lift_logs(lift_id);
CREATE INDEX IF NOT EXISTS idx_lift_logs_user_id ON lift_logs(user_id);
CREATE INDEX IF NOT EXISTS idx_workouts_user_id ON workouts(user_id);
CREATE INDEX IF NOT EXISTS idx_workout_results_workout_id ON workout_results(workout_id);
CREATE INDEX IF NOT EXISTS idx_workout_results_user_id ON workout_results(user_id);

CREATE TABLE IF NOT EXISTS device_tokens (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id),
    token_hash TEXT UNIQUE NOT NULL,
    device_name TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL,
    last_used_at DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_device_tokens_user_id ON device_tokens(user_id);
CREATE INDEX IF NOT EXISTS idx_device_tokens_token_hash ON device_tokens(token_hash);
`

func NewDB(dataSourceName string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", dataSourceName)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		return nil, fmt.Errorf("setting WAL mode: %w", err)
	}
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		return nil, fmt.Errorf("enabling foreign keys: %w", err)
	}

	if _, err := db.Exec(migrationSQL); err != nil {
		return nil, fmt.Errorf("running migrations: %w", err)
	}

	return db, nil
}
