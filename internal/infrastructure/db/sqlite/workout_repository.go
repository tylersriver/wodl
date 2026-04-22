package sqlite

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/tyler/wodl/internal/domain/entities"
)

type WorkoutRepository struct {
	db *sql.DB
}

func NewWorkoutRepository(db *sql.DB) *WorkoutRepository {
	return &WorkoutRepository{db: db}
}

func (r *WorkoutRepository) Create(w *entities.ValidatedWorkout) (*entities.Workout, error) {
	var liftIdStr *string
	if w.LiftId != nil {
		s := w.LiftId.String()
		liftIdStr = &s
	}
	_, err := r.db.Exec(
		`INSERT INTO workouts (id, user_id, name, type, description, time_cap, rounds, interval_seconds,
		 lift_id, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		w.Id.String(), w.UserId.String(), w.Name, string(w.Type), w.Description,
		w.TimeCap, w.Rounds, w.IntervalSeconds,
		liftIdStr,
		w.CreatedAt, w.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("creating workout: %w", err)
	}
	result := w.Workout
	return &result, nil
}

func (r *WorkoutRepository) FindById(id uuid.UUID) (*entities.Workout, error) {
	row := r.db.QueryRow(
		`SELECT id, user_id, name, type, description, time_cap, rounds, interval_seconds,
		 lift_id, created_at, updated_at, deleted_at
		 FROM workouts WHERE id = ? AND deleted_at IS NULL`, id.String(),
	)
	return r.scanWorkout(row)
}

func (r *WorkoutRepository) FindAllByUserId(userId uuid.UUID) ([]*entities.Workout, error) {
	rows, err := r.db.Query(
		`SELECT id, user_id, name, type, description, time_cap, rounds, interval_seconds,
		 lift_id, created_at, updated_at, deleted_at
		 FROM workouts WHERE user_id = ? AND deleted_at IS NULL ORDER BY name`, userId.String(),
	)
	if err != nil {
		return nil, fmt.Errorf("querying workouts: %w", err)
	}
	defer rows.Close()

	var workouts []*entities.Workout
	for rows.Next() {
		w, err := r.scanWorkoutRow(rows)
		if err != nil {
			return nil, err
		}
		workouts = append(workouts, w)
	}
	return workouts, rows.Err()
}

func (r *WorkoutRepository) Update(w *entities.ValidatedWorkout) (*entities.Workout, error) {
	var liftIdStr *string
	if w.LiftId != nil {
		s := w.LiftId.String()
		liftIdStr = &s
	}
	_, err := r.db.Exec(
		`UPDATE workouts SET name = ?, type = ?, description = ?, time_cap = ?, rounds = ?, interval_seconds = ?,
		 lift_id = ?, updated_at = ?
		 WHERE id = ? AND deleted_at IS NULL`,
		w.Name, string(w.Type), w.Description, w.TimeCap, w.Rounds, w.IntervalSeconds,
		liftIdStr,
		w.UpdatedAt, w.Id.String(),
	)
	if err != nil {
		return nil, fmt.Errorf("updating workout: %w", err)
	}
	result := w.Workout
	return &result, nil
}

func (r *WorkoutRepository) Delete(id uuid.UUID) error {
	_, err := r.db.Exec(
		`UPDATE workouts SET deleted_at = ? WHERE id = ? AND deleted_at IS NULL`,
		time.Now(), id.String(),
	)
	if err != nil {
		return fmt.Errorf("deleting workout: %w", err)
	}
	return nil
}

func (r *WorkoutRepository) scanWorkout(row *sql.Row) (*entities.Workout, error) {
	var w entities.Workout
	var idStr, userIdStr, wType string
	var liftIdStr sql.NullString
	err := row.Scan(&idStr, &userIdStr, &w.Name, &wType, &w.Description,
		&w.TimeCap, &w.Rounds, &w.IntervalSeconds,
		&liftIdStr,
		&w.CreatedAt, &w.UpdatedAt, &w.DeletedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scanning workout: %w", err)
	}
	w.Id, _ = uuid.Parse(idStr)
	w.UserId, _ = uuid.Parse(userIdStr)
	w.Type = entities.WorkoutType(wType)
	if liftIdStr.Valid {
		if parsed, err := uuid.Parse(liftIdStr.String); err == nil {
			w.LiftId = &parsed
		}
	}
	return &w, nil
}

func (r *WorkoutRepository) scanWorkoutRow(rows *sql.Rows) (*entities.Workout, error) {
	var w entities.Workout
	var idStr, userIdStr, wType string
	var liftIdStr sql.NullString
	err := rows.Scan(&idStr, &userIdStr, &w.Name, &wType, &w.Description,
		&w.TimeCap, &w.Rounds, &w.IntervalSeconds,
		&liftIdStr,
		&w.CreatedAt, &w.UpdatedAt, &w.DeletedAt)
	if err != nil {
		return nil, fmt.Errorf("scanning workout row: %w", err)
	}
	w.Id, _ = uuid.Parse(idStr)
	w.UserId, _ = uuid.Parse(userIdStr)
	w.Type = entities.WorkoutType(wType)
	if liftIdStr.Valid {
		if parsed, err := uuid.Parse(liftIdStr.String); err == nil {
			w.LiftId = &parsed
		}
	}
	return &w, nil
}
