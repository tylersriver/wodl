package entities

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

type WorkoutType string

const (
	WorkoutTypeAMRAP   WorkoutType = "amrap"
	WorkoutTypeForTime WorkoutType = "for_time"
	WorkoutTypeEMOM    WorkoutType = "emom"
	WorkoutTypeTabata  WorkoutType = "tabata"
	WorkoutTypeChipper WorkoutType = "chipper"
	WorkoutTypeLifting WorkoutType = "lifting"
	WorkoutTypeCustom  WorkoutType = "custom"
)

func ValidWorkoutTypes() []WorkoutType {
	return []WorkoutType{
		WorkoutTypeAMRAP, WorkoutTypeForTime, WorkoutTypeEMOM,
		WorkoutTypeTabata, WorkoutTypeChipper, WorkoutTypeLifting, WorkoutTypeCustom,
	}
}

type Workout struct {
	Id              uuid.UUID
	UserId          uuid.UUID
	Name            string
	Type            WorkoutType
	Description     string
	TimeCap         *int
	Rounds          *int
	IntervalSeconds *int
	// Lifting-specific fields (only meaningful when Type == WorkoutTypeLifting).
	LiftId          *uuid.UUID
	Sets            *int
	Reps            *int
	WorkTimeSeconds *int
	Percentage      *float64
	CreatedAt       time.Time
	UpdatedAt       time.Time
	DeletedAt       *time.Time
}

func (w *Workout) validate() error {
	if w.Name == "" {
		return errors.New("workout name must not be empty")
	}
	if w.UserId == uuid.Nil {
		return errors.New("user id must not be empty")
	}
	valid := false
	for _, t := range ValidWorkoutTypes() {
		if w.Type == t {
			valid = true
			break
		}
	}
	if !valid {
		return errors.New("invalid workout type")
	}
	if w.Percentage != nil && (*w.Percentage < 0 || *w.Percentage > 200) {
		return errors.New("percentage must be between 0 and 200")
	}
	return nil
}

func NewWorkout(userId uuid.UUID, name string, wType WorkoutType, description string, timeCap, rounds, interval *int) *Workout {
	now := time.Now()
	return &Workout{
		Id:              uuid.Must(uuid.NewV7()),
		UserId:          userId,
		Name:            name,
		Type:            wType,
		Description:     description,
		TimeCap:         timeCap,
		Rounds:          rounds,
		IntervalSeconds: interval,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
}

func (w *Workout) UpdateName(name string) {
	w.Name = name
	w.UpdatedAt = time.Now()
}

func (w *Workout) UpdateDescription(desc string) {
	w.Description = desc
	w.UpdatedAt = time.Now()
}

type ValidatedWorkout struct {
	Workout
}

func NewValidatedWorkout(workout *Workout) (*ValidatedWorkout, error) {
	if err := workout.validate(); err != nil {
		return nil, err
	}
	return &ValidatedWorkout{Workout: *workout}, nil
}
