package main

import (
	"syscall/js"
	"time"

	"github.com/crystade/cron"
)

func main() {
	js.Global().Set("cronParse", js.FuncOf(cronParse))
	// Legacy: parse and compute next in one call
	js.Global().Set("cronNext", js.FuncOf(cronNext))

	select {}
}

// cronParse parses a cron spec once and returns a reusable schedule object.
//
//	const spec = cronParse("* * * * *")
//	spec.next(new Date())
//	spec.next(new Date())
//	spec.free()          // release Go function handles when done
func cronParse(this js.Value, args []js.Value) any {
	if len(args) < 1 {
		return map[string]any{"error": "cronParse requires a spec string"}
	}

	schedule, err := cron.Parse(args[0].String())
	if err != nil {
		return map[string]any{"error": err.Error()}
	}

	var nextFn, freeFn js.Func

	nextFn = js.FuncOf(func(this js.Value, args []js.Value) any {
		if len(args) < 1 {
			return map[string]any{"error": "next requires unix milliseconds as an integer"}
		}
		next := schedule.Next(time.UnixMilli(int64(args[0].Float())))
		if next.IsZero() {
			return nil
		}
		return next.UnixMilli()
	})

	freeFn = js.FuncOf(func(this js.Value, args []js.Value) any {
		nextFn.Release()
		freeFn.Release()
		return nil
	})

	return map[string]any{"next": nextFn, "free": freeFn}
}

// cronNext is a function that parses and computes next in one call.
// Usage: cronNext("* * * * *", Date.now())
func cronNext(this js.Value, args []js.Value) any {
	if len(args) < 2 {
		return map[string]any{"error": "cronNext requires spec and unix milliseconds"}
	}

	schedule, err := cron.Parse(args[0].String())
	if err != nil {
		return map[string]any{"error": err.Error()}
	}

	from := time.UnixMilli(int64(args[1].Float()))
	next := schedule.Next(from)
	if next.IsZero() {
		return nil
	}

	return next.UnixMilli()
}
