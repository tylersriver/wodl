package sqlite

import (
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/tyler/wodl/internal/domain/entities"
)

type LiftLogRepository struct {
	db *sql.DB
}

func NewLiftLogRepository(db *sql.DB) *LiftLogRepository {
	return &LiftLogRepository{db: db}
}

func (r *LiftLogRepository) Create(log *entities.ValidatedLiftLog) (*entities.LiftLog, error) {
	_, err := r.db.Exec(
		`INSERT INTO lift_logs (id, user_id, lift_id, weight, reps, sets, rpe, estimated_1rm, percent_of_1rm, notes, logged_at, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		log.Id.String(), log.UserId.String(), log.LiftId.String(),
		log.Weight, log.Reps, log.Sets, log.RPE, log.Estimated1RM, log.PercentOf1RM,
		log.Notes, log.LoggedAt, log.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("creating lift log: %w", err)
	}
	result := log.LiftLog
	return &result, nil
}

func (r *LiftLogRepository) FindById(id uuid.UUID) (*entities.LiftLog, error) {
	row := r.db.QueryRow(
		`SELECT id, user_id, lift_id, weight, reps, sets, rpe, estimated_1rm, percent_of_1rm, notes, logged_at, created_at
		 FROM lift_logs WHERE id = ?`, id.String(),
	)
	return r.scanRow(row)
}

func (r *LiftLogRepository) FindByLiftId(liftId uuid.UUID, limit int) ([]*entities.LiftLog, error) {
	rows, err := r.db.Query(
		`SELECT id, user_id, lift_id, weight, reps, sets, rpe, estimated_1rm, percent_of_1rm, notes, logged_at, created_at
		 FROM lift_logs WHERE lift_id = ? ORDER BY logged_at DESC LIMIT ?`,
		liftId.String(), limit,
	)
	if err != nil {
		return nil, fmt.Errorf("querying lift logs: %w", err)
	}
	defer rows.Close()
	return r.scanRows(rows)
}

func (r *LiftLogRepository) FindByUserId(userId uuid.UUID, limit int) ([]*entities.LiftLog, error) {
	rows, err := r.db.Query(
		`SELECT id, user_id, lift_id, weight, reps, sets, rpe, estimated_1rm, percent_of_1rm, notes, logged_at, created_at
		 FROM lift_logs WHERE user_id = ? ORDER BY logged_at DESC LIMIT ?`,
		userId.String(), limit,
	)
	if err != nil {
		return nil, fmt.Errorf("querying lift logs: %w", err)
	}
	defer rows.Close()
	return r.scanRows(rows)
}

func (r *LiftLogRepository) Delete(id uuid.UUID) error {
	_, err := r.db.Exec(`DELETE FROM lift_logs WHERE id = ?`, id.String())
	if err != nil {
		return fmt.Errorf("deleting lift log: %w", err)
	}
	return nil
}

func (r *LiftLogRepository) scanRow(row *sql.Row) (*entities.LiftLog, error) {
	var ll entities.LiftLog
	var idStr, userIdStr, liftIdStr string
	err := row.Scan(&idStr, &userIdStr, &liftIdStr, &ll.Weight, &ll.Reps, &ll.Sets,
		&ll.RPE, &ll.Estimated1RM, &ll.PercentOf1RM, &ll.Notes, &ll.LoggedAt, &ll.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scanning lift log: %w", err)
	}
	ll.Id, _ = uuid.Parse(idStr)
	ll.UserId, _ = uuid.Parse(userIdStr)
	ll.LiftId, _ = uuid.Parse(liftIdStr)
	return &ll, nil
}

func (r *LiftLogRepository) scanRows(rows *sql.Rows) ([]*entities.LiftLog, error) {
	var logs []*entities.LiftLog
	for rows.Next() {
		var ll entities.LiftLog
		var idStr, userIdStr, liftIdStr string
		err := rows.Scan(&idStr, &userIdStr, &liftIdStr, &ll.Weight, &ll.Reps, &ll.Sets,
			&ll.RPE, &ll.Estimated1RM, &ll.PercentOf1RM, &ll.Notes, &ll.LoggedAt, &ll.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("scanning lift log row: %w", err)
		}
		ll.Id, _ = uuid.Parse(idStr)
		ll.UserId, _ = uuid.Parse(userIdStr)
		ll.LiftId, _ = uuid.Parse(liftIdStr)
		logs = append(logs, &ll)
	}
	return logs, rows.Err()
}
