package entities

import (
	"time"

	"github.com/google/uuid"
)

type DeviceToken struct {
	Id         uuid.UUID
	UserId     uuid.UUID
	TokenHash  string
	DeviceName string
	CreatedAt  time.Time
	LastUsedAt time.Time
}

func NewDeviceToken(userId uuid.UUID, tokenHash, deviceName string) *DeviceToken {
	now := time.Now()
	return &DeviceToken{
		Id:         uuid.Must(uuid.NewV7()),
		UserId:     userId,
		TokenHash:  tokenHash,
		DeviceName: deviceName,
		CreatedAt:  now,
		LastUsedAt: now,
	}
}
