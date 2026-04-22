package command

import (
	"github.com/google/uuid"
	"github.com/tyler/wodl/internal/application/common"
)

type CreateSessionCommand struct {
	UserId           uuid.UUID
	Name             string
	Warmup           string
	TotalTimeMinutes *int
	WorkoutIds       []uuid.UUID
}

type UpdateSessionCommand struct {
	Id               uuid.UUID
	UserId           uuid.UUID
	Name             string
	Warmup           string
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
