package sqlite

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/tyler/wodl/internal/domain/entities"
	"github.com/tyler/wodl/internal/domain/repositories"
)

type SessionLogRepository struct {
	db *sql.DB
}

func NewSessionLogRepository(db *sql.DB) *SessionLogRepository {
	return &SessionLogRepository{db: db}
}

func (r *SessionLogRepository) Create(log *entities.ValidatedSessionLog) (*entities.SessionLog, error) {
	_, err := r.db.Exec(
		`INSERT INTO session_logs (id, user_id, session_id, performed_at, notes, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		log.Id.String(), log.UserId.String(), log.SessionId.String(),
		log.PerformedAt, log.Notes, log.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("creating session log: %w", err)
	}
	result := log.SessionLog
	return &result, nil
}

func (r *SessionLogRepository) FindById(id uuid.UUID) (*entities.SessionLog, error) {
	row := r.db.QueryRow(
		`SELECT id, user_id, session_id, performed_at, notes, created_at
		 FROM session_logs WHERE id = ?`, id.String(),
	)
	var log entities.SessionLog
	var idStr, userIdStr, sessionIdStr string
	err := row.Scan(&idStr, &userIdStr, &sessionIdStr, &log.PerformedAt, &log.Notes, &log.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scanning session log: %w", err)
	}
	log.Id, _ = uuid.Parse(idStr)
	log.UserId, _ = uuid.Parse(userIdStr)
	log.SessionId, _ = uuid.Parse(sessionIdStr)
	return &log, nil
}

func (r *SessionLogRepository) FindBySessionId(sessionId uuid.UUID) ([]*entities.SessionLog, error) {
	rows, err := r.db.Query(
		`SELECT id, user_id, session_id, performed_at, notes, created_at
		 FROM session_logs WHERE session_id = ? ORDER BY performed_at DESC`,
		sessionId.String(),
	)
	if err != nil {
		return nil, fmt.Errorf("querying session logs: %w", err)
	}
	defer rows.Close()

	var logs []*entities.SessionLog
	for rows.Next() {
		var log entities.SessionLog
		var idStr, userIdStr, sessionIdStr string
		if err := rows.Scan(&idStr, &userIdStr, &sessionIdStr, &log.PerformedAt, &log.Notes, &log.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning session log: %w", err)
		}
		log.Id, _ = uuid.Parse(idStr)
		log.UserId, _ = uuid.Parse(userIdStr)
		log.SessionId, _ = uuid.Parse(sessionIdStr)
		logs = append(logs, &log)
	}
	return logs, rows.Err()
}

func (r *SessionLogRepository) FindByUserInRange(userId uuid.UUID, start, end time.Time) ([]*repositories.SessionLogWithName, error) {
	rows, err := r.db.Query(
		`SELECT sl.id, sl.user_id, sl.session_id, sl.performed_at, sl.notes, sl.created_at, s.name
		 FROM session_logs sl
		 JOIN sessions s ON s.id = sl.session_id
		 WHERE sl.user_id = ? AND sl.performed_at >= ? AND sl.performed_at < ?
		   AND s.deleted_at IS NULL
		 ORDER BY sl.performed_at ASC`,
		userId.String(), start, end,
	)
	if err != nil {
		return nil, fmt.Errorf("querying session logs in range: %w", err)
	}
	defer rows.Close()

	var out []*repositories.SessionLogWithName
	for rows.Next() {
		var log entities.SessionLog
		var idStr, userIdStr, sessionIdStr, name string
		if err := rows.Scan(&idStr, &userIdStr, &sessionIdStr, &log.PerformedAt, &log.Notes, &log.CreatedAt, &name); err != nil {
			return nil, fmt.Errorf("scanning session log with name: %w", err)
		}
		log.Id, _ = uuid.Parse(idStr)
		log.UserId, _ = uuid.Parse(userIdStr)
		log.SessionId, _ = uuid.Parse(sessionIdStr)
		out = append(out, &repositories.SessionLogWithName{Log: &log, SessionName: name})
	}
	return out, rows.Err()
}

func (r *SessionLogRepository) Delete(id uuid.UUID) error {
	_, err := r.db.Exec(`DELETE FROM session_logs WHERE id = ?`, id.String())
	if err != nil {
		return fmt.Errorf("deleting session log: %w", err)
	}
	return nil
}
