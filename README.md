# Cron

> _Don't waste another hour configuring crontabs._ Spin up, manage, and debug scheduled tasks in seconds with [Crystade](https://crystade.com) - the next-level SaaS for modern cron management.

![banner](./assets/banner.png)

Cron is a fork of [robfig/cron](https://github.com/robfig/cron). Compared to the original project:
- The scheduler is removed
- The syntax is modified:
  - Only `CRON_TZ=` is permitted to represent timezone (the original permits `CRON_TZ=` and `TZ=`)
  - Only `*` is permitted to represent "every-step" (the original permits both `*` and `?`)
  - Descriptors are unsupported
  - Seconds are unsupported
- The default timezone is UTC (the original sets default to local)
- Customizable year limit for the scheduling algorithm

## Installation

```bash
go get github.com/crystade/cron
```

## Usage

### Basic Usage

```go
package main

import (
    "fmt"
    "log"
    "time"
    
    "github.com/crystade/cron"
)

func main() {
    // Parse a cron expression
    schedule, err := cron.Parse("0 9 * * mon-fri")
    if err != nil {
        log.Fatal(err)
    }
    
    // Get the next execution time
    next := schedule.Next(time.Now())
    fmt.Println("Next run:", next)
}
```

### Custom Parser Configuration

```go
// Create a parser with custom year limit
parser := cron.NewParser(10) // Search up to 10 years ahead
schedule, err := parser.Parse("0 0 29 2 *") // Feb 29 (leap years only)
if err != nil {
    log.Fatal(err)
}

next := schedule.Next(time.Now())
fmt.Println("Next leap year Feb 29:", next)
```

### WASM/TinyGo Support

This library is fully compatible with TinyGo and WASM. Build for WASM:

```bash
tinygo build -target wasm -o cron.wasm ./cmd/wasm
```

Example WASM usage in JavaScript:

```javascript
// Load the WASM module
const go = new Go();
WebAssembly.instantiateStreaming(fetch("cron.wasm"), go.importObject)
    .then((result) => {
        go.run(result.instance);

        // Reusable schedule: parse once, call next() many times
        const spec = cronParse("0 9 * * mon-fri");
        if (spec.error) {
            console.error(spec.error);
        } else {
            const now = Temporal.Now.instant();
            const r1 = spec.next(now.epochMilliseconds);
            console.log("Next run:", Temporal.Instant.fromEpochMilliseconds(r1));
            const r2 = spec.next(r1);
            console.log("Run after:", Temporal.Instant.fromEpochMilliseconds(r2));
            spec.free(); // release Go handles when done
        }

        // One-shot helper
        const r = cronNext("0 9 * * mon-fri", Temporal.Now.instant().epochMilliseconds);
        if (r.error) {
            console.error(r.error);
        } else {
            console.log("Next run:", Temporal.Instant.fromEpochMilliseconds(r));
        }
    });
```

Both functions accept and return Unix timestamps in **milliseconds** (`number` / integer). Use `Temporal.Now.instant().epochMilliseconds` to get the current time and `Temporal.Instant.fromEpochMilliseconds(result.unixMillis)` to convert the result back to a JS `Temporal.Instant`.

## Syntax
```
[CRON_TZ=<timezone>] <min> <hour> <dom> <month> <dow>
```

- Explanation to each field (from left to right):

| Field        | Range           |
|--------------|-----------------|
| Minute       | 0–59            |
| Hour         | 0–23            |
| Day of month | 1–31            |
| Month        | 1–12 or jan–dec |
| Day of week  | 0–6 or sun–sat  |

- Special presentation (any, every, range and list)

```
* = any   */n = every n   a-b = range   a,b = list (OR)
```

- When both day-of-month and day-of-week coexist, they act as OR condition

### Example

| Cron Expression                      | Meaning                  |
|--------------------------------------|--------------------------|
| `0 * * * *`                          | Every hour               |
| `0 9 * * mon-fri`                    | 9 AM on weekdays         |
| `*/15 * * * *`                       | Every 15 minutes         |
| `CRON_TZ=America/New_York 0 8 * * *` | 8 AM New York time daily |

### EBNF

```ebnf
Expression      = [ TimeZone, Space ], UnixExpr
TimeZone        = TimeZonePrefix, "=", IANATimeZone
TimeZonePrefix  = "CRON_TZ"
IANATimeZone    = (* IANA timezone identifier, e.g., "America/New_York", "UTC" *)
Space           = " " { " " } (* one or more U+0020 spaces *)

(* 5-field standard Unix cron: minute hour day-of-month month day-of-week *)
UnixExpr        = MinuteField, Space, HourField, Space, DayOfMonthField, Space, MonthField, Space, DayOfWeekField

(* Comma-separated field lists *)
MinuteField     = NumericFieldPart { ",", NumericFieldPart } (* 0-59 *)
HourField       = NumericFieldPart { ",", NumericFieldPart } (* 0-23 *)
DayOfMonthField = NumericFieldPart   { ",", NumericFieldPart }  (* 1-31 *)
MonthField      = MonthFieldPart   { ",", MonthFieldPart }  (* 1-12 *)
DayOfWeekField  = DOWFieldPart     { ",", DOWFieldPart }  (* 0-6 *)

(* Core field components: *, */N, N, N-N, N/N, N-N/N *)
NumericFieldPart = EveryStep [ "/" number ] | number [ "-" number ] [ "/" number ]
MonthFieldPart   = EveryStep [ "/" number ] | MonthValue [ "-" MonthValue ] [ "/" number ]
DOWFieldPart     = EveryStep [ "/" number ] | DOWValue [ "-" DOWValue ] [ "/" number ]

EveryStep       = "*"
number          = digit { digit }
digit           = "0" | "1" | "2" | "3" | "4" | "5" | "6" | "7" | "8" | "9"

(* Named values for months and days of week (case-insensitive in practice) *)
MonthValue      = number | "jan" | "feb" | "mar" | "apr" | "may" | "jun" | "jul" | "aug" | "sep" | "oct" | "nov" | "dec"
DOWValue        = number | "sun" | "mon" | "tue" | "wed" | "thu" | "fri" | "sat"
```

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
