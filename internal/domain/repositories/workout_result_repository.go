package repositories

import (
	"github.com/google/uuid"
	"github.com/tyler/wodl/internal/domain/entities"
)

type WorkoutResultRepository interface {
	Create(r *entities.ValidatedWorkoutResult) (*entities.WorkoutResult, error)
	FindById(id uuid.UUID) (*entities.WorkoutResult, error)
	FindByWorkoutId(workoutId uuid.UUID) ([]*entities.WorkoutResult, error)
	FindByUserId(userId uuid.UUID, limit int) ([]*entities.WorkoutResult, error)
	Delete(id uuid.UUID) error
}
