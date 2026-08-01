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
    session_date DATETIME,
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
CREATE INDEX IF NOT EXISTS idx_sessions_session_date ON sessions(session_date);
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

// sessionColumns adds columns added to the sessions table after its initial
// introduction; idempotent for databases that already had the sessions table.
var sessionColumns = []struct {
	name string
	ddl  string
}{
	{"session_date", "ALTER TABLE sessions ADD COLUMN session_date DATETIME"},
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

	if err := ensureColumns(db, "workouts", workoutLiftingColumns); err != nil {
		return nil, fmt.Errorf("ensuring workout lifting columns: %w", err)
	}
	if err := ensureColumns(db, "sessions", sessionColumns); err != nil {
		return nil, fmt.Errorf("ensuring session columns: %w", err)
	}
	if err := runVersionedMigrations(db); err != nil {
		return nil, fmt.Errorf("running versioned migrations: %w", err)
	}

	return db, nil
}

// runVersionedMigrations applies migrations gated by SQLite's PRAGMA user_version
// so each runs exactly once per database.
func runVersionedMigrations(db *sql.DB) error {
	var version int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
		return fmt.Errorf("reading user_version: %w", err)
	}

	// v1: workouts.time_cap changed units from seconds to minutes.
	if version < 1 {
		if _, err := db.Exec(
			`UPDATE workouts
			 SET time_cap = CASE
			     WHEN time_cap IS NULL OR time_cap <= 0 THEN time_cap
			     ELSE MAX(1, (time_cap + 30) / 60)
			 END`,
		); err != nil {
			return fmt.Errorf("migrating time_cap to minutes: %w", err)
		}
		if _, err := db.Exec(`PRAGMA user_version = 1`); err != nil {
			return fmt.Errorf("bumping user_version to 1: %w", err)
		}
	}

	// v2: sessions became the plan for a given day rather than something you
	// separately recorded having done, so session_logs and its completion
	// records are gone. Any session that predates the change and has no date
	// is backfilled from its earliest completion before the table is dropped.
	if version < 2 {
		// Databases created after this change never had session_logs, so only
		// reach for it when it is actually present.
		var hasLogs int
		if err := db.QueryRow(
			`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'session_logs'`,
		).Scan(&hasLogs); err != nil {
			return fmt.Errorf("checking for session_logs: %w", err)
		}

		if hasLogs > 0 {
			if _, err := db.Exec(
				`UPDATE sessions
				 SET session_date = (
				     SELECT MIN(performed_at) FROM session_logs
				     WHERE session_logs.session_id = sessions.id
				 )
				 WHERE session_date IS NULL
				   AND EXISTS (SELECT 1 FROM session_logs WHERE session_logs.session_id = sessions.id)`,
			); err != nil {
				return fmt.Errorf("backfilling session dates: %w", err)
			}
			if _, err := db.Exec(`DROP TABLE session_logs`); err != nil {
				return fmt.Errorf("dropping session_logs: %w", err)
			}
		}

		// A session is now always tied to a day, so anything still undated
		// falls back to when it was created.
		if _, err := db.Exec(`UPDATE sessions SET session_date = created_at WHERE session_date IS NULL`); err != nil {
			return fmt.Errorf("defaulting session dates: %w", err)
		}
		if _, err := db.Exec(`PRAGMA user_version = 2`); err != nil {
			return fmt.Errorf("bumping user_version to 2: %w", err)
		}
	}

	return nil
}

// ensureColumns runs ALTER TABLE ADD COLUMN for each column missing from the
// given table, skipping ones already present. Idempotent.
func ensureColumns(db *sql.DB, table string, cols []struct {
	name string
	ddl  string
}) error {
	rows, err := db.Query(fmt.Sprintf(`PRAGMA table_info(%s)`, table))
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

	for _, col := range cols {
		if existing[col.name] {
			continue
		}
		if _, err := db.Exec(col.ddl); err != nil {
			if !strings.Contains(err.Error(), "duplicate column") {
				return fmt.Errorf("adding column %s: %w", col.name, err)
			}
		}
	}
	return nil
}
