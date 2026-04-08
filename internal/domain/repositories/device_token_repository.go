package repositories

import (
	"github.com/google/uuid"
	"github.com/tyler/wodl/internal/domain/entities"
)

type DeviceTokenRepository interface {
	Create(token *entities.DeviceToken) error
	FindByTokenHash(tokenHash string) (*entities.DeviceToken, error)
	UpdateLastUsed(id uuid.UUID) error
	DeleteByUserId(userId uuid.UUID) error
	DeleteById(id uuid.UUID) error
}
