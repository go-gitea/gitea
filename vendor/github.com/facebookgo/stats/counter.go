package stats

import "fmt"

// Type is the type of aggregation of apply
type Type int

const (
	AggregateAvg Type = iota
	AggregateSum
	AggregateHistogram
)

var (
	// HistogramPercentiles is used to determine which percentiles to return for
	// SimpleCounter.Aggregate
	HistogramPercentiles = map[string]float64{
		"p50": 0.5,
		"p95": 0.95,
		"p99": 0.99,
	}

	// MinSamplesForPercentiles is used by SimpleCounter.Aggregate to determine
	// what the minimum number of samples is required for percentile analysis
	MinSamplesForPercentiles = 10
)

// Aggregates can be used to merge counters together. This is not goroutine safe
type Aggregates map[string]Counter

// Add adds the counter for aggregation. This is not goroutine safe
func (a Aggregates) Add(c Counter) error {
	key := c.FullKey()
	if counter, ok := a[key]; ok {
		if counter.GetType() != c.GetType() {
			return fmt.Errorf("stats: mismatched aggregation type for: %s", key)
		}
		counter.AddValues(c.GetValues()...)
	} else {
		a[key] = c
	}
	return nil
}

// Counter is the interface used by Aggregates to merge counters together
type Counter interface {
	// FullKey is used to uniquely identify the counter
	FullKey() string

	// AddValues adds values for aggregation
	AddValues(...float64)

	// GetValues returns the values for aggregation
	GetValues() []float64

	// GetType returns the type of aggregation to apply
	GetType() Type
}

// SimpleCounter is a basic implementation of the Counter interface
type SimpleCounter struct {
	Key    string
	Values []float64
	Type   Type
}

// FullKey is part of the Counter interace
func (s *SimpleCounter) FullKey() string {
	return s.Key
}

// GetValues is part of the Counter interface
func (s *SimpleCounter) GetValues() []float64 {
	return s.Values
}

// AddValues is part of the Counter interface
func (s *SimpleCounter) AddValues(vs ...float64) {
	s.Values = append(s.Values, vs...)
}

// GetType is part of the Counter interface
func (s *SimpleCounter) GetType() Type {
	return s.Type
}

// Aggregate aggregates the provided values appropriately, returning a map
// from key to value. If AggregateHistogram is specified, the map will contain
// the relevant percentiles as specified by HistogramPercentiles
func (s *SimpleCounter) Aggregate() map[string]float64 {
	switch s.Type {
	case AggregateAvg:
		return map[string]float64{
			s.Key: Average(s.Values),
		}
	case AggregateSum:
		return map[string]float64{
			s.Key: Sum(s.Values),
		}
	case AggregateHistogram:
		histogram := map[string]float64{
			s.Key: Average(s.Values),
		}
		if len(s.Values) > MinSamplesForPercentiles {
			for k, v := range Percentiles(s.Values, HistogramPercentiles) {
				histogram[fmt.Sprintf("%s.%s", s.Key, k)] = v
			}
		}
		return histogram
	}
	panic("stats: unsupported aggregation type")
}
