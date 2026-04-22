package query

import (
	"github.com/google/uuid"
	"github.com/tyler/wodl/internal/application/common"
)

type GetLiftsByUserQuery struct {
	UserId uuid.UUID
}

type GetLiftsByUserQueryResult struct {
	Results []*common.LiftResult
}

type GetLiftByIdQuery struct {
	Id     uuid.UUID
	UserId uuid.UUID
}

type GetLiftByIdQueryResult struct {
	Lift           *common.LiftResult
	Logs           []*common.LiftLogResult
	PercentageTable map[int]float64
}

type GetLiftLogsByLiftQuery struct {
	LiftId uuid.UUID
	Limit  int
}

type GetRecentLiftLogsQuery struct {
	UserId uuid.UUID
	Limit  int
}

type GetRecentLiftLogsQueryResult struct {
	Results []*common.LiftLogResult
}

type GetWorkoutsByUserQuery struct {
	UserId uuid.UUID
}

type GetWorkoutsByUserQueryResult struct {
	Results []*common.WorkoutResult
}

type GetWorkoutByIdQuery struct {
	Id     uuid.UUID
	UserId uuid.UUID
}

type GetWorkoutByIdQueryResult struct {
	Workout *common.WorkoutResult
	Results []*common.WorkoutResultResult
}

type GetRecentWorkoutResultsQuery struct {
	UserId uuid.UUID
	Limit  int
}

type GetRecentWorkoutResultsQueryResult struct {
	Results []*common.WorkoutResultResult
}

type GetSessionsByUserQuery struct {
	UserId uuid.UUID
}

type GetSessionsByUserQueryResult struct {
	Results []*common.SessionResult
}

type GetSessionByIdQuery struct {
	Id     uuid.UUID
	UserId uuid.UUID
}

type GetSessionByIdQueryResult struct {
	Session *common.SessionResult
}
