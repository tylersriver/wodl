package services

import (
	"errors"
	"time"

	"github.com/tyler/wodl/internal/application/command"
	"github.com/tyler/wodl/internal/application/mapper"
	"github.com/tyler/wodl/internal/application/query"
	"github.com/tyler/wodl/internal/domain/entities"
	"github.com/tyler/wodl/internal/domain/repositories"
)

type WorkoutService struct {
	workoutRepo       repositories.WorkoutRepository
	workoutResultRepo repositories.WorkoutResultRepository
}

func NewWorkoutService(workoutRepo repositories.WorkoutRepository, workoutResultRepo repositories.WorkoutResultRepository) *WorkoutService {
	return &WorkoutService{workoutRepo: workoutRepo, workoutResultRepo: workoutResultRepo}
}

func (s *WorkoutService) CreateWorkout(cmd *command.CreateWorkoutCommand) (*command.CreateWorkoutCommandResult, error) {
	w := entities.NewWorkout(cmd.UserId, cmd.Name, entities.WorkoutType(cmd.Type), cmd.Description,
		cmd.TimeCap, cmd.Rounds, cmd.IntervalSeconds)
	w.LiftId = cmd.LiftId
	validated, err := entities.NewValidatedWorkout(w)
	if err != nil {
		return nil, err
	}

	created, err := s.workoutRepo.Create(validated)
	if err != nil {
		return nil, err
	}

	return &command.CreateWorkoutCommandResult{Result: mapper.WorkoutToResult(created)}, nil
}

func (s *WorkoutService) UpdateWorkout(cmd *command.UpdateWorkoutCommand) error {
	w, err := s.workoutRepo.FindById(cmd.Id)
	if err != nil {
		return err
	}
	if w == nil {
		return errors.New("workout not found")
	}
	if w.UserId != cmd.UserId {
		return errors.New("unauthorized")
	}

	w.UpdateName(cmd.Name)
	w.UpdateDescription(cmd.Description)
	w.Type = entities.WorkoutType(cmd.Type)
	w.TimeCap = cmd.TimeCap
	w.Rounds = cmd.Rounds
	w.IntervalSeconds = cmd.IntervalSeconds
	w.LiftId = cmd.LiftId

	validated, err := entities.NewValidatedWorkout(w)
	if err != nil {
		return err
	}

	_, err = s.workoutRepo.Update(validated)
	return err
}

func (s *WorkoutService) DeleteWorkout(cmd *command.DeleteWorkoutCommand) error {
	w, err := s.workoutRepo.FindById(cmd.Id)
	if err != nil {
		return err
	}
	if w == nil {
		return errors.New("workout not found")
	}
	if w.UserId != cmd.UserId {
		return errors.New("unauthorized")
	}
	return s.workoutRepo.Delete(cmd.Id)
}

func (s *WorkoutService) CreateWorkoutResult(cmd *command.CreateWorkoutResultCommand) (*command.CreateWorkoutResultCommandResult, error) {
	w, err := s.workoutRepo.FindById(cmd.WorkoutId)
	if err != nil {
		return nil, err
	}
	if w == nil {
		return nil, errors.New("workout not found")
	}
	if w.UserId != cmd.UserId {
		return nil, errors.New("unauthorized")
	}

	loggedAt := cmd.LoggedAt
	if loggedAt.IsZero() {
		loggedAt = time.Now()
	}
	result := entities.NewWorkoutResult(cmd.UserId, cmd.WorkoutId, cmd.Score,
		entities.ScoreType(cmd.ScoreType), cmd.Rx, cmd.Notes, loggedAt)
	validated, err := entities.NewValidatedWorkoutResult(result)
	if err != nil {
		return nil, err
	}

	created, err := s.workoutResultRepo.Create(validated)
	if err != nil {
		return nil, err
	}

	return &command.CreateWorkoutResultCommandResult{Result: mapper.WorkoutResultToResult(created)}, nil
}

// UpdateWorkoutResult revises a score already recorded, the day it was recorded
// for included — a score typed in against the wrong day is the ordinary way
// this goes wrong, and it is what decides which day the week grid ticks.
func (s *WorkoutService) UpdateWorkoutResult(cmd *command.UpdateWorkoutResultCommand) error {
	result, err := s.workoutResultRepo.FindById(cmd.Id)
	if err != nil {
		return err
	}
	if result == nil {
		return errors.New("workout result not found")
	}
	if result.UserId != cmd.UserId {
		return errors.New("unauthorized")
	}

	loggedAt := cmd.LoggedAt
	if loggedAt.IsZero() {
		loggedAt = result.LoggedAt
	}
	result.Update(cmd.Score, entities.ScoreType(cmd.ScoreType), cmd.Rx, cmd.Notes, loggedAt)

	validated, err := entities.NewValidatedWorkoutResult(result)
	if err != nil {
		return err
	}

	_, err = s.workoutResultRepo.Update(validated)
	return err
}

func (s *WorkoutService) DeleteWorkoutResult(cmd *command.DeleteWorkoutResultCommand) error {
	result, err := s.workoutResultRepo.FindById(cmd.Id)
	if err != nil {
		return err
	}
	if result == nil {
		return errors.New("workout result not found")
	}
	if result.UserId != cmd.UserId {
		return errors.New("unauthorized")
	}
	return s.workoutResultRepo.Delete(cmd.Id)
}

func (s *WorkoutService) GetWorkoutsByUser(q *query.GetWorkoutsByUserQuery) (*query.GetWorkoutsByUserQueryResult, error) {
	workouts, err := s.workoutRepo.FindAllByUserId(q.UserId)
	if err != nil {
		return nil, err
	}

	var results query.GetWorkoutsByUserQueryResult
	for _, w := range workouts {
		results.Results = append(results.Results, mapper.WorkoutToResult(w))
	}
	return &results, nil
}

func (s *WorkoutService) GetWorkoutById(q *query.GetWorkoutByIdQuery) (*query.GetWorkoutByIdQueryResult, error) {
	w, err := s.workoutRepo.FindById(q.Id)
	if err != nil {
		return nil, err
	}
	if w == nil {
		return nil, errors.New("workout not found")
	}
	if w.UserId != q.UserId {
		return nil, errors.New("unauthorized")
	}

	results, err := s.workoutResultRepo.FindByWorkoutId(q.Id)
	if err != nil {
		return nil, err
	}

	result := &query.GetWorkoutByIdQueryResult{
		Workout: mapper.WorkoutToResult(w),
	}
	for _, r := range results {
		result.Results = append(result.Results, mapper.WorkoutResultToResult(r))
	}
	return result, nil
}

func (s *WorkoutService) GetWorkoutResultsInRange(q *query.GetWorkoutResultsInRangeQuery) (*query.GetWorkoutResultsInRangeQueryResult, error) {
	results, err := s.workoutResultRepo.FindByUserInRange(q.UserId, q.Start, q.End)
	if err != nil {
		return nil, err
	}

	var queryResult query.GetWorkoutResultsInRangeQueryResult
	for _, r := range results {
		queryResult.Results = append(queryResult.Results, mapper.WorkoutResultToResult(r))
	}
	return &queryResult, nil
}

func (s *WorkoutService) GetRecentWorkoutResults(q *query.GetRecentWorkoutResultsQuery) (*query.GetRecentWorkoutResultsQueryResult, error) {
	results, err := s.workoutResultRepo.FindByUserId(q.UserId, q.Limit)
	if err != nil {
		return nil, err
	}

	var queryResult query.GetRecentWorkoutResultsQueryResult
	for _, r := range results {
		queryResult.Results = append(queryResult.Results, mapper.WorkoutResultToResult(r))
	}
	return &queryResult, nil
}
