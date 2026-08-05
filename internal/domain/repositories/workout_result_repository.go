package repositories

import (
	"time"

	"github.com/google/uuid"
	"github.com/tyler/wodl/internal/domain/entities"
)

type WorkoutResultRepository interface {
	Create(r *entities.ValidatedWorkoutResult) (*entities.WorkoutResult, error)
	FindById(id uuid.UUID) (*entities.WorkoutResult, error)
	FindByWorkoutId(workoutId uuid.UUID) ([]*entities.WorkoutResult, error)
	FindByUserId(userId uuid.UUID, limit int) ([]*entities.WorkoutResult, error)
	// FindByUserInRange selects the results logged within [start, end). Unlike a
	// session date, LoggedAt is an instant, so the bounds are instants too — the
	// caller decides which zone's days they stand for.
	FindByUserInRange(userId uuid.UUID, start, end time.Time) ([]*entities.WorkoutResult, error)
	Delete(id uuid.UUID) error
}
