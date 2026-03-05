package sqlite

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/tyler/wodl/internal/domain/entities"
)

type LiftRepository struct {
	db *sql.DB
}

func NewLiftRepository(db *sql.DB) *LiftRepository {
	return &LiftRepository{db: db}
}

func (r *LiftRepository) Create(lift *entities.ValidatedLift) (*entities.Lift, error) {
	_, err := r.db.Exec(
		`INSERT INTO lifts (id, user_id, name, category, one_rep_max, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		lift.Id.String(), lift.UserId.String(), lift.Name, string(lift.Category),
		lift.OneRepMax, lift.CreatedAt, lift.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("creating lift: %w", err)
	}
	result := lift.Lift
	return &result, nil
}

func (r *LiftRepository) FindById(id uuid.UUID) (*entities.Lift, error) {
	row := r.db.QueryRow(
		`SELECT id, user_id, name, category, one_rep_max, created_at, updated_at, deleted_at
		 FROM lifts WHERE id = ? AND deleted_at IS NULL`, id.String(),
	)
	return r.scanLift(row)
}

func (r *LiftRepository) FindAllByUserId(userId uuid.UUID) ([]*entities.Lift, error) {
	rows, err := r.db.Query(
		`SELECT id, user_id, name, category, one_rep_max, created_at, updated_at, deleted_at
		 FROM lifts WHERE user_id = ? AND deleted_at IS NULL ORDER BY name`, userId.String(),
	)
	if err != nil {
		return nil, fmt.Errorf("querying lifts: %w", err)
	}
	defer rows.Close()

	var lifts []*entities.Lift
	for rows.Next() {
		lift, err := r.scanLiftRow(rows)
		if err != nil {
			return nil, err
		}
		lifts = append(lifts, lift)
	}
	return lifts, rows.Err()
}

func (r *LiftRepository) Update(lift *entities.ValidatedLift) (*entities.Lift, error) {
	_, err := r.db.Exec(
		`UPDATE lifts SET name = ?, category = ?, one_rep_max = ?, updated_at = ?
		 WHERE id = ? AND deleted_at IS NULL`,
		lift.Name, string(lift.Category), lift.OneRepMax, lift.UpdatedAt, lift.Id.String(),
	)
	if err != nil {
		return nil, fmt.Errorf("updating lift: %w", err)
	}
	result := lift.Lift
	return &result, nil
}

func (r *LiftRepository) Delete(id uuid.UUID) error {
	_, err := r.db.Exec(
		`UPDATE lifts SET deleted_at = ? WHERE id = ? AND deleted_at IS NULL`,
		time.Now(), id.String(),
	)
	if err != nil {
		return fmt.Errorf("deleting lift: %w", err)
	}
	return nil
}

func (r *LiftRepository) scanLift(row *sql.Row) (*entities.Lift, error) {
	var lift entities.Lift
	var idStr, userIdStr, category string
	err := row.Scan(&idStr, &userIdStr, &lift.Name, &category, &lift.OneRepMax,
		&lift.CreatedAt, &lift.UpdatedAt, &lift.DeletedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scanning lift: %w", err)
	}
	lift.Id, _ = uuid.Parse(idStr)
	lift.UserId, _ = uuid.Parse(userIdStr)
	lift.Category = entities.LiftCategory(category)
	return &lift, nil
}

func (r *LiftRepository) scanLiftRow(rows *sql.Rows) (*entities.Lift, error) {
	var lift entities.Lift
	var idStr, userIdStr, category string
	err := rows.Scan(&idStr, &userIdStr, &lift.Name, &category, &lift.OneRepMax,
		&lift.CreatedAt, &lift.UpdatedAt, &lift.DeletedAt)
	if err != nil {
		return nil, fmt.Errorf("scanning lift row: %w", err)
	}
	lift.Id, _ = uuid.Parse(idStr)
	lift.UserId, _ = uuid.Parse(userIdStr)
	lift.Category = entities.LiftCategory(category)
	return &lift, nil
}
