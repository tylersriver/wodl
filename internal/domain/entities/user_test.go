package entities

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewUser_Valid(t *testing.T) {
	user := NewUser("test@example.com", "$2a$10$hashedpassword", "Test User")
	validated, err := NewValidatedUser(user)
	require.NoError(t, err)
	assert.Equal(t, "test@example.com", validated.Email)
	assert.Equal(t, "Test User", validated.DisplayName)
	assert.NotEmpty(t, validated.Id)
}

func TestNewUser_InvalidEmail(t *testing.T) {
	user := NewUser("notanemail", "hash", "Name")
	_, err := NewValidatedUser(user)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid email")
}

func TestNewUser_EmptyDisplayName(t *testing.T) {
	user := NewUser("test@example.com", "hash", "")
	_, err := NewValidatedUser(user)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "display name")
}

func TestNewUser_EmptyPasswordHash(t *testing.T) {
	user := NewUser("test@example.com", "", "Name")
	_, err := NewValidatedUser(user)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "password hash")
}
