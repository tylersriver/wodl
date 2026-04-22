package services

import (
	"errors"

	"github.com/google/uuid"
	"github.com/tyler/wodl/internal/application/command"
	"github.com/tyler/wodl/internal/application/mapper"
	"github.com/tyler/wodl/internal/application/query"
	"github.com/tyler/wodl/internal/domain/entities"
	"github.com/tyler/wodl/internal/domain/repositories"
)

type SessionService struct {
	sessionRepo    repositories.SessionRepository
	workoutRepo    repositories.WorkoutRepository
	sessionLogRepo repositories.SessionLogRepository
}

func NewSessionService(sessionRepo repositories.SessionRepository, workoutRepo repositories.WorkoutRepository, sessionLogRepo repositories.SessionLogRepository) *SessionService {
	return &SessionService{
		sessionRepo:    sessionRepo,
		workoutRepo:    workoutRepo,
		sessionLogRepo: sessionLogRepo,
	}
}

func (s *SessionService) CreateSession(cmd *command.CreateSessionCommand) (*command.CreateSessionCommandResult, error) {
	if err := s.ensureWorkoutsOwned(cmd.UserId, cmd.WorkoutIds); err != nil {
		return nil, err
	}

	session := entities.NewSession(cmd.UserId, cmd.Name, cmd.Warmup, cmd.Date, cmd.TotalTimeMinutes, cmd.WorkoutIds)
	validated, err := entities.NewValidatedSession(session)
	if err != nil {
		return nil, err
	}

	created, err := s.sessionRepo.Create(validated)
	if err != nil {
		return nil, err
	}

	workouts, err := s.loadWorkouts(created.WorkoutIds)
	if err != nil {
		return nil, err
	}

	return &command.CreateSessionCommandResult{Result: mapper.SessionToResult(created, workouts)}, nil
}

func (s *SessionService) UpdateSession(cmd *command.UpdateSessionCommand) error {
	existing, err := s.sessionRepo.FindById(cmd.Id)
	if err != nil {
		return err
	}
	if existing == nil {
		return errors.New("session not found")
	}
	if existing.UserId != cmd.UserId {
		return errors.New("unauthorized")
	}

	if err := s.ensureWorkoutsOwned(cmd.UserId, cmd.WorkoutIds); err != nil {
		return err
	}

	existing.UpdateName(cmd.Name)
	existing.UpdateWarmup(cmd.Warmup)
	existing.UpdateDate(cmd.Date)
	existing.UpdateTotalTime(cmd.TotalTimeMinutes)
	existing.UpdateWorkoutIds(cmd.WorkoutIds)

	validated, err := entities.NewValidatedSession(existing)
	if err != nil {
		return err
	}

	_, err = s.sessionRepo.Update(validated)
	return err
}

func (s *SessionService) DeleteSession(cmd *command.DeleteSessionCommand) error {
	existing, err := s.sessionRepo.FindById(cmd.Id)
	if err != nil {
		return err
	}
	if existing == nil {
		return errors.New("session not found")
	}
	if existing.UserId != cmd.UserId {
		return errors.New("unauthorized")
	}
	return s.sessionRepo.Delete(cmd.Id)
}

func (s *SessionService) GetSessionsByUser(q *query.GetSessionsByUserQuery) (*query.GetSessionsByUserQueryResult, error) {
	sessions, err := s.sessionRepo.FindAllByUserId(q.UserId)
	if err != nil {
		return nil, err
	}

	var result query.GetSessionsByUserQueryResult
	for _, sess := range sessions {
		workouts, err := s.loadWorkouts(sess.WorkoutIds)
		if err != nil {
			return nil, err
		}
		result.Results = append(result.Results, mapper.SessionToResult(sess, workouts))
	}
	return &result, nil
}

func (s *SessionService) GetSessionById(q *query.GetSessionByIdQuery) (*query.GetSessionByIdQueryResult, error) {
	sess, err := s.sessionRepo.FindById(q.Id)
	if err != nil {
		return nil, err
	}
	if sess == nil {
		return nil, errors.New("session not found")
	}
	if sess.UserId != q.UserId {
		return nil, errors.New("unauthorized")
	}

	workouts, err := s.loadWorkouts(sess.WorkoutIds)
	if err != nil {
		return nil, err
	}

	logs, err := s.sessionLogRepo.FindBySessionId(sess.Id)
	if err != nil {
		return nil, err
	}

	result := mapper.SessionToResult(sess, workouts)
	for _, l := range logs {
		result.Logs = append(result.Logs, mapper.SessionLogToResult(l))
	}

	return &query.GetSessionByIdQueryResult{Session: result}, nil
}

func (s *SessionService) CreateSessionLog(cmd *command.CreateSessionLogCommand) (*command.CreateSessionLogCommandResult, error) {
	sess, err := s.sessionRepo.FindById(cmd.SessionId)
	if err != nil {
		return nil, err
	}
	if sess == nil {
		return nil, errors.New("session not found")
	}
	if sess.UserId != cmd.UserId {
		return nil, errors.New("unauthorized")
	}

	log := entities.NewSessionLog(cmd.UserId, cmd.SessionId, cmd.PerformedAt, cmd.Notes)
	validated, err := entities.NewValidatedSessionLog(log)
	if err != nil {
		return nil, err
	}

	created, err := s.sessionLogRepo.Create(validated)
	if err != nil {
		return nil, err
	}

	return &command.CreateSessionLogCommandResult{Result: mapper.SessionLogToResult(created)}, nil
}

func (s *SessionService) DeleteSessionLog(cmd *command.DeleteSessionLogCommand) error {
	log, err := s.sessionLogRepo.FindById(cmd.Id)
	if err != nil {
		return err
	}
	if log == nil {
		return errors.New("session log not found")
	}
	if log.UserId != cmd.UserId {
		return errors.New("unauthorized")
	}
	return s.sessionLogRepo.Delete(cmd.Id)
}

func (s *SessionService) GetSessionLogsInRange(q *query.GetSessionLogsInRangeQuery) (*query.GetSessionLogsInRangeQueryResult, error) {
	logs, err := s.sessionLogRepo.FindByUserInRange(q.UserId, q.Start, q.End)
	if err != nil {
		return nil, err
	}

	var result query.GetSessionLogsInRangeQueryResult
	for _, lw := range logs {
		r := mapper.SessionLogToResult(lw.Log)
		r.SessionName = lw.SessionName
		result.Results = append(result.Results, r)
	}
	return &result, nil
}

// ensureWorkoutsOwned verifies every workout id belongs to the given user; this
// prevents a user from embedding another user's workouts in their session.
func (s *SessionService) ensureWorkoutsOwned(userId uuid.UUID, workoutIds []uuid.UUID) error {
	for _, id := range workoutIds {
		w, err := s.workoutRepo.FindById(id)
		if err != nil {
			return err
		}
		if w == nil {
			return errors.New("workout not found")
		}
		if w.UserId != userId {
			return errors.New("unauthorized workout")
		}
	}
	return nil
}

// loadWorkouts fetches the workouts referenced by a session, preserving order
// and silently skipping any that have been soft-deleted.
func (s *SessionService) loadWorkouts(ids []uuid.UUID) ([]*entities.Workout, error) {
	workouts := make([]*entities.Workout, 0, len(ids))
	for _, id := range ids {
		w, err := s.workoutRepo.FindById(id)
		if err != nil {
			return nil, err
		}
		if w == nil {
			continue
		}
		workouts = append(workouts, w)
	}
	return workouts, nil
}

