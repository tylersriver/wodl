package entities

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEstimateOneRepMax_SingleRep(t *testing.T) {
	result := EstimateOneRepMax(225, 1)
	assert.Equal(t, 225.0, result)
}

func TestEstimateOneRepMax_Epley(t *testing.T) {
	// 200 lbs x 5 reps = 200 * (1 + 5/30) = 200 * 1.1667 = 233.33
	result := EstimateOneRepMax(200, 5)
	assert.InDelta(t, 233.33, result, 0.01)
}

func TestEstimateOneRepMax_TenReps(t *testing.T) {
	// 135 x 10 = 135 * (1 + 10/30) = 135 * 1.3333 = 180
	result := EstimateOneRepMax(135, 10)
	assert.InDelta(t, 180.0, result, 0.01)
}

func TestPercentOf1RM(t *testing.T) {
	result := PercentOf1RM(225, 300)
	assert.Equal(t, 75.0, result)
}

func TestPercentOf1RM_ZeroMax(t *testing.T) {
	result := PercentOf1RM(225, 0)
	assert.Equal(t, 0.0, result)
}

func TestPercentageTable(t *testing.T) {
	table := PercentageTable(400)
	assert.Equal(t, 200.0, table[50])
	assert.Equal(t, 300.0, table[75])
	assert.Equal(t, 400.0, table[100])
	assert.InDelta(t, 340.0, table[85], 0.01)
	assert.Len(t, table, 11) // 50,55,60,...,100
}
