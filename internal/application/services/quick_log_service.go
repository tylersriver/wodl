package services

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tyler/wodl/internal/application/command"
	"github.com/tyler/wodl/internal/application/query"
)

// QuickLogService orchestrates the existing Lift, Workout, and Session services
// so the user can record a CrossFit-style "lift + metcon" day in a single
// submission instead of navigating four separate pages.
type QuickLogService struct {
	liftService    *LiftService
	workoutService *WorkoutService
	sessionService *SessionService
}

func NewQuickLogService(liftService *LiftService, workoutService *WorkoutService, sessionService *SessionService) *QuickLogService {
	return &QuickLogService{
		liftService:    liftService,
		workoutService: workoutService,
		sessionService: sessionService,
	}
}

// QuickLogSession runs the Lift, Workout, and Session writes in sequence. If a
// later step fails, earlier writes are not rolled back — Lifts/Workouts created
// inline persist as normal entities so the user can finish the flow without
// losing their inline-created definitions on a downstream error.
func (s *QuickLogService) QuickLogSession(cmd *command.QuickLogSessionCommand) (*command.QuickLogSessionCommandResult, error) {
	if cmd.Lift == nil && cmd.Metcon == nil {
		return nil, errors.New("must log at least a lift or a metcon")
	}

	var (
		liftName   string
		metconName string
		workoutIds []uuid.UUID
	)

	if cmd.Lift != nil {
		liftId, name, err := s.resolveLift(cmd.UserId, cmd.Lift)
		if err != nil {
			return nil, fmt.Errorf("lift: %w", err)
		}
		liftName = name

		_, err = s.liftService.CreateLiftLog(&command.CreateLiftLogCommand{
			UserId: cmd.UserId,
			LiftId: liftId,
			Weight: cmd.Lift.Weight,
			Reps:   cmd.Lift.Reps,
			Sets:   cmd.Lift.Sets,
			RPE:    cmd.Lift.RPE,
			Notes:  cmd.Lift.Notes,
		})
		if err != nil {
			return nil, fmt.Errorf("logging lift: %w", err)
		}
	}

	if cmd.Metcon != nil {
		workoutId, name, err := s.resolveWorkout(cmd.UserId, cmd.Metcon)
		if err != nil {
			return nil, fmt.Errorf("metcon: %w", err)
		}
		metconName = name
		workoutIds = append(workoutIds, workoutId)

		// Score is optional — when blank, the workout is still attached to the
		// session but no WorkoutResult is recorded.
		if strings.TrimSpace(cmd.Metcon.Score) != "" {
			_, err = s.workoutService.CreateWorkoutResult(&command.CreateWorkoutResultCommand{
				UserId:    cmd.UserId,
				WorkoutId: workoutId,
				Score:     cmd.Metcon.Score,
				ScoreType: cmd.Metcon.ScoreType,
				Rx:        cmd.Metcon.Rx,
				Notes:     cmd.Metcon.Notes,
			})
			if err != nil {
				return nil, fmt.Errorf("logging metcon result: %w", err)
			}
		}
	}

	sessionName := strings.TrimSpace(cmd.SessionName)
	if sessionName == "" {
		sessionName = autoSessionName(liftName, metconName, cmd.Date)
	}

	sessionResult, err := s.sessionService.CreateSession(&command.CreateSessionCommand{
		UserId:           cmd.UserId,
		Name:             sessionName,
		Warmup:           cmd.Warmup,
		Date:             cmd.Date,
		TotalTimeMinutes: cmd.TotalTimeMinutes,
		WorkoutIds:       workoutIds,
	})
	if err != nil {
		return nil, fmt.Errorf("creating session: %w", err)
	}

	performed := time.Now()
	if cmd.Date != nil {
		performed = *cmd.Date
	}
	_, err = s.sessionService.CreateSessionLog(&command.CreateSessionLogCommand{
		UserId:      cmd.UserId,
		SessionId:   sessionResult.Result.Id,
		PerformedAt: performed,
		Notes:       cmd.Notes,
	})
	if err != nil {
		return nil, fmt.Errorf("marking session done: %w", err)
	}

	return &command.QuickLogSessionCommandResult{Session: sessionResult.Result}, nil
}

// resolveLift returns the lift id either from the existing selection or by
// creating one inline. The display name is returned for session-name building.
func (s *QuickLogService) resolveLift(userId uuid.UUID, section *command.QuickLogLiftSection) (uuid.UUID, string, error) {
	if section.ExistingLiftId != nil {
		res, err := s.liftService.GetLiftById(&query.GetLiftByIdQuery{Id: *section.ExistingLiftId, UserId: userId})
		if err != nil {
			return uuid.Nil, "", err
		}
		return res.Lift.Id, res.Lift.Name, nil
	}

	if strings.TrimSpace(section.NewLiftName) == "" {
		return uuid.Nil, "", errors.New("lift name required")
	}

	created, err := s.liftService.CreateLift(&command.CreateLiftCommand{
		UserId:    userId,
		Name:      section.NewLiftName,
		Category:  section.NewLiftCategory,
		OneRepMax: section.NewLiftOneRepMax,
	})
	if err != nil {
		return uuid.Nil, "", err
	}
	return created.Result.Id, created.Result.Name, nil
}

// resolveWorkout returns the workout id either from the existing selection or
// by creating one inline. Lifting-type workouts are not created here — the
// quick-log flow models the lift via a LiftLog rather than a workout entry.
func (s *QuickLogService) resolveWorkout(userId uuid.UUID, section *command.QuickLogMetconSection) (uuid.UUID, string, error) {
	if section.ExistingWorkoutId != nil {
		res, err := s.workoutService.GetWorkoutById(&query.GetWorkoutByIdQuery{Id: *section.ExistingWorkoutId, UserId: userId})
		if err != nil {
			return uuid.Nil, "", err
		}
		return res.Workout.Id, res.Workout.Name, nil
	}

	if strings.TrimSpace(section.NewWorkoutName) == "" {
		return uuid.Nil, "", errors.New("workout name required")
	}

	created, err := s.workoutService.CreateWorkout(&command.CreateWorkoutCommand{
		UserId:          userId,
		Name:            section.NewWorkoutName,
		Type:            section.NewWorkoutType,
		Description:     section.NewWorkoutDescription,
		TimeCap:         section.NewWorkoutTimeCap,
		Rounds:          section.NewWorkoutRounds,
		IntervalSeconds: section.NewWorkoutIntervalSeconds,
	})
	if err != nil {
		return uuid.Nil, "", err
	}
	return created.Result.Id, created.Result.Name, nil
}

// autoSessionName builds a sensible default like "Back Squat + Fran" when the
// user doesn't bother naming the session.
func autoSessionName(liftName, metconName string, date *time.Time) string {
	switch {
	case liftName != "" && metconName != "":
		return fmt.Sprintf("%s + %s", liftName, metconName)
	case metconName != "":
		return metconName
	case liftName != "":
		return liftName + " - Strength"
	}
	if date != nil {
		return "Session " + date.Format("Jan 2")
	}
	return "Session"
}
