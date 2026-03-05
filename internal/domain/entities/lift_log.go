package entities

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

type LiftLog struct {
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
	CreatedAt    time.Time
}

func (ll *LiftLog) validate() error {
	if ll.UserId == uuid.Nil {
		return errors.New("user id must not be empty")
	}
	if ll.LiftId == uuid.Nil {
		return errors.New("lift id must not be empty")
	}
	if ll.Weight <= 0 {
		return errors.New("weight must be greater than 0")
	}
	if ll.Reps <= 0 {
		return errors.New("reps must be greater than 0")
	}
	if ll.Sets <= 0 {
		return errors.New("sets must be greater than 0")
	}
	if ll.RPE != nil && (*ll.RPE < 1 || *ll.RPE > 10) {
		return errors.New("RPE must be between 1 and 10")
	}
	return nil
}

func NewLiftLog(userId, liftId uuid.UUID, weight float64, reps, sets int, rpe *float64, notes string, liftOneRepMax *float64) *LiftLog {
	estimated1RM := EstimateOneRepMax(weight, float64(reps))

	var percentOf1RM *float64
	if liftOneRepMax != nil && *liftOneRepMax > 0 {
		p := PercentOf1RM(weight, *liftOneRepMax)
		percentOf1RM = &p
	}

	now := time.Now()
	return &LiftLog{
		Id:           uuid.Must(uuid.NewV7()),
		UserId:       userId,
		LiftId:       liftId,
		Weight:       weight,
		Reps:         reps,
		Sets:         sets,
		RPE:          rpe,
		Estimated1RM: estimated1RM,
		PercentOf1RM: percentOf1RM,
		Notes:        notes,
		LoggedAt:     now,
		CreatedAt:    now,
	}
}

type ValidatedLiftLog struct {
	LiftLog
}

func NewValidatedLiftLog(log *LiftLog) (*ValidatedLiftLog, error) {
	if err := log.validate(); err != nil {
		return nil, err
	}
	return &ValidatedLiftLog{LiftLog: *log}, nil
}
