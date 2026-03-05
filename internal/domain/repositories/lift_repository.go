package repositories

import (
	"github.com/google/uuid"
	"github.com/tyler/wodl/internal/domain/entities"
)

type LiftRepository interface {
	Create(lift *entities.ValidatedLift) (*entities.Lift, error)
	FindById(id uuid.UUID) (*entities.Lift, error)
	FindAllByUserId(userId uuid.UUID) ([]*entities.Lift, error)
	Update(lift *entities.ValidatedLift) (*entities.Lift, error)
	Delete(id uuid.UUID) error
}
