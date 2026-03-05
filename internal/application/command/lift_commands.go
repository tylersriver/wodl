package command

import (
	"github.com/google/uuid"
	"github.com/tyler/wodl/internal/application/common"
)

type CreateLiftCommand struct {
	UserId    uuid.UUID
	Name      string
	Category  string
	OneRepMax *float64
}

type UpdateLiftCommand struct {
	Id        uuid.UUID
	UserId    uuid.UUID
	Name      string
	Category  string
	OneRepMax *float64
}

type DeleteLiftCommand struct {
	Id     uuid.UUID
	UserId uuid.UUID
}

type CreateLiftLogCommand struct {
	UserId uuid.UUID
	LiftId uuid.UUID
	Weight float64
	Reps   int
	Sets   int
	RPE    *float64
	Notes  string
}

type DeleteLiftLogCommand struct {
	Id     uuid.UUID
	UserId uuid.UUID
}

type CreateLiftCommandResult struct {
	Result *common.LiftResult
}

type CreateLiftLogCommandResult struct {
	Result *common.LiftLogResult
}
