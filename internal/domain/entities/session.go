package entities

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// Session groups an ordered list of workouts into a single training session
// with an optional free-text warmup and an estimated total duration.
type Session struct {
	Id               uuid.UUID
	UserId           uuid.UUID
	Name             string
	Warmup           string
	TotalTimeMinutes *int
	WorkoutIds       []uuid.UUID
	CreatedAt        time.Time
	UpdatedAt        time.Time
	DeletedAt        *time.Time
}

func (s *Session) validate() error {
	if s.Name == "" {
		return errors.New("session name must not be empty")
	}
	if s.UserId == uuid.Nil {
		return errors.New("user id must not be empty")
	}
	if s.TotalTimeMinutes != nil && *s.TotalTimeMinutes < 0 {
		return errors.New("total time must not be negative")
	}
	for _, id := range s.WorkoutIds {
		if id == uuid.Nil {
			return errors.New("workout id must not be empty")
		}
	}
	return nil
}

func NewSession(userId uuid.UUID, name, warmup string, totalTimeMinutes *int, workoutIds []uuid.UUID) *Session {
	now := time.Now()
	return &Session{
		Id:               uuid.Must(uuid.NewV7()),
		UserId:           userId,
		Name:             name,
		Warmup:           warmup,
		TotalTimeMinutes: totalTimeMinutes,
		WorkoutIds:       workoutIds,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
}

func (s *Session) UpdateName(name string) {
	s.Name = name
	s.UpdatedAt = time.Now()
}

func (s *Session) UpdateWarmup(warmup string) {
	s.Warmup = warmup
	s.UpdatedAt = time.Now()
}

func (s *Session) UpdateTotalTime(minutes *int) {
	s.TotalTimeMinutes = minutes
	s.UpdatedAt = time.Now()
}

func (s *Session) UpdateWorkoutIds(ids []uuid.UUID) {
	s.WorkoutIds = ids
	s.UpdatedAt = time.Now()
}

type ValidatedSession struct {
	Session
}

func NewValidatedSession(session *Session) (*ValidatedSession, error) {
	if err := session.validate(); err != nil {
		return nil, err
	}
	return &ValidatedSession{Session: *session}, nil
}
