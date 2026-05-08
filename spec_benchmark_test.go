package cron

import (
	"testing"
	"time"
)

func BenchmarkSpecScheduleNext(b *testing.B) {
	benchmarks := []struct {
		name string
		spec string
	}{
		{name: "EveryMinute", spec: "* * * * *"},
		{name: "EveryHour", spec: "0 * * * *"},
		{name: "EveryDay", spec: "0 0 * * *"},
		{name: "EveryWeek", spec: "0 0 * * 0"},
		{name: "EveryMonth", spec: "0 0 1 * *"},
	}

	start := time.Date(2024, time.January, 1, 0, 0, 0, 0, time.UTC)

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			sched, err := Parse(bm.spec)
			if err != nil {
				b.Fatalf("Parse(%q): %v", bm.spec, err)
			}

			t := start
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = sched.Next(t)
			}
		})
	}
}
