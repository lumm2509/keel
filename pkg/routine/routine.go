package routine

import (
	"log/slog"
	"runtime"
	"sync"
)

// FireAndForget executes f() in a new go routine and auto recovers if panic.
//
// **Note:** Use this only if you are not interested in the result of f()
// and don't want to block the parent go routine.
func FireAndForget(f func(), wg ...*sync.WaitGroup) {
	if len(wg) > 0 && wg[0] != nil {
		wg[0].Add(1)
	}

	go func() {
		if len(wg) > 0 && wg[0] != nil {
			defer wg[0].Done()
		}

		defer func() {
			if err := recover(); err != nil {
				stack := make([]byte, 2<<10)
				length := runtime.Stack(stack, false)
				slog.Error("goroutine panic recovered", "error", err, "stack", string(stack[:length]))
			}
		}()

		f()
	}()
}
