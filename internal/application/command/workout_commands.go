package command

import (
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
	Sets            *int
	Reps            *int
	WorkTimeSeconds *int
	Percentage      *float64
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
	Sets            *int
	Reps            *int
	WorkTimeSeconds *int
	Percentage      *float64
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
