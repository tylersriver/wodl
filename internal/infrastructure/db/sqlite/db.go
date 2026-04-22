package sqlite

import (
	"database/sql"
	"fmt"
	"strings"

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
    lift_id TEXT REFERENCES lifts(id),
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

CREATE TABLE IF NOT EXISTS sessions (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id),
    name TEXT NOT NULL,
    warmup TEXT,
    total_time_minutes INTEGER,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    deleted_at DATETIME
);

CREATE TABLE IF NOT EXISTS session_workouts (
    session_id TEXT NOT NULL REFERENCES sessions(id),
    workout_id TEXT NOT NULL REFERENCES workouts(id),
    position INTEGER NOT NULL,
    PRIMARY KEY (session_id, position)
);

CREATE INDEX IF NOT EXISTS idx_lifts_user_id ON lifts(user_id);
CREATE INDEX IF NOT EXISTS idx_lift_logs_lift_id ON lift_logs(lift_id);
CREATE INDEX IF NOT EXISTS idx_lift_logs_user_id ON lift_logs(user_id);
CREATE INDEX IF NOT EXISTS idx_workouts_user_id ON workouts(user_id);
CREATE INDEX IF NOT EXISTS idx_workout_results_workout_id ON workout_results(workout_id);
CREATE INDEX IF NOT EXISTS idx_workout_results_user_id ON workout_results(user_id);
CREATE INDEX IF NOT EXISTS idx_sessions_user_id ON sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_session_workouts_session_id ON session_workouts(session_id);
CREATE INDEX IF NOT EXISTS idx_session_workouts_workout_id ON session_workouts(workout_id);
`

// workoutLiftingColumns adds the lifting-specific columns to the workouts table
// if they don't already exist (idempotent migration for databases created before
// the lifting feature).
var workoutLiftingColumns = []struct {
	name string
	ddl  string
}{
	{"lift_id", "ALTER TABLE workouts ADD COLUMN lift_id TEXT REFERENCES lifts(id)"},
}

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

	if err := ensureWorkoutLiftingColumns(db); err != nil {
		return nil, fmt.Errorf("ensuring workout lifting columns: %w", err)
	}

	return db, nil
}

func ensureWorkoutLiftingColumns(db *sql.DB) error {
	rows, err := db.Query(`PRAGMA table_info(workouts)`)
	if err != nil {
		return err
	}
	defer rows.Close()

	existing := map[string]bool{}
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return err
		}
		existing[name] = true
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, col := range workoutLiftingColumns {
		if existing[col.name] {
			continue
		}
		if _, err := db.Exec(col.ddl); err != nil {
			// If another process added it in the meantime, SQLite reports a duplicate column error.
			if !strings.Contains(err.Error(), "duplicate column") {
				return fmt.Errorf("adding column %s: %w", col.name, err)
			}
		}
	}
	return nil
}
