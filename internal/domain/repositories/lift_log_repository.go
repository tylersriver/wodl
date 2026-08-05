package repositories

import (
	"time"

	"github.com/google/uuid"
	"github.com/tyler/wodl/internal/domain/entities"
)

type LiftLogRepository interface {
	Create(log *entities.ValidatedLiftLog) (*entities.LiftLog, error)
	FindById(id uuid.UUID) (*entities.LiftLog, error)
	FindByLiftId(liftId uuid.UUID, limit int) ([]*entities.LiftLog, error)
	FindByUserId(userId uuid.UUID, limit int) ([]*entities.LiftLog, error)
	// FindByUserInRange selects the sets logged within [start, end). Unlike a
	// session date, LoggedAt is an instant, so the bounds are instants too — the
	// caller decides which zone's days they stand for.
	FindByUserInRange(userId uuid.UUID, start, end time.Time) ([]*entities.LiftLog, error)
	Delete(id uuid.UUID) error
}
