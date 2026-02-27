package engine

import "time"

// Clock abstracts time.Now() to allow deterministic testing.
// It is used by the Generator to determine "today" and handle date projections.
type Clock interface {
	Now() time.Time
}

// RealClock implements Clock using the standard time package.
type RealClock struct{}

// Now returns the current local time.
func (RealClock) Now() time.Time {
	return time.Now()
}
