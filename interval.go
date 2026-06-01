package cron

import (
	"fmt"
	"sort"
	"time"
)

// computeMinInterval computes the smallest gap between consecutive fire times
// within a 24-hour window, given the minute and hour bit fields from a parsed
// cron expression. Returns the minimum interval as a time.Duration.
//
// For expressions where dom/month/dow restrict firing to specific days, the
// actual interval is >= 24h, which always exceeds the 1h cap. Callers should
// only invoke this when interval checking is relevant.
func computeMinInterval(minute, hour uint64) time.Duration {
	minutes := extractBits(minute, 0, 59)
	hours := extractBits(hour, 0, 23)

	if len(minutes) == 0 || len(hours) == 0 {
		return 0
	}

	// Build all (hour, minute) fire times as minutes-since-midnight.
	times := make([]int, 0, len(hours)*len(minutes))
	for _, h := range hours {
		for _, m := range minutes {
			times = append(times, h*60+m)
		}
	}

	sort.Ints(times)

	if len(times) == 1 {
		// Fires once per day — interval is 24h.
		return 24 * time.Hour
	}

	// Find the minimum gap between consecutive fire times.
	minGap := 24 * 60 // start with max possible (full day in minutes)
	for i := 1; i < len(times); i++ {
		gap := times[i] - times[i-1]
		if gap < minGap {
			minGap = gap
		}
	}

	// Check wrap-around gap (last fire to first fire next day).
	wrapGap := (24*60 - times[len(times)-1]) + times[0]
	if wrapGap < minGap {
		minGap = wrapGap
	}

	return time.Duration(minGap) * time.Minute
}

// correctToMinInterval returns new minute and hour bit fields that enforce
// the given minimum interval. The interval is imposed exactly as the step
// (no rounding to "clean" divisors).
//
// For intervals <= 59m: fires at */step minutes, every hour.
// For interval == 60m: fires at minute 0, every hour.
func correctToMinInterval(minInterval time.Duration) (minute, hour uint64) {
	step := int(minInterval.Minutes())
	if step <= 0 {
		step = 1
	}

	if step >= 60 {
		// Fire once per hour at :00.
		return 1 << 0, all(hours)
	}

	// Generate minute bits with the given step starting from 0.
	minute = getBits(0, 59, uint(step))
	hour = all(hours)
	return minute, hour
}

// extractBits returns the positions of set bits in the given range [min, max],
// ignoring the star bit (bit 63).
func extractBits(bitfield uint64, min, max int) []int {
	bits := make([]int, 0, max-min+1)
	for i := min; i <= max; i++ {
		if bitfield&(1<<uint(i)) != 0 {
			bits = append(bits, i)
		}
	}
	return bits
}

// validateMinInterval checks that the MinInterval value is within the
// allowed range [1m, 1h]. Returns an error if out of range.
func validateMinInterval(d time.Duration) error {
	if d < time.Minute {
		return fmt.Errorf("minimum interval %v is below the allowed minimum of 1m", d)
	}
	if d > time.Hour {
		return fmt.Errorf("minimum interval %v exceeds the allowed maximum of 1h", d)
	}
	return nil
}
