package services

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/tyler/wodl/internal/application/command"
	"github.com/tyler/wodl/internal/application/common"
	"github.com/tyler/wodl/internal/application/query"
	"github.com/tyler/wodl/internal/domain/entities"
)

// BoardExtractor reads a session off images of a workout board. The application
// layer owns this port so the AI vendor stays an infrastructure detail — tests
// substitute a stub, and swapping providers touches one package.
type BoardExtractor interface {
	Extract(ctx context.Context, images []common.BoardImage) (*common.ExtractedSession, error)
}

// ImportService turns board images into a session. Extraction and creation are
// deliberately separate: Preview only reads, so the user can correct the model
// before Create writes anything.
type ImportService struct {
	extractor      BoardExtractor
	liftService    *LiftService
	workoutService *WorkoutService
	sessionService *SessionService
}

func NewImportService(extractor BoardExtractor, liftService *LiftService, workoutService *WorkoutService, sessionService *SessionService) *ImportService {
	return &ImportService{
		extractor:      extractor,
		liftService:    liftService,
		workoutService: workoutService,
		sessionService: sessionService,
	}
}

// Enabled reports whether an extractor is configured. Without credentials the
// feature hides itself rather than failing at upload time.
func (s *ImportService) Enabled() bool {
	return s != nil && s.extractor != nil
}

// Preview reads the images and returns a proposed session. Nothing is saved.
func (s *ImportService) Preview(ctx context.Context, images []common.BoardImage) (*common.ExtractedSession, error) {
	if !s.Enabled() {
		return nil, errors.New("image import is not configured")
	}
	if len(images) == 0 {
		return nil, errors.New("no images provided")
	}
	return s.extractor.Extract(ctx, images)
}

// Create saves a reviewed extraction. Lifts and workouts are matched to what
// the user already has by name, so importing the same benchmark each week
// keeps one workout with a continuous history instead of accruing copies.
func (s *ImportService) Create(cmd *command.ImportSessionCommand) (*command.ImportSessionCommandResult, error) {
	if len(cmd.Workouts) == 0 {
		return nil, errors.New("a session needs at least one workout")
	}

	existingLifts, err := s.liftService.GetLiftsByUser(&query.GetLiftsByUserQuery{UserId: cmd.UserId})
	if err != nil {
		return nil, err
	}
	existingWorkouts, err := s.workoutService.GetWorkoutsByUser(&query.GetWorkoutsByUserQuery{UserId: cmd.UserId})
	if err != nil {
		return nil, err
	}

	liftsByName := map[string]uuid.UUID{}
	if existingLifts != nil {
		for _, l := range existingLifts.Results {
			liftsByName[normalizeName(l.Name)] = l.Id
		}
	}
	workoutsByName := map[string]uuid.UUID{}
	if existingWorkouts != nil {
		for _, w := range existingWorkouts.Results {
			workoutsByName[normalizeName(w.Name)] = w.Id
		}
	}

	result := &command.ImportSessionCommandResult{}
	workoutIds := make([]uuid.UUID, 0, len(cmd.Workouts))

	for _, w := range cmd.Workouts {
		name := strings.TrimSpace(w.Name)
		if name == "" {
			return nil, errors.New("every workout needs a name")
		}

		var liftId *uuid.UUID
		if w.Type == string(entities.WorkoutTypeLifting) && strings.TrimSpace(w.LiftName) != "" {
			id, created, err := s.resolveLift(cmd.UserId, w.LiftName, w.LiftCategory, liftsByName)
			if err != nil {
				return nil, err
			}
			if created {
				result.CreatedLifts++
			} else {
				result.ReusedLifts++
			}
			liftId = &id
		}

		if id, ok := workoutsByName[normalizeName(name)]; ok {
			result.ReusedWorkouts++
			workoutIds = append(workoutIds, id)
			continue
		}

		created, err := s.workoutService.CreateWorkout(&command.CreateWorkoutCommand{
			UserId:          cmd.UserId,
			Name:            name,
			Type:            w.Type,
			Description:     w.Description,
			TimeCap:         w.TimeCap,
			Rounds:          w.Rounds,
			IntervalSeconds: w.IntervalSeconds,
			LiftId:          liftId,
		})
		if err != nil {
			return nil, fmt.Errorf("creating workout %q: %w", name, err)
		}
		result.CreatedWorkouts++
		workoutsByName[normalizeName(name)] = created.Result.Id
		workoutIds = append(workoutIds, created.Result.Id)
	}

	session, err := s.sessionService.CreateSession(&command.CreateSessionCommand{
		UserId:           cmd.UserId,
		Name:             cmd.Name,
		Warmup:           cmd.Warmup,
		Date:             cmd.Date,
		TotalTimeMinutes: cmd.TotalTimeMinutes,
		WorkoutIds:       workoutIds,
	})
	if err != nil {
		return nil, err
	}

	result.SessionId = session.Result.Id
	return result, nil
}

// resolveLift returns the id of an existing lift with this name, creating one
// when there is no match. The bool reports whether it had to create it.
func (s *ImportService) resolveLift(userId uuid.UUID, name, category string, known map[string]uuid.UUID) (uuid.UUID, bool, error) {
	name = strings.TrimSpace(name)
	if id, ok := known[normalizeName(name)]; ok {
		return id, false, nil
	}

	if category == "" {
		category = string(entities.LiftCategoryCustom)
	}
	created, err := s.liftService.CreateLift(&command.CreateLiftCommand{
		UserId:   userId,
		Name:     name,
		Category: category,
	})
	if err != nil {
		return uuid.Nil, false, fmt.Errorf("creating lift %q: %w", name, err)
	}
	known[normalizeName(name)] = created.Result.Id
	return created.Result.Id, true, nil
}

// normalizeName is the matching key for reusing an existing lift or workout —
// case and surrounding whitespace shouldn't make "Back Squat" a second lift.
func normalizeName(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(s), " "))
}
