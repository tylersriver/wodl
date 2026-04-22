package repositories

import (
	"time"

	"github.com/google/uuid"
	"github.com/tyler/wodl/internal/domain/entities"
)

// SessionLogWithName pairs a SessionLog with the name of its parent Session,
// letting the calendar view render both in a single query.
type SessionLogWithName struct {
	Log         *entities.SessionLog
	SessionName string
}

type SessionLogRepository interface {
	Create(log *entities.ValidatedSessionLog) (*entities.SessionLog, error)
	FindById(id uuid.UUID) (*entities.SessionLog, error)
	FindBySessionId(sessionId uuid.UUID) ([]*entities.SessionLog, error)
	FindByUserInRange(userId uuid.UUID, start, end time.Time) ([]*SessionLogWithName, error)
	Delete(id uuid.UUID) error
}
