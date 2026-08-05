package query

import (
	"time"

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
	Lift            *common.LiftResult
	Logs            []*common.LiftLogResult
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

// GetWorkoutResultsInRangeQuery selects the results logged within [Start, End).
// The bounds are instants, not civil dates: LoggedAt records the moment a score
// was written down, so the caller supplies the zone's day boundaries.
type GetWorkoutResultsInRangeQuery struct {
	UserId uuid.UUID
	Start  time.Time
	End    time.Time
}

type GetWorkoutResultsInRangeQueryResult struct {
	Results []*common.WorkoutResultResult
}

// GetLiftLogsInRangeQuery is the lift-side counterpart of
// GetWorkoutResultsInRangeQuery; sets are logged against the lift, not against
// the lifting workout that prescribed them.
type GetLiftLogsInRangeQuery struct {
	UserId uuid.UUID
	Start  time.Time
	End    time.Time
}

type GetLiftLogsInRangeQueryResult struct {
	Results []*common.LiftLogResult
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

// GetSessionsInRangeQuery selects sessions dated within [Start, End), backing
// the today, week and month views.
type GetSessionsInRangeQuery struct {
	UserId uuid.UUID
	Start  time.Time
	End    time.Time
}

type GetSessionsInRangeQueryResult struct {
	Results []*common.SessionResult
}
