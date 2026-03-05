package services

import (
	"errors"

	"github.com/tyler/wodl/internal/application/command"
	"github.com/tyler/wodl/internal/application/mapper"
	"github.com/tyler/wodl/internal/application/query"
	"github.com/tyler/wodl/internal/domain/entities"
	"github.com/tyler/wodl/internal/domain/repositories"
)

type LiftService struct {
	liftRepo    repositories.LiftRepository
	liftLogRepo repositories.LiftLogRepository
}

func NewLiftService(liftRepo repositories.LiftRepository, liftLogRepo repositories.LiftLogRepository) *LiftService {
	return &LiftService{liftRepo: liftRepo, liftLogRepo: liftLogRepo}
}

func (s *LiftService) CreateLift(cmd *command.CreateLiftCommand) (*command.CreateLiftCommandResult, error) {
	lift := entities.NewLift(cmd.UserId, cmd.Name, entities.LiftCategory(cmd.Category), cmd.OneRepMax)
	validated, err := entities.NewValidatedLift(lift)
	if err != nil {
		return nil, err
	}

	created, err := s.liftRepo.Create(validated)
	if err != nil {
		return nil, err
	}

	return &command.CreateLiftCommandResult{Result: mapper.LiftToResult(created)}, nil
}

func (s *LiftService) UpdateLift(cmd *command.UpdateLiftCommand) error {
	lift, err := s.liftRepo.FindById(cmd.Id)
	if err != nil {
		return err
	}
	if lift == nil {
		return errors.New("lift not found")
	}
	if lift.UserId != cmd.UserId {
		return errors.New("unauthorized")
	}

	lift.UpdateName(cmd.Name)
	lift.Category = entities.LiftCategory(cmd.Category)
	lift.UpdateOneRepMax(cmd.OneRepMax)

	validated, err := entities.NewValidatedLift(lift)
	if err != nil {
		return err
	}

	_, err = s.liftRepo.Update(validated)
	return err
}

func (s *LiftService) DeleteLift(cmd *command.DeleteLiftCommand) error {
	lift, err := s.liftRepo.FindById(cmd.Id)
	if err != nil {
		return err
	}
	if lift == nil {
		return errors.New("lift not found")
	}
	if lift.UserId != cmd.UserId {
		return errors.New("unauthorized")
	}
	return s.liftRepo.Delete(cmd.Id)
}

func (s *LiftService) CreateLiftLog(cmd *command.CreateLiftLogCommand) (*command.CreateLiftLogCommandResult, error) {
	lift, err := s.liftRepo.FindById(cmd.LiftId)
	if err != nil {
		return nil, err
	}
	if lift == nil {
		return nil, errors.New("lift not found")
	}
	if lift.UserId != cmd.UserId {
		return nil, errors.New("unauthorized")
	}

	log := entities.NewLiftLog(cmd.UserId, cmd.LiftId, cmd.Weight, cmd.Reps, cmd.Sets, cmd.RPE, cmd.Notes, lift.OneRepMax)
	validated, err := entities.NewValidatedLiftLog(log)
	if err != nil {
		return nil, err
	}

	created, err := s.liftLogRepo.Create(validated)
	if err != nil {
		return nil, err
	}

	return &command.CreateLiftLogCommandResult{Result: mapper.LiftLogToResult(created)}, nil
}

func (s *LiftService) DeleteLiftLog(cmd *command.DeleteLiftLogCommand) error {
	log, err := s.liftLogRepo.FindById(cmd.Id)
	if err != nil {
		return err
	}
	if log == nil {
		return errors.New("lift log not found")
	}
	if log.UserId != cmd.UserId {
		return errors.New("unauthorized")
	}
	return s.liftLogRepo.Delete(cmd.Id)
}

func (s *LiftService) GetLiftsByUser(q *query.GetLiftsByUserQuery) (*query.GetLiftsByUserQueryResult, error) {
	lifts, err := s.liftRepo.FindAllByUserId(q.UserId)
	if err != nil {
		return nil, err
	}

	var results query.GetLiftsByUserQueryResult
	for _, l := range lifts {
		results.Results = append(results.Results, mapper.LiftToResult(l))
	}
	return &results, nil
}

func (s *LiftService) GetLiftById(q *query.GetLiftByIdQuery) (*query.GetLiftByIdQueryResult, error) {
	lift, err := s.liftRepo.FindById(q.Id)
	if err != nil {
		return nil, err
	}
	if lift == nil {
		return nil, errors.New("lift not found")
	}
	if lift.UserId != q.UserId {
		return nil, errors.New("unauthorized")
	}

	logs, err := s.liftLogRepo.FindByLiftId(q.Id, 50)
	if err != nil {
		return nil, err
	}

	result := &query.GetLiftByIdQueryResult{
		Lift: mapper.LiftToResult(lift),
	}

	for _, l := range logs {
		result.Logs = append(result.Logs, mapper.LiftLogToResult(l))
	}

	if lift.OneRepMax != nil {
		result.PercentageTable = entities.PercentageTable(*lift.OneRepMax)
	}

	return result, nil
}

func (s *LiftService) GetRecentLiftLogs(q *query.GetRecentLiftLogsQuery) (*query.GetRecentLiftLogsQueryResult, error) {
	logs, err := s.liftLogRepo.FindByUserId(q.UserId, q.Limit)
	if err != nil {
		return nil, err
	}

	var results query.GetRecentLiftLogsQueryResult
	for _, l := range logs {
		results.Results = append(results.Results, mapper.LiftLogToResult(l))
	}
	return &results, nil
}
