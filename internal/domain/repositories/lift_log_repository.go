package repositories

import (
	"github.com/google/uuid"
	"github.com/tyler/wodl/internal/domain/entities"
)

type LiftLogRepository interface {
	Create(log *entities.ValidatedLiftLog) (*entities.LiftLog, error)
	FindById(id uuid.UUID) (*entities.LiftLog, error)
	FindByLiftId(liftId uuid.UUID, limit int) ([]*entities.LiftLog, error)
	FindByUserId(userId uuid.UUID, limit int) ([]*entities.LiftLog, error)
	Delete(id uuid.UUID) error
}
