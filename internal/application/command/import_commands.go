package command

import (
	"time"

	"github.com/google/uuid"
)

// ImportSessionCommand creates a session from a board extraction the user has
// reviewed. It mirrors common.ExtractedSession, but carries what the user
// actually confirmed rather than what the model proposed.
type ImportSessionCommand struct {
	UserId           uuid.UUID
	Name             string
	Date             time.Time
	Warmup           string
	TotalTimeMinutes *int
	Workouts         []ImportWorkout
}

type ImportWorkout struct {
	Name            string
	Type            string
	Description     string
	TimeCap         *int
	Rounds          *int
	IntervalSeconds *int
	LiftName        string
	LiftCategory    string
}

type ImportSessionCommandResult struct {
	SessionId uuid.UUID
	// Counts of what the import had to create, so the UI can tell the user what
	// was reused rather than duplicated.
	CreatedLifts    int
	CreatedWorkouts int
	ReusedLifts     int
	ReusedWorkouts  int
}
