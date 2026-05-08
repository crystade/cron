package cron

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

// Parser is a cron expression parser with configurable options.
// Parser is safe for concurrent use and can be reused across multiple Parse calls.
// It is compatible with TinyGo and WASM environments.
type Parser struct {
	// YearLimit is the maximum number of years to search for the next schedule.
	// Default is 5 years if not set or set to 0.
	YearLimit int
}

// NewParser creates a new Parser with the specified year limit.
// If yearLimit is 0 or negative, the default of 5 years will be used.
//
// Example:
//
//	parser := cron.NewParser(10) // Search up to 10 years ahead
//	schedule, err := parser.Parse("0 9 * * mon-fri")
func NewParser(yearLimit int) *Parser {
	return &Parser{YearLimit: yearLimit}
}

// Parse parses a 5-field cron expression and returns a schedule that can compute
// the next matching activation time. This is a convenience function that uses
// the default Parser configuration (5 year limit).
//
// For custom configuration, create a Parser with NewParser() or use a Parser literal.
//
// Example:
//
//	schedule, err := cron.Parse("0 9 * * mon-fri")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	next := schedule.Next(time.Now())
func Parse(spec string) (Schedule, error) {
	return (&Parser{}).Parse(spec)
}

// Parse parses a 5-field cron expression and returns a schedule that can compute
// the next matching activation time using the parser's configuration.
func (p *Parser) Parse(spec string) (Schedule, error) {
	if len(spec) == 0 {
		return nil, fmt.Errorf("empty spec string")
	}

	// Validate YearLimit before parsing
	if p.YearLimit < 0 {
		return nil, fmt.Errorf("year limit cannot be negative: %d", p.YearLimit)
	}

	spec = strings.TrimSpace(spec)
	loc, spec, err := extractLocation(spec)
	if err != nil {
		return nil, err
	}

	fields := strings.Fields(spec)
	if len(fields) != 5 {
		return nil, fmt.Errorf("expected exactly 5 fields, found %d: %v", len(fields), fields)
	}

	field := func(expr string, r bounds) uint64 {
		if err != nil {
			return 0
		}
		var bits uint64
		bits, err = getField(expr, r)
		return bits
	}

	minute := field(fields[0], minutes)
	hour := field(fields[1], hours)
	dayOfMonth := field(fields[2], dom)
	month := field(fields[3], months)
	dayOfWeek := field(fields[4], dow)

	if err != nil {
		return nil, err
	}

	return &SpecSchedule{
		Second:    1 << seconds.min,
		Minute:    minute,
		Hour:      hour,
		Dom:       dayOfMonth,
		Month:     month,
		Dow:       dayOfWeek,
		Location:  loc,
		YearLimit: p.YearLimit,
	}, nil
}

func extractLocation(spec string) (*time.Location, string, error) {
	const prefix = "CRON_TZ="

	if !strings.HasPrefix(spec, prefix) {
		return time.UTC, spec, nil
	}

	rest := spec[len(prefix):]

	// Strip the prefix; IANA names contain only ASCII letters, '.', '-', '_', '/'
	// so the name ends at the single space separator defined by the grammar.
	i := strings.Index(rest, " ")
	if i == -1 {
		return nil, "", fmt.Errorf("expected expression after timezone")
	}

	tzName := rest[:i]
	loc, err := time.LoadLocation(tzName)
	if err != nil {
		return nil, "", fmt.Errorf("provided bad location %s: %v", tzName, err)
	}

	return loc, strings.TrimLeft(rest[i:], " "), nil
}

func getField(field string, r bounds) (uint64, error) {
	var bits uint64
	ranges := strings.FieldsFunc(field, func(char rune) bool { return char == ',' })
	for _, expr := range ranges {
		if len(expr) == 0 {
			return 0, fmt.Errorf("empty list element")
		}
		bit, err := getRange(expr, r)
		if err != nil {
			return bits, err
		}
		bits |= bit
	}
	return bits, nil
}

func getRange(expr string, r bounds) (uint64, error) {
	var (
		start, end, step uint
		rangeAndStep     = strings.SplitN(expr, "/", 2)
		lowAndHigh       = strings.SplitN(rangeAndStep[0], "-", 2)
		singleDigit      = len(lowAndHigh) == 1
		err              error
	)

	var extra uint64
	if lowAndHigh[0] == "*" {
		start = r.min
		end = r.max
		extra = starBit
	} else {
		start, err = parseIntOrName(lowAndHigh[0], r.names)
		if err != nil {
			return 0, err
		}
		switch len(lowAndHigh) {
		case 1:
			end = start
		case 2:
			end, err = parseIntOrName(lowAndHigh[1], r.names)
			if err != nil {
				return 0, err
			}
		default:
			return 0, fmt.Errorf("too many hyphens: %s", expr)
		}
	}

	switch len(rangeAndStep) {
	case 1:
		step = 1
	case 2:
		step, err = mustParseInt(rangeAndStep[1])
		if err != nil {
			return 0, err
		}
		if singleDigit {
			end = r.max
		}
		if step > 1 {
			extra = 0
		}
	default:
		return 0, fmt.Errorf("too many slashes: %s", expr)
	}

	if start < r.min {
		return 0, fmt.Errorf("beginning of range (%d) below minimum (%d): %s", start, r.min, expr)
	}
	if end > r.max {
		return 0, fmt.Errorf("end of range (%d) above maximum (%d): %s", end, r.max, expr)
	}
	if start > end {
		return 0, fmt.Errorf("beginning of range (%d) beyond end of range (%d): %s", start, end, expr)
	}
	if step == 0 {
		return 0, fmt.Errorf("step of range should be a positive number: %s", expr)
	}

	return getBits(start, end, step) | extra, nil
}

func parseIntOrName(expr string, names map[string]uint) (uint, error) {
	if names != nil {
		if namedInt, ok := names[strings.ToLower(expr)]; ok {
			return namedInt, nil
		}
	}
	return mustParseInt(expr)
}

func mustParseInt(expr string) (uint, error) {
	num, err := strconv.Atoi(expr)
	if err != nil {
		return 0, fmt.Errorf("failed to parse int from %s: %s", expr, err)
	}
	if num < 0 {
		return 0, fmt.Errorf("negative number (%d) not allowed: %s", num, expr)
	}
	return uint(num), nil
}

func getBits(min, max, step uint) uint64 {
	if step == 1 {
		return ^(math.MaxUint64 << (max + 1)) & (math.MaxUint64 << min)
	}

	var bits uint64
	for value := min; value <= max; value += step {
		bits |= 1 << value
	}
	return bits
}

func all(r bounds) uint64 {
	return getBits(r.min, r.max, 1) | starBit
}
