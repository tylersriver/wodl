package entities

import (
	"errors"
	"net/mail"
	"time"

	"github.com/google/uuid"
)

type User struct {
	Id           uuid.UUID
	Email        string
	PasswordHash string
	DisplayName  string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (u *User) validate() error {
	if _, err := mail.ParseAddress(u.Email); err != nil {
		return errors.New("invalid email address")
	}
	if u.PasswordHash == "" {
		return errors.New("password hash must not be empty")
	}
	if u.DisplayName == "" {
		return errors.New("display name must not be empty")
	}
	return nil
}

func NewUser(email, passwordHash, displayName string) *User {
	now := time.Now()
	return &User{
		Id:           uuid.Must(uuid.NewV7()),
		Email:        email,
		PasswordHash: passwordHash,
		DisplayName:  displayName,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
}

type ValidatedUser struct {
	User
}

func NewValidatedUser(user *User) (*ValidatedUser, error) {
	if err := user.validate(); err != nil {
		return nil, err
	}
	return &ValidatedUser{User: *user}, nil
}
