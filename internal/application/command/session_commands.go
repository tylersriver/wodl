package command

import (
	"time"

	"github.com/google/uuid"
	"github.com/tyler/wodl/internal/application/common"
)

type CreateSessionCommand struct {
	UserId           uuid.UUID
	Name             string
	Warmup           string
	Date             *time.Time
	TotalTimeMinutes *int
	WorkoutIds       []uuid.UUID
}

type UpdateSessionCommand struct {
	Id               uuid.UUID
	UserId           uuid.UUID
	Name             string
	Warmup           string
	Date             *time.Time
	TotalTimeMinutes *int
	WorkoutIds       []uuid.UUID
}

type DeleteSessionCommand struct {
	Id     uuid.UUID
	UserId uuid.UUID
}

type CreateSessionCommandResult struct {
	Result *common.SessionResult
}

type CreateSessionLogCommand struct {
	UserId      uuid.UUID
	SessionId   uuid.UUID
	PerformedAt time.Time
	Notes       string
}

type DeleteSessionLogCommand struct {
	Id     uuid.UUID
	UserId uuid.UUID
}

type CreateSessionLogCommandResult struct {
	Result *common.SessionLogResult
}
