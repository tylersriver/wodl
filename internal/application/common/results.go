package common

import (
	"time"

	"github.com/google/uuid"
)

type UserResult struct {
	Id          uuid.UUID
	Email       string
	DisplayName string
	CreatedAt   time.Time
}

type LiftResult struct {
	Id        uuid.UUID
	UserId    uuid.UUID
	Name      string
	Category  string
	OneRepMax *float64
	CreatedAt time.Time
	UpdatedAt time.Time
}

type LiftLogResult struct {
	Id           uuid.UUID
	UserId       uuid.UUID
	LiftId       uuid.UUID
	Weight       float64
	Reps         int
	Sets         int
	RPE          *float64
	Estimated1RM float64
	PercentOf1RM *float64
	Notes        string
	LoggedAt     time.Time
}

type WorkoutResult struct {
	Id              uuid.UUID
	UserId          uuid.UUID
	Name            string
	Type            string
	Description     string
	TimeCap         *int
	Rounds          *int
	IntervalSeconds *int
	LiftId          *uuid.UUID
	// Lift and PercentageTable are populated at view time for lifting-type
	// workouts so the template can render 1RM context without an extra fetch.
	Lift            *LiftResult
	PercentageTable map[int]float64
	PctKeys         []int
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type SessionResult struct {
	Id               uuid.UUID
	UserId           uuid.UUID
	Name             string
	Warmup           string
	TotalTimeMinutes *int
	Workouts         []*WorkoutResult
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type WorkoutResultResult struct {
	Id        uuid.UUID
	UserId    uuid.UUID
	WorkoutId uuid.UUID
	Score     string
	ScoreType string
	Rx        bool
	Notes     string
	LoggedAt  time.Time
}

type AuthResult struct {
	Token string
	User  *UserResult
}
