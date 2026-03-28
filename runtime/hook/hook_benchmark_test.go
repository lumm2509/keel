package hook

import "testing"

func BenchmarkHookTrigger(b *testing.B) {
	h := Hook[*Event]{}
	for i := 0; i < 8; i++ {
		h.BindFunc(func(e *Event) error {
			return e.Next()
		})
	}

	legacyHandlers := make([]*Handler[*Event], len(h.handlers))
	copy(legacyHandlers, h.handlers)

	b.Run("current", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			if err := h.TriggerWithOneOff(&Event{}, func(e *Event) error { return e.Next() }); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("legacy", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			event := &Event{}
			if err := legacyHookTrigger(legacyHandlers, event, func(e *Event) error { return e.Next() }); err != nil {
				b.Fatal(err)
			}
		}
	})
}

func legacyHookTrigger(handlers []*Handler[*Event], event *Event, oneOffHandlerFuncs ...func(*Event) error) error {
	fns := make([]func(*Event) error, 0, len(handlers)+len(oneOffHandlerFuncs))
	for _, handler := range handlers {
		fns = append(fns, handler.Func)
	}
	fns = append(fns, oneOffHandlerFuncs...)

	event.setNextFunc(nil)

	for i := len(fns) - 1; i >= 0; i-- {
		i := i
		old := event.nextFunc()
		event.setNextFunc(func() error {
			event.setNextFunc(old)
			return fns[i](event)
		})
	}

	return event.Next()
}
