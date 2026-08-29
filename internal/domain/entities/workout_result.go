package entities

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

type ScoreType string

const (
	ScoreTypeReps       ScoreType = "reps"
	ScoreTypeTime       ScoreType = "time"
	ScoreTypeRoundsReps ScoreType = "rounds_and_reps"
	ScoreTypeLoad       ScoreType = "load"
	ScoreTypeCustom     ScoreType = "custom"
)

func ValidScoreTypes() []ScoreType {
	return []ScoreType{
		ScoreTypeReps, ScoreTypeTime, ScoreTypeRoundsReps,
		ScoreTypeLoad, ScoreTypeCustom,
	}
}

type WorkoutResult struct {
	Id        uuid.UUID
	UserId    uuid.UUID
	WorkoutId uuid.UUID
	Score     string
	ScoreType ScoreType
	Rx        bool
	Notes     string
	LoggedAt  time.Time
	CreatedAt time.Time
}

func (wr *WorkoutResult) validate() error {
	if wr.UserId == uuid.Nil {
		return errors.New("user id must not be empty")
	}
	if wr.WorkoutId == uuid.Nil {
		return errors.New("workout id must not be empty")
	}
	if wr.Score == "" {
		return errors.New("score must not be empty")
	}
	valid := false
	for _, t := range ValidScoreTypes() {
		if wr.ScoreType == t {
			valid = true
			break
		}
	}
	if !valid {
		return errors.New("invalid score type")
	}
	return nil
}

// NewWorkoutResult records a score against a workout.
//
// loggedAt is the moment the work was done, which is not always the moment it
// is typed in — a score can be written up the morning after. CreatedAt is the
// second of those and stays the clock's, so only LoggedAt is the caller's to
// choose.
func NewWorkoutResult(userId, workoutId uuid.UUID, score string, scoreType ScoreType, rx bool, notes string, loggedAt time.Time) *WorkoutResult {
	return &WorkoutResult{
		Id:        uuid.Must(uuid.NewV7()),
		UserId:    userId,
		WorkoutId: workoutId,
		Score:     score,
		ScoreType: scoreType,
		Rx:        rx,
		Notes:     notes,
		LoggedAt:  loggedAt,
		CreatedAt: time.Now(),
	}
}

// Update revises a score already recorded. Everything the user typed is theirs
// to correct, the day included; Id, WorkoutId and CreatedAt are not.
func (wr *WorkoutResult) Update(score string, scoreType ScoreType, rx bool, notes string, loggedAt time.Time) {
	wr.Score = score
	wr.ScoreType = scoreType
	wr.Rx = rx
	wr.Notes = notes
	wr.LoggedAt = loggedAt
}

type ValidatedWorkoutResult struct {
	WorkoutResult
}

func NewValidatedWorkoutResult(result *WorkoutResult) (*ValidatedWorkoutResult, error) {
	if err := result.validate(); err != nil {
		return nil, err
	}
	return &ValidatedWorkoutResult{WorkoutResult: *result}, nil
}
