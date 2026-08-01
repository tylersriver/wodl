package common

import "time"

// BoardImage is one uploaded photo or screenshot of a workout board, ready to
// be handed to an extractor.
type BoardImage struct {
	MediaType string
	Data      []byte
}

// ExtractedSession is a session read off one or more board images. It is a
// proposal, not a saved record — the user reviews and edits it before anything
// is written, so every field is best-effort and may be empty.
type ExtractedSession struct {
	Name string
	// Date is the day the board is for. Boards usually print a weekday and a
	// day-month with no year, so this is resolved against the current date at
	// extraction time; it is the zero value when no date could be read.
	Date             time.Time
	Warmup           string
	TotalTimeMinutes *int
	Workouts         []*ExtractedWorkout
}

// ExtractedWorkout is one piece of a board — a lift, a metcon, a finisher.
type ExtractedWorkout struct {
	Name string
	Type string
	// Description carries the board's own wording, including details the schema
	// has no field for (prescribed loads, "Score = Time", percentages of
	// something other than a one-rep max). Kept verbatim rather than
	// reinterpreted, so nothing is silently lost or misrepresented.
	Description     string
	TimeCapMinutes  *int
	Rounds          *int
	IntervalSeconds *int
	// LiftName and LiftCategory are set only for lifting pieces, and name the
	// barbell movement the piece is built around.
	LiftName     string
	LiftCategory string
}
