package command

import (
	"time"

	"github.com/google/uuid"
	"github.com/tyler/wodl/internal/application/common"
)

type CreateWorkoutCommand struct {
	UserId          uuid.UUID
	Name            string
	Type            string
	Description     string
	TimeCap         *int
	Rounds          *int
	IntervalSeconds *int
	LiftId          *uuid.UUID
}

type UpdateWorkoutCommand struct {
	Id              uuid.UUID
	UserId          uuid.UUID
	Name            string
	Type            string
	Description     string
	TimeCap         *int
	Rounds          *int
	IntervalSeconds *int
	LiftId          *uuid.UUID
}

type DeleteWorkoutCommand struct {
	Id     uuid.UUID
	UserId uuid.UUID
}

type CreateWorkoutResultCommand struct {
	UserId    uuid.UUID
	WorkoutId uuid.UUID
	Score     string
	ScoreType string
	Rx        bool
	Notes     string
	// LoggedAt is when the work was done, which is not always when it was
	// written up. The interface layer turns the day the user picked into an
	// instant, since only it knows the reader's zone.
	LoggedAt time.Time
}

type UpdateWorkoutResultCommand struct {
	Id        uuid.UUID
	UserId    uuid.UUID
	Score     string
	ScoreType string
	Rx        bool
	Notes     string
	LoggedAt  time.Time
}

type DeleteWorkoutResultCommand struct {
	Id     uuid.UUID
	UserId uuid.UUID
}

type CreateWorkoutCommandResult struct {
	Result *common.WorkoutResult
}

type CreateWorkoutResultCommandResult struct {
	Result *common.WorkoutResultResult
}
