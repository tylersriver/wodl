package entities

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

type LiftCategory string

const (
	LiftCategorySquat    LiftCategory = "squat"
	LiftCategoryBench    LiftCategory = "bench"
	LiftCategoryDeadlift LiftCategory = "deadlift"
	LiftCategoryOHP      LiftCategory = "ohp"
	LiftCategoryClean    LiftCategory = "clean"
	LiftCategorySnatch   LiftCategory = "snatch"
	LiftCategoryCustom   LiftCategory = "custom"
)

func ValidLiftCategories() []LiftCategory {
	return []LiftCategory{
		LiftCategorySquat, LiftCategoryBench, LiftCategoryDeadlift,
		LiftCategoryOHP, LiftCategoryClean, LiftCategorySnatch, LiftCategoryCustom,
	}
}

type Lift struct {
	Id        uuid.UUID
	UserId    uuid.UUID
	Name      string
	Category  LiftCategory
	OneRepMax *float64
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time
}

func (l *Lift) validate() error {
	if l.Name == "" {
		return errors.New("lift name must not be empty")
	}
	if l.UserId == uuid.Nil {
		return errors.New("user id must not be empty")
	}
	valid := false
	for _, c := range ValidLiftCategories() {
		if l.Category == c {
			valid = true
			break
		}
	}
	if !valid {
		return errors.New("invalid lift category")
	}
	if l.OneRepMax != nil && *l.OneRepMax <= 0 {
		return errors.New("one rep max must be greater than 0")
	}
	return nil
}

func NewLift(userId uuid.UUID, name string, category LiftCategory, oneRepMax *float64) *Lift {
	now := time.Now()
	return &Lift{
		Id:        uuid.Must(uuid.NewV7()),
		UserId:    userId,
		Name:      name,
		Category:  category,
		OneRepMax: oneRepMax,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func (l *Lift) UpdateName(name string) {
	l.Name = name
	l.UpdatedAt = time.Now()
}

func (l *Lift) UpdateOneRepMax(orm *float64) {
	l.OneRepMax = orm
	l.UpdatedAt = time.Now()
}

type ValidatedLift struct {
	Lift
}

func NewValidatedLift(lift *Lift) (*ValidatedLift, error) {
	if err := lift.validate(); err != nil {
		return nil, err
	}
	return &ValidatedLift{Lift: *lift}, nil
}
