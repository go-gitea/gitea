package stats

import "time"

// Stopper calls Client.BumpSum and Client.BumpHistogram when End'ed
type Stopper struct {
	Key    string
	Start  time.Time
	Client Client
}

// End the Stopper
func (s *Stopper) End() {
	since := time.Since(s.Start).Seconds() * 1000.0
	s.Client.BumpSum(s.Key+".total", since)
	s.Client.BumpHistogram(s.Key, since)
}
