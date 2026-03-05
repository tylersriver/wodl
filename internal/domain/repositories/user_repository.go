package repositories

import (
	"github.com/google/uuid"
	"github.com/tyler/wodl/internal/domain/entities"
)

type UserRepository interface {
	Create(user *entities.ValidatedUser) (*entities.User, error)
	FindById(id uuid.UUID) (*entities.User, error)
	FindByEmail(email string) (*entities.User, error)
	Update(user *entities.ValidatedUser) (*entities.User, error)
}
