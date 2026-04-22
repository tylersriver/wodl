package sqlite

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/tyler/wodl/internal/domain/entities"
)

type SessionRepository struct {
	db *sql.DB
}

func NewSessionRepository(db *sql.DB) *SessionRepository {
	return &SessionRepository{db: db}
}

func (r *SessionRepository) Create(s *entities.ValidatedSession) (*entities.Session, error) {
	tx, err := r.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(
		`INSERT INTO sessions (id, user_id, name, warmup, session_date, total_time_minutes, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		s.Id.String(), s.UserId.String(), s.Name, s.Warmup, s.Date, s.TotalTimeMinutes,
		s.CreatedAt, s.UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("creating session: %w", err)
	}

	if err := r.insertSessionWorkouts(tx, s.Id, s.WorkoutIds); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}

	result := s.Session
	return &result, nil
}

func (r *SessionRepository) FindById(id uuid.UUID) (*entities.Session, error) {
	row := r.db.QueryRow(
		`SELECT id, user_id, name, warmup, session_date, total_time_minutes, created_at, updated_at, deleted_at
		 FROM sessions WHERE id = ? AND deleted_at IS NULL`, id.String(),
	)
	s, err := r.scanSession(row)
	if err != nil || s == nil {
		return s, err
	}
	ids, err := r.findWorkoutIds(s.Id)
	if err != nil {
		return nil, err
	}
	s.WorkoutIds = ids
	return s, nil
}

func (r *SessionRepository) FindAllByUserId(userId uuid.UUID) ([]*entities.Session, error) {
	rows, err := r.db.Query(
		`SELECT id, user_id, name, warmup, session_date, total_time_minutes, created_at, updated_at, deleted_at
		 FROM sessions WHERE user_id = ? AND deleted_at IS NULL
		 ORDER BY session_date DESC, name`, userId.String(),
	)
	if err != nil {
		return nil, fmt.Errorf("querying sessions: %w", err)
	}
	defer rows.Close()

	var sessions []*entities.Session
	for rows.Next() {
		s, err := r.scanSessionRow(rows)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, s)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for _, s := range sessions {
		ids, err := r.findWorkoutIds(s.Id)
		if err != nil {
			return nil, err
		}
		s.WorkoutIds = ids
	}
	return sessions, nil
}

func (r *SessionRepository) Update(s *entities.ValidatedSession) (*entities.Session, error) {
	tx, err := r.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(
		`UPDATE sessions SET name = ?, warmup = ?, session_date = ?, total_time_minutes = ?, updated_at = ?
		 WHERE id = ? AND deleted_at IS NULL`,
		s.Name, s.Warmup, s.Date, s.TotalTimeMinutes, s.UpdatedAt, s.Id.String(),
	); err != nil {
		return nil, fmt.Errorf("updating session: %w", err)
	}

	if _, err := tx.Exec(`DELETE FROM session_workouts WHERE session_id = ?`, s.Id.String()); err != nil {
		return nil, fmt.Errorf("clearing session workouts: %w", err)
	}

	if err := r.insertSessionWorkouts(tx, s.Id, s.WorkoutIds); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}

	result := s.Session
	return &result, nil
}

func (r *SessionRepository) Delete(id uuid.UUID) error {
	_, err := r.db.Exec(
		`UPDATE sessions SET deleted_at = ? WHERE id = ? AND deleted_at IS NULL`,
		time.Now(), id.String(),
	)
	if err != nil {
		return fmt.Errorf("deleting session: %w", err)
	}
	return nil
}

func (r *SessionRepository) insertSessionWorkouts(tx *sql.Tx, sessionId uuid.UUID, workoutIds []uuid.UUID) error {
	for i, wid := range workoutIds {
		if _, err := tx.Exec(
			`INSERT INTO session_workouts (session_id, workout_id, position) VALUES (?, ?, ?)`,
			sessionId.String(), wid.String(), i,
		); err != nil {
			return fmt.Errorf("linking workout to session: %w", err)
		}
	}
	return nil
}

func (r *SessionRepository) findWorkoutIds(sessionId uuid.UUID) ([]uuid.UUID, error) {
	rows, err := r.db.Query(
		`SELECT workout_id FROM session_workouts WHERE session_id = ? ORDER BY position`,
		sessionId.String(),
	)
	if err != nil {
		return nil, fmt.Errorf("querying session workouts: %w", err)
	}
	defer rows.Close()

	var ids []uuid.UUID
	for rows.Next() {
		var idStr string
		if err := rows.Scan(&idStr); err != nil {
			return nil, fmt.Errorf("scanning session workout: %w", err)
		}
		parsed, err := uuid.Parse(idStr)
		if err != nil {
			return nil, fmt.Errorf("parsing workout id: %w", err)
		}
		ids = append(ids, parsed)
	}
	return ids, rows.Err()
}

func (r *SessionRepository) scanSession(row *sql.Row) (*entities.Session, error) {
	var s entities.Session
	var idStr, userIdStr string
	err := row.Scan(&idStr, &userIdStr, &s.Name, &s.Warmup, &s.Date, &s.TotalTimeMinutes,
		&s.CreatedAt, &s.UpdatedAt, &s.DeletedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scanning session: %w", err)
	}
	s.Id, _ = uuid.Parse(idStr)
	s.UserId, _ = uuid.Parse(userIdStr)
	return &s, nil
}

func (r *SessionRepository) scanSessionRow(rows *sql.Rows) (*entities.Session, error) {
	var s entities.Session
	var idStr, userIdStr string
	err := rows.Scan(&idStr, &userIdStr, &s.Name, &s.Warmup, &s.Date, &s.TotalTimeMinutes,
		&s.CreatedAt, &s.UpdatedAt, &s.DeletedAt)
	if err != nil {
		return nil, fmt.Errorf("scanning session row: %w", err)
	}
	s.Id, _ = uuid.Parse(idStr)
	s.UserId, _ = uuid.Parse(userIdStr)
	return &s, nil
}
