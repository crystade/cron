package cron

import (
	"strings"
	"testing"
	"time"
)

func TestComputeMinInterval(t *testing.T) {
	tests := []struct {
		name     string
		minute   uint64
		hour     uint64
		expected time.Duration
	}{
		{
			name:     "every minute",
			minute:   all(minutes),
			hour:     all(hours),
			expected: 1 * time.Minute,
		},
		{
			name:     "every 5 minutes",
			minute:   getBits(0, 59, 5),
			hour:     all(hours),
			expected: 5 * time.Minute,
		},
		{
			name:     "every 3 minutes",
			minute:   getBits(0, 59, 3),
			hour:     all(hours),
			expected: 3 * time.Minute,
		},
		{
			name:     "every 15 minutes",
			minute:   getBits(0, 59, 15),
			hour:     all(hours),
			expected: 15 * time.Minute,
		},
		{
			name:     "every 30 minutes",
			minute:   getBits(0, 59, 30),
			hour:     all(hours),
			expected: 30 * time.Minute,
		},
		{
			name:     "once per hour at :00",
			minute:   1 << 0,
			hour:     all(hours),
			expected: 60 * time.Minute,
		},
		{
			name:     "once per hour at :45",
			minute:   1 << 45,
			hour:     all(hours),
			expected: 60 * time.Minute,
		},
		{
			name:     "twice per hour at :00 and :30",
			minute:   1<<0 | 1<<30,
			hour:     all(hours),
			expected: 30 * time.Minute,
		},
		{
			name:     "specific hours 9 and 17, minute 0",
			minute:   1 << 0,
			hour:     1<<9 | 1<<17,
			expected: 8 * time.Hour,
		},
		{
			name:     "single fire per day (9:00)",
			minute:   1 << 0,
			hour:     1 << 9,
			expected: 24 * time.Hour,
		},
		{
			name:     "every 7 minutes",
			minute:   getBits(0, 59, 7),
			hour:     all(hours),
			expected: 4 * time.Minute, // wrap: 56 -> 0 next hour = 4 min gap
		},
		{
			name:     "every 20 minutes",
			minute:   getBits(0, 59, 20),
			hour:     all(hours),
			expected: 20 * time.Minute,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			actual := computeMinInterval(tc.minute, tc.hour)
			if actual != tc.expected {
				t.Errorf("expected %v, got %v", tc.expected, actual)
			}
		})
	}
}

func TestCorrectToMinInterval(t *testing.T) {
	tests := []struct {
		name        string
		minInterval time.Duration
		expectMin   time.Duration // minimum interval of the corrected schedule
	}{
		{
			name:        "correct to 5m",
			minInterval: 5 * time.Minute,
			expectMin:   5 * time.Minute,
		},
		{
			name:        "correct to 7m",
			minInterval: 7 * time.Minute,
			expectMin:   4 * time.Minute, // */7 gives 0,7,14,21,28,35,42,49,56 -> wrap gap is 4m
		},
		{
			name:        "correct to 10m",
			minInterval: 10 * time.Minute,
			expectMin:   10 * time.Minute,
		},
		{
			name:        "correct to 15m",
			minInterval: 15 * time.Minute,
			expectMin:   15 * time.Minute,
		},
		{
			name:        "correct to 60m",
			minInterval: 60 * time.Minute,
			expectMin:   60 * time.Minute,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			minute, hour := correctToMinInterval(tc.minInterval)
			actual := computeMinInterval(minute, hour)
			if actual != tc.expectMin {
				t.Errorf("expected min interval %v after correction, got %v", tc.expectMin, actual)
			}
		})
	}
}

func TestParserMinIntervalError(t *testing.T) {
	tests := []struct {
		name        string
		spec        string
		minInterval time.Duration
		errContains string
	}{
		{
			name:        "every 3m violates 5m minimum",
			spec:        "*/3 * * * *",
			minInterval: 5 * time.Minute,
			errContains: "more frequent than the minimum interval",
		},
		{
			name:        "every 1m violates 10m minimum",
			spec:        "* * * * *",
			minInterval: 10 * time.Minute,
			errContains: "more frequent than the minimum interval",
		},
		{
			name:        "every 5m violates 15m minimum",
			spec:        "*/5 * * * *",
			minInterval: 15 * time.Minute,
			errContains: "more frequent than the minimum interval",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := &Parser{
				MinInterval:           tc.minInterval,
				MinIntervalCorrection: false,
			}
			schedule, err := p.Parse(tc.spec)
			if err == nil {
				t.Fatalf("expected error, got schedule: %v", schedule)
			}
			if !strings.Contains(err.Error(), tc.errContains) {
				t.Errorf("expected error containing %q, got: %v", tc.errContains, err)
			}
		})
	}
}

func TestParserMinIntervalCorrection(t *testing.T) {
	tests := []struct {
		name        string
		spec        string
		minInterval time.Duration
	}{
		{
			name:        "*/3 corrected to 5m",
			spec:        "*/3 * * * *",
			minInterval: 5 * time.Minute,
		},
		{
			name:        "* corrected to 10m",
			spec:        "* * * * *",
			minInterval: 10 * time.Minute,
		},
		{
			name:        "*/2 corrected to 15m",
			spec:        "*/2 * * * *",
			minInterval: 15 * time.Minute,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := &Parser{
				MinInterval:           tc.minInterval,
				MinIntervalCorrection: true,
			}
			schedule, err := p.Parse(tc.spec)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Verify by computing two consecutive Next() calls.
			now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
			first := schedule.Next(now)
			second := schedule.Next(first)
			gap := second.Sub(first)

			if gap < tc.minInterval {
				t.Errorf("expected gap >= %v, got %v (first=%v, second=%v)",
					tc.minInterval, gap, first, second)
			}
		})
	}
}

func TestParserMinIntervalPassesWhenNotViolated(t *testing.T) {
	tests := []struct {
		name        string
		spec        string
		minInterval time.Duration
	}{
		{
			name:        "every 10m with 5m minimum",
			spec:        "*/10 * * * *",
			minInterval: 5 * time.Minute,
		},
		{
			name:        "hourly with 30m minimum",
			spec:        "0 * * * *",
			minInterval: 30 * time.Minute,
		},
		{
			name:        "daily at 9am with 1h minimum",
			spec:        "0 9 * * *",
			minInterval: 1 * time.Hour,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := &Parser{
				MinInterval:           tc.minInterval,
				MinIntervalCorrection: false,
			}
			_, err := p.Parse(tc.spec)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestParserMinIntervalValidation(t *testing.T) {
	tests := []struct {
		name        string
		minInterval time.Duration
		errContains string
	}{
		{
			name:        "below 1m",
			minInterval: 30 * time.Second,
			errContains: "below the allowed minimum of 1m",
		},
		{
			name:        "above 1h",
			minInterval: 2 * time.Hour,
			errContains: "exceeds the allowed maximum of 1h",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := &Parser{
				MinInterval: tc.minInterval,
			}
			_, err := p.Parse("* * * * *")
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tc.errContains) {
				t.Errorf("expected error containing %q, got: %v", tc.errContains, err)
			}
		})
	}
}
