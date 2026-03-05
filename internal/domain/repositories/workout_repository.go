package repositories

import (
	"github.com/google/uuid"
	"github.com/tyler/wodl/internal/domain/entities"
)

type WorkoutRepository interface {
	Create(w *entities.ValidatedWorkout) (*entities.Workout, error)
	FindById(id uuid.UUID) (*entities.Workout, error)
	FindAllByUserId(userId uuid.UUID) ([]*entities.Workout, error)
	Update(w *entities.ValidatedWorkout) (*entities.Workout, error)
	Delete(id uuid.UUID) error
}
