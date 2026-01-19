package clock

import "time"

// Clock provides time-related functions that can be mocked for testing
type Clock interface {
	Now() time.Time
}

// RealClock implements Clock using actual system time
type RealClock struct{}

// Now returns the current system time
func (RealClock) Now() time.Time {
	return time.Now()
}
