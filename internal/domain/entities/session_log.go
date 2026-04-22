package entities

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// SessionLog records a single occurrence of a Session being performed. A
// Session can have many SessionLogs, one per day the user did it.
type SessionLog struct {
	Id          uuid.UUID
	UserId      uuid.UUID
	SessionId   uuid.UUID
	PerformedAt time.Time
	Notes       string
	CreatedAt   time.Time
}

func (l *SessionLog) validate() error {
	if l.UserId == uuid.Nil {
		return errors.New("user id must not be empty")
	}
	if l.SessionId == uuid.Nil {
		return errors.New("session id must not be empty")
	}
	if l.PerformedAt.IsZero() {
		return errors.New("performed_at must not be empty")
	}
	return nil
}

func NewSessionLog(userId, sessionId uuid.UUID, performedAt time.Time, notes string) *SessionLog {
	return &SessionLog{
		Id:          uuid.Must(uuid.NewV7()),
		UserId:      userId,
		SessionId:   sessionId,
		PerformedAt: performedAt,
		Notes:       notes,
		CreatedAt:   time.Now(),
	}
}

type ValidatedSessionLog struct {
	SessionLog
}

func NewValidatedSessionLog(log *SessionLog) (*ValidatedSessionLog, error) {
	if err := log.validate(); err != nil {
		return nil, err
	}
	return &ValidatedSessionLog{SessionLog: *log}, nil
}
