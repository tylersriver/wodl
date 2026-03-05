package sqlite

import (
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/tyler/wodl/internal/domain/entities"
)

type WorkoutResultRepository struct {
	db *sql.DB
}

func NewWorkoutResultRepository(db *sql.DB) *WorkoutResultRepository {
	return &WorkoutResultRepository{db: db}
}

func (r *WorkoutResultRepository) Create(wr *entities.ValidatedWorkoutResult) (*entities.WorkoutResult, error) {
	_, err := r.db.Exec(
		`INSERT INTO workout_results (id, user_id, workout_id, score, score_type, rx, notes, logged_at, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		wr.Id.String(), wr.UserId.String(), wr.WorkoutId.String(),
		wr.Score, string(wr.ScoreType), wr.Rx, wr.Notes, wr.LoggedAt, wr.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("creating workout result: %w", err)
	}
	result := wr.WorkoutResult
	return &result, nil
}

func (r *WorkoutResultRepository) FindById(id uuid.UUID) (*entities.WorkoutResult, error) {
	row := r.db.QueryRow(
		`SELECT id, user_id, workout_id, score, score_type, rx, notes, logged_at, created_at
		 FROM workout_results WHERE id = ?`, id.String(),
	)
	return r.scanRow(row)
}

func (r *WorkoutResultRepository) FindByWorkoutId(workoutId uuid.UUID) ([]*entities.WorkoutResult, error) {
	rows, err := r.db.Query(
		`SELECT id, user_id, workout_id, score, score_type, rx, notes, logged_at, created_at
		 FROM workout_results WHERE workout_id = ? ORDER BY logged_at DESC`, workoutId.String(),
	)
	if err != nil {
		return nil, fmt.Errorf("querying workout results: %w", err)
	}
	defer rows.Close()
	return r.scanRows(rows)
}

func (r *WorkoutResultRepository) FindByUserId(userId uuid.UUID, limit int) ([]*entities.WorkoutResult, error) {
	rows, err := r.db.Query(
		`SELECT id, user_id, workout_id, score, score_type, rx, notes, logged_at, created_at
		 FROM workout_results WHERE user_id = ? ORDER BY logged_at DESC LIMIT ?`,
		userId.String(), limit,
	)
	if err != nil {
		return nil, fmt.Errorf("querying workout results: %w", err)
	}
	defer rows.Close()
	return r.scanRows(rows)
}

func (r *WorkoutResultRepository) Delete(id uuid.UUID) error {
	_, err := r.db.Exec(`DELETE FROM workout_results WHERE id = ?`, id.String())
	if err != nil {
		return fmt.Errorf("deleting workout result: %w", err)
	}
	return nil
}

func (r *WorkoutResultRepository) scanRow(row *sql.Row) (*entities.WorkoutResult, error) {
	var wr entities.WorkoutResult
	var idStr, userIdStr, workoutIdStr, scoreType string
	err := row.Scan(&idStr, &userIdStr, &workoutIdStr, &wr.Score, &scoreType,
		&wr.Rx, &wr.Notes, &wr.LoggedAt, &wr.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scanning workout result: %w", err)
	}
	wr.Id, _ = uuid.Parse(idStr)
	wr.UserId, _ = uuid.Parse(userIdStr)
	wr.WorkoutId, _ = uuid.Parse(workoutIdStr)
	wr.ScoreType = entities.ScoreType(scoreType)
	return &wr, nil
}

func (r *WorkoutResultRepository) scanRows(rows *sql.Rows) ([]*entities.WorkoutResult, error) {
	var results []*entities.WorkoutResult
	for rows.Next() {
		var wr entities.WorkoutResult
		var idStr, userIdStr, workoutIdStr, scoreType string
		err := rows.Scan(&idStr, &userIdStr, &workoutIdStr, &wr.Score, &scoreType,
			&wr.Rx, &wr.Notes, &wr.LoggedAt, &wr.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("scanning workout result row: %w", err)
		}
		wr.Id, _ = uuid.Parse(idStr)
		wr.UserId, _ = uuid.Parse(userIdStr)
		wr.WorkoutId, _ = uuid.Parse(workoutIdStr)
		wr.ScoreType = entities.ScoreType(scoreType)
		results = append(results, &wr)
	}
	return results, rows.Err()
}
