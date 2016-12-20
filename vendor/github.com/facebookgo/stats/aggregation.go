package stats

import "sort"

// Average returns the average value
func Average(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	var val float64
	for _, point := range values {
		val += point
	}
	return val / float64(len(values))
}

// Sum returns the sum of all the given values
func Sum(values []float64) float64 {
	var val float64
	for _, point := range values {
		val += point
	}
	return val
}

// Percentiles returns a map containing the asked for percentiles
func Percentiles(values []float64, percentiles map[string]float64) map[string]float64 {
	sort.Float64s(values)
	results := map[string]float64{}
	for label, p := range percentiles {
		results[label] = values[int(float64(len(values))*p)]
	}
	return results
}
