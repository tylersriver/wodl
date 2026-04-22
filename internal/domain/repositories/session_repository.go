package repositories

import (
	"github.com/google/uuid"
	"github.com/tyler/wodl/internal/domain/entities"
)

type SessionRepository interface {
	Create(s *entities.ValidatedSession) (*entities.Session, error)
	FindById(id uuid.UUID) (*entities.Session, error)
	FindAllByUserId(userId uuid.UUID) ([]*entities.Session, error)
	Update(s *entities.ValidatedSession) (*entities.Session, error)
	Delete(id uuid.UUID) error
}
