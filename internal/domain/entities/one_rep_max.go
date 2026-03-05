package entities

// EstimateOneRepMax calculates estimated 1RM using the Epley formula.
func EstimateOneRepMax(weight, reps float64) float64 {
	if reps <= 1 {
		return weight
	}
	return weight * (1 + reps/30.0)
}

// PercentOf1RM calculates what percentage of 1RM a given weight represents.
func PercentOf1RM(weight, oneRepMax float64) float64 {
	if oneRepMax <= 0 {
		return 0
	}
	return (weight / oneRepMax) * 100
}

// PercentageTable returns a map of percentage (50-100 in 5% increments) to weight.
func PercentageTable(oneRepMax float64) map[int]float64 {
	table := make(map[int]float64)
	for pct := 50; pct <= 100; pct += 5 {
		table[pct] = oneRepMax * float64(pct) / 100.0
	}
	return table
}
