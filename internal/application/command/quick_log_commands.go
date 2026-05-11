package command

import (
	"time"

	"github.com/google/uuid"
	"github.com/tyler/wodl/internal/application/common"
)

// QuickLogSessionCommand bundles a same-day lift log + metcon result + session
// completion into a single transaction-shaped command. Either the lift or the
// metcon section may be omitted, but at least one must be present.
type QuickLogSessionCommand struct {
	UserId           uuid.UUID
	Date             *time.Time
	TotalTimeMinutes *int
	SessionName      string // optional; auto-generated when blank
	Warmup           string
	Notes            string

	Lift   *QuickLogLiftSection
	Metcon *QuickLogMetconSection
}

// QuickLogLiftSection captures the lift portion of a quick-log submission.
// Either ExistingLiftId is set to reuse an entity, or NewLift fields are set
// to create one inline.
type QuickLogLiftSection struct {
	ExistingLiftId *uuid.UUID

	NewLiftName      string
	NewLiftCategory  string
	NewLiftOneRepMax *float64

	Weight float64
	Reps   int
	Sets   int
	RPE    *float64
	Notes  string
}

// QuickLogMetconSection captures the metcon portion. As with the lift section,
// the user either picks an existing workout or fills the NewWorkout fields.
type QuickLogMetconSection struct {
	ExistingWorkoutId *uuid.UUID

	NewWorkoutName            string
	NewWorkoutType            string
	NewWorkoutDescription     string
	NewWorkoutTimeCap         *int
	NewWorkoutRounds          *int
	NewWorkoutIntervalSeconds *int

	Score     string
	ScoreType string
	Rx        bool
	Notes     string
}

type QuickLogSessionCommandResult struct {
	Session *common.SessionResult
}
