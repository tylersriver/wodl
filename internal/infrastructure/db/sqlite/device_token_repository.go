package sqlite

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/tyler/wodl/internal/domain/entities"
)

type DeviceTokenRepository struct {
	db *sql.DB
}

func NewDeviceTokenRepository(db *sql.DB) *DeviceTokenRepository {
	return &DeviceTokenRepository{db: db}
}

func (r *DeviceTokenRepository) Create(token *entities.DeviceToken) error {
	_, err := r.db.Exec(
		`INSERT INTO device_tokens (id, user_id, token_hash, device_name, created_at, last_used_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		token.Id.String(), token.UserId.String(), token.TokenHash,
		token.DeviceName, token.CreatedAt, token.LastUsedAt,
	)
	if err != nil {
		return fmt.Errorf("creating device token: %w", err)
	}
	return nil
}

func (r *DeviceTokenRepository) FindByTokenHash(tokenHash string) (*entities.DeviceToken, error) {
	var dt entities.DeviceToken
	var idStr, userIdStr string
	err := r.db.QueryRow(
		`SELECT id, user_id, token_hash, device_name, created_at, last_used_at
		 FROM device_tokens WHERE token_hash = ?`, tokenHash,
	).Scan(&idStr, &userIdStr, &dt.TokenHash, &dt.DeviceName, &dt.CreatedAt, &dt.LastUsedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("finding device token: %w", err)
	}
	dt.Id, err = uuid.Parse(idStr)
	if err != nil {
		return nil, fmt.Errorf("parsing device token id: %w", err)
	}
	dt.UserId, err = uuid.Parse(userIdStr)
	if err != nil {
		return nil, fmt.Errorf("parsing device token user id: %w", err)
	}
	return &dt, nil
}

func (r *DeviceTokenRepository) UpdateLastUsed(id uuid.UUID) error {
	_, err := r.db.Exec(
		`UPDATE device_tokens SET last_used_at = ? WHERE id = ?`,
		time.Now(), id.String(),
	)
	if err != nil {
		return fmt.Errorf("updating device token last used: %w", err)
	}
	return nil
}

func (r *DeviceTokenRepository) DeleteByUserId(userId uuid.UUID) error {
	_, err := r.db.Exec(`DELETE FROM device_tokens WHERE user_id = ?`, userId.String())
	if err != nil {
		return fmt.Errorf("deleting device tokens by user: %w", err)
	}
	return nil
}

func (r *DeviceTokenRepository) DeleteById(id uuid.UUID) error {
	_, err := r.db.Exec(`DELETE FROM device_tokens WHERE id = ?`, id.String())
	if err != nil {
		return fmt.Errorf("deleting device token: %w", err)
	}
	return nil
}
