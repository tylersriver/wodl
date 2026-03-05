package entities

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewLift_Valid(t *testing.T) {
	userId := uuid.Must(uuid.NewV7())
	lift := NewLift(userId, "Back Squat", LiftCategorySquat, nil)
	validated, err := NewValidatedLift(lift)
	require.NoError(t, err)
	assert.Equal(t, "Back Squat", validated.Name)
	assert.Equal(t, LiftCategorySquat, validated.Category)
}

func TestNewLift_WithOneRepMax(t *testing.T) {
	userId := uuid.Must(uuid.NewV7())
	orm := 315.0
	lift := NewLift(userId, "Deadlift", LiftCategoryDeadlift, &orm)
	validated, err := NewValidatedLift(lift)
	require.NoError(t, err)
	assert.Equal(t, 315.0, *validated.OneRepMax)
}

func TestNewLift_EmptyName(t *testing.T) {
	userId := uuid.Must(uuid.NewV7())
	lift := NewLift(userId, "", LiftCategorySquat, nil)
	_, err := NewValidatedLift(lift)
	assert.Error(t, err)
}

func TestNewLift_InvalidCategory(t *testing.T) {
	userId := uuid.Must(uuid.NewV7())
	lift := NewLift(userId, "Test", LiftCategory("invalid"), nil)
	_, err := NewValidatedLift(lift)
	assert.Error(t, err)
}

func TestNewLift_ZeroOneRepMax(t *testing.T) {
	userId := uuid.Must(uuid.NewV7())
	orm := 0.0
	lift := NewLift(userId, "Bench", LiftCategoryBench, &orm)
	_, err := NewValidatedLift(lift)
	assert.Error(t, err)
}
