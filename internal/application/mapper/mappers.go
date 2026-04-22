package mapper

import (
	"github.com/tyler/wodl/internal/application/common"
	"github.com/tyler/wodl/internal/domain/entities"
)

func UserToResult(u *entities.User) *common.UserResult {
	if u == nil {
		return nil
	}
	return &common.UserResult{
		Id:          u.Id,
		Email:       u.Email,
		DisplayName: u.DisplayName,
		CreatedAt:   u.CreatedAt,
	}
}

func LiftToResult(l *entities.Lift) *common.LiftResult {
	if l == nil {
		return nil
	}
	return &common.LiftResult{
		Id:        l.Id,
		UserId:    l.UserId,
		Name:      l.Name,
		Category:  string(l.Category),
		OneRepMax: l.OneRepMax,
		CreatedAt: l.CreatedAt,
		UpdatedAt: l.UpdatedAt,
	}
}

func LiftLogToResult(ll *entities.LiftLog) *common.LiftLogResult {
	if ll == nil {
		return nil
	}
	return &common.LiftLogResult{
		Id:           ll.Id,
		UserId:       ll.UserId,
		LiftId:       ll.LiftId,
		Weight:       ll.Weight,
		Reps:         ll.Reps,
		Sets:         ll.Sets,
		RPE:          ll.RPE,
		Estimated1RM: ll.Estimated1RM,
		PercentOf1RM: ll.PercentOf1RM,
		Notes:        ll.Notes,
		LoggedAt:     ll.LoggedAt,
	}
}

func WorkoutToResult(w *entities.Workout) *common.WorkoutResult {
	if w == nil {
		return nil
	}
	return &common.WorkoutResult{
		Id:              w.Id,
		UserId:          w.UserId,
		Name:            w.Name,
		Type:            string(w.Type),
		Description:     w.Description,
		TimeCap:         w.TimeCap,
		Rounds:          w.Rounds,
		IntervalSeconds: w.IntervalSeconds,
		LiftId:          w.LiftId,
		CreatedAt:       w.CreatedAt,
		UpdatedAt:       w.UpdatedAt,
	}
}

func SessionToResult(s *entities.Session, workouts []*entities.Workout) *common.SessionResult {
	if s == nil {
		return nil
	}
	var ws []*common.WorkoutResult
	for _, w := range workouts {
		ws = append(ws, WorkoutToResult(w))
	}
	return &common.SessionResult{
		Id:               s.Id,
		UserId:           s.UserId,
		Name:             s.Name,
		Warmup:           s.Warmup,
		TotalTimeMinutes: s.TotalTimeMinutes,
		Workouts:         ws,
		CreatedAt:        s.CreatedAt,
		UpdatedAt:        s.UpdatedAt,
	}
}

func WorkoutResultToResult(wr *entities.WorkoutResult) *common.WorkoutResultResult {
	if wr == nil {
		return nil
	}
	return &common.WorkoutResultResult{
		Id:        wr.Id,
		UserId:    wr.UserId,
		WorkoutId: wr.WorkoutId,
		Score:     wr.Score,
		ScoreType: string(wr.ScoreType),
		Rx:        wr.Rx,
		Notes:     wr.Notes,
		LoggedAt:  wr.LoggedAt,
	}
}
