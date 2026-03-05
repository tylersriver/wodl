package services

import (
	"errors"

	"github.com/tyler/wodl/internal/application/command"
	"github.com/tyler/wodl/internal/application/common"
	"github.com/tyler/wodl/internal/application/mapper"
	"github.com/tyler/wodl/internal/domain/entities"
	"github.com/tyler/wodl/internal/domain/repositories"
	"github.com/tyler/wodl/internal/infrastructure/auth"
)

type AuthService struct {
	userRepo   repositories.UserRepository
	jwtService *auth.JWTService
}

func NewAuthService(userRepo repositories.UserRepository, jwtService *auth.JWTService) *AuthService {
	return &AuthService{userRepo: userRepo, jwtService: jwtService}
}

func (s *AuthService) Register(cmd *command.RegisterUserCommand) (*common.AuthResult, error) {
	if len(cmd.Password) < 8 {
		return nil, errors.New("password must be at least 8 characters")
	}

	existing, err := s.userRepo.FindByEmail(cmd.Email)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, errors.New("email already registered")
	}

	hash, err := auth.HashPassword(cmd.Password)
	if err != nil {
		return nil, err
	}

	user := entities.NewUser(cmd.Email, hash, cmd.DisplayName)
	validated, err := entities.NewValidatedUser(user)
	if err != nil {
		return nil, err
	}

	created, err := s.userRepo.Create(validated)
	if err != nil {
		return nil, err
	}

	token, err := s.jwtService.GenerateToken(created.Id)
	if err != nil {
		return nil, err
	}

	return &common.AuthResult{
		Token: token,
		User:  mapper.UserToResult(created),
	}, nil
}

func (s *AuthService) Login(cmd *command.LoginCommand) (*common.AuthResult, error) {
	user, err := s.userRepo.FindByEmail(cmd.Email)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, errors.New("invalid email or password")
	}

	if !auth.CheckPassword(cmd.Password, user.PasswordHash) {
		return nil, errors.New("invalid email or password")
	}

	token, err := s.jwtService.GenerateToken(user.Id)
	if err != nil {
		return nil, err
	}

	return &common.AuthResult{
		Token: token,
		User:  mapper.UserToResult(user),
	}, nil
}
