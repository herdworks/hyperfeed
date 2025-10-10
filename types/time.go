package hyperfeed

import (
	"time"
)

// ============================ time.Time helpers =============================

// earliest returns the earliest non-zero time in times, or zero if all are zero.
func earliest(times ...time.Time) time.Time {
	var earliest time.Time = time.Time{}
	for _, t := range times {
		if !t.IsZero() && earliest.IsZero() {
			earliest = t
		} else if !t.IsZero() && t.Before(earliest) {
			earliest = t
		}
	}
	return earliest.UTC()
}

// latest returns the latest non-zero time in times, or zero if all are zero.
func latest(times ...time.Time) time.Time {
	var latest time.Time = time.Time{}
	for _, t := range times {
		if t.After(latest) {
			latest = t
		}
	}
	return latest.UTC()
}
