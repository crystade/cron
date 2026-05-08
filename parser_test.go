package cron

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestRange(t *testing.T) {
	zero := uint64(0)
	ranges := []struct {
		expr     string
		min, max uint
		expected uint64
		err      string
	}{
		{"5", 0, 7, 1 << 5, ""},
		{"0", 0, 7, 1 << 0, ""},
		{"7", 0, 7, 1 << 7, ""},

		{"5-5", 0, 7, 1 << 5, ""},
		{"5-6", 0, 7, 1<<5 | 1<<6, ""},
		{"5-7", 0, 7, 1<<5 | 1<<6 | 1<<7, ""},

		{"5-6/2", 0, 7, 1 << 5, ""},
		{"5-7/2", 0, 7, 1<<5 | 1<<7, ""},
		{"5-7/1", 0, 7, 1<<5 | 1<<6 | 1<<7, ""},

		{"*", 1, 3, 1<<1 | 1<<2 | 1<<3 | starBit, ""},
		{"*/2", 1, 3, 1<<1 | 1<<3, ""},

		{"5--5", 0, 0, zero, "negative number"},
		{"jan-x", 0, 0, zero, "failed to parse int from"},
		{"2-x", 1, 5, zero, "failed to parse int from"},
		{"*/-12", 0, 0, zero, "negative number"},
		{"*//2", 0, 0, zero, "failed to parse int from"},
		{"1", 3, 5, zero, "below minimum"},
		{"6", 3, 5, zero, "above maximum"},
		{"5-3", 3, 5, zero, "beyond end of range"},
		{"*/0", 0, 0, zero, "should be a positive number"},
	}

	for _, c := range ranges {
		actual, err := getRange(c.expr, bounds{c.min, c.max, nil})
		if len(c.err) != 0 && (err == nil || !strings.Contains(err.Error(), c.err)) {
			t.Errorf("%s => expected %v, got %v", c.expr, c.err, err)
		}
		if len(c.err) == 0 && err != nil {
			t.Errorf("%s => unexpected error %v", c.expr, err)
		}
		if actual != c.expected {
			t.Errorf("%s => expected %d, got %d", c.expr, c.expected, actual)
		}
	}
}

func TestField(t *testing.T) {
	fields := []struct {
		expr     string
		min, max uint
		expected uint64
	}{
		{"5", 1, 7, 1 << 5},
		{"5,6", 1, 7, 1<<5 | 1<<6},
		{"5,6,7", 1, 7, 1<<5 | 1<<6 | 1<<7},
		{"1,5-7/2,3", 1, 7, 1<<1 | 1<<5 | 1<<7 | 1<<3},
	}

	for _, c := range fields {
		actual, _ := getField(c.expr, bounds{c.min, c.max, nil})
		if actual != c.expected {
			t.Errorf("%s => expected %d, got %d", c.expr, c.expected, actual)
		}
	}
}

func TestAll(t *testing.T) {
	allBits := []struct {
		r        bounds
		expected uint64
	}{
		{minutes, 0xfffffffffffffff}, // 0-59: 60 ones
		{hours, 0xffffff},            // 0-23: 24 ones
		{dom, 0xfffffffe},            // 1-31: 31 ones, 1 zero
		{months, 0x1ffe},             // 1-12: 12 ones, 1 zero
		{dow, 0x7f},                  // 0-6: 7 ones
	}

	for _, c := range allBits {
		actual := all(c.r) // all() adds the starBit, so compensate for that.
		if c.expected|starBit != actual {
			t.Errorf("%d-%d/%d => expected %b, got %b",
				c.r.min, c.r.max, 1, c.expected|starBit, actual)
		}
	}
}

func TestBits(t *testing.T) {
	bits := []struct {
		min, max, step uint
		expected       uint64
	}{
		{0, 0, 1, 0x1},
		{1, 1, 1, 0x2},
		{1, 5, 2, 0x2a}, // 101010
		{1, 4, 2, 0xa},  // 1010
	}

	for _, c := range bits {
		actual := getBits(c.min, c.max, c.step)
		if c.expected != actual {
			t.Errorf("%d-%d/%d => expected %b, got %b",
				c.min, c.max, c.step, c.expected, actual)
		}
	}
}

func TestParseScheduleErrors(t *testing.T) {
	var tests = []struct{ expr, err string }{
		{"* 5 j * *", "failed to parse int from"},
		{"@every Xm", "expected exactly 5 fields"},
		{"@unrecognized", "expected exactly 5 fields"},
		{"* * * *", "expected exactly 5 fields"},
		{"", "empty spec string"},
		{"TZ=UTC  * * * * *", "expected exactly 5 fields"},
	}
	for _, c := range tests {
		actual, err := Parse(c.expr)
		if err == nil || !strings.Contains(err.Error(), c.err) {
			t.Errorf("%s => expected %v, got %v", c.expr, c.err, err)
		}
		if actual != nil {
			t.Errorf("expected nil schedule on error, got %v", actual)
		}
	}
}

func TestParseSchedule(t *testing.T) {
	tokyo, _ := time.LoadLocation("Asia/Tokyo")
	entries := []struct {
		expr     string
		expected Schedule
	}{
		{"5 * * * *", every5min(time.UTC)},
		{"CRON_TZ=UTC  5 * * * *", every5min(time.UTC)},
		{"CRON_TZ=Asia/Tokyo 5 * * * *", every5min(tokyo)},
		{
			expr: "5 * * * *",
			expected: &SpecSchedule{
				Second:   1 << seconds.min,
				Minute:   1 << 5,
				Hour:     all(hours),
				Dom:      all(dom),
				Month:    all(months),
				Dow:      all(dow),
				Location: time.UTC,
			},
		},
	}

	for _, c := range entries {
		actual, err := Parse(c.expr)
		if err != nil {
			t.Errorf("%s => unexpected error %v", c.expr, err)
		}
		if !reflect.DeepEqual(actual, c.expected) {
			t.Errorf("%s => expected %b, got %b", c.expr, c.expected, actual)
		}
	}
}

func TestParseWithYearLimit(t *testing.T) {
	expected := every5min(time.UTC)
	expected.(*SpecSchedule).YearLimit = 10

	actual, err := (&Parser{YearLimit: 10}).Parse("5 * * * *")
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Errorf("expected %b, got %b", expected, actual)
	}
}

func TestParseWithNegativeYearLimit(t *testing.T) {
	actual, err := (&Parser{YearLimit: -1}).Parse("5 * * * *")
	if err == nil || !strings.Contains(err.Error(), "year limit cannot be negative") {
		t.Errorf("expected negative year limit error, got %v", err)
	}
	if actual != nil {
		t.Errorf("expected nil schedule on error, got %v", actual)
	}
}

func TestParseLocationErrors(t *testing.T) {
	tests := []struct{ expr, err string }{
		{"CRON_TZ=", "expected expression after timezone"},
		{"CRON_TZ=Not_A_Timezone * * * * *", "provided bad location Not_A_Timezone"},
	}

	for _, c := range tests {
		actual, err := Parse(c.expr)
		if err == nil || !strings.Contains(err.Error(), c.err) {
			t.Errorf("%s => expected %v, got %v", c.expr, c.err, err)
		}
		if actual != nil {
			t.Errorf("expected nil schedule on error, got %v", actual)
		}
	}
}

func every5min(loc *time.Location) Schedule {
	return &SpecSchedule{
		1 << seconds.min,
		1 << 5,
		all(hours),
		all(dom),
		all(months),
		all(dow),
		loc,
		0,
	}
}
