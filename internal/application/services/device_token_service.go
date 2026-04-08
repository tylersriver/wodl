package services

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"

	"github.com/google/uuid"
	"github.com/tyler/wodl/internal/domain/entities"
	"github.com/tyler/wodl/internal/domain/repositories"
	"github.com/tyler/wodl/internal/infrastructure/auth"
)

type DeviceTokenService struct {
	deviceTokenRepo repositories.DeviceTokenRepository
	userRepo        repositories.UserRepository
	jwtService      *auth.JWTService
}

func NewDeviceTokenService(
	deviceTokenRepo repositories.DeviceTokenRepository,
	userRepo repositories.UserRepository,
	jwtService *auth.JWTService,
) *DeviceTokenService {
	return &DeviceTokenService{
		deviceTokenRepo: deviceTokenRepo,
		userRepo:        userRepo,
		jwtService:      jwtService,
	}
}

// CreateToken generates a device token for the authenticated user.
// Returns the raw token (to be stored on the device). Only the hash is persisted server-side.
func (s *DeviceTokenService) CreateToken(userId uuid.UUID, deviceName string) (string, error) {
	rawToken, err := generateSecureToken()
	if err != nil {
		return "", err
	}

	tokenHash := hashToken(rawToken)

	dt := entities.NewDeviceToken(userId, tokenHash, deviceName)
	if err := s.deviceTokenRepo.Create(dt); err != nil {
		return "", err
	}

	return rawToken, nil
}

// ExchangeToken validates a raw device token and returns a JWT session token.
func (s *DeviceTokenService) ExchangeToken(rawToken string) (string, error) {
	tokenHash := hashToken(rawToken)

	dt, err := s.deviceTokenRepo.FindByTokenHash(tokenHash)
	if err != nil {
		return "", err
	}
	if dt == nil {
		return "", errors.New("invalid device token")
	}

	// Verify the user still exists
	user, err := s.userRepo.FindById(dt.UserId)
	if err != nil {
		return "", err
	}
	if user == nil {
		// User was deleted; clean up the token
		s.deviceTokenRepo.DeleteById(dt.Id)
		return "", errors.New("invalid device token")
	}

	// Update last used timestamp
	s.deviceTokenRepo.UpdateLastUsed(dt.Id)

	// Issue a normal JWT session token
	return s.jwtService.GenerateToken(dt.UserId)
}

// RevokeAllTokens deletes all device tokens for a user (e.g., on password change).
func (s *DeviceTokenService) RevokeAllTokens(userId uuid.UUID) error {
	return s.deviceTokenRepo.DeleteByUserId(userId)
}

func generateSecureToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func hashToken(raw string) string {
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
}
