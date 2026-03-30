package hook

import (
	"errors"
	"testing"
)

func TestHookAddHandlerAndAdd(t *testing.T) {
	calls := ""

	h := Hook[*Event]{}

	h.BindFunc(func(e *Event) error { calls += "1"; return e.Next() })
	h.BindFunc(func(e *Event) error { calls += "2"; return e.Next() })
	h3Id := h.BindFunc(func(e *Event) error { calls += "3"; return e.Next() })
	h.Bind(&Handler[*Event]{
		Id:   h3Id, // should replace 3
		Func: func(e *Event) error { calls += "3'"; return e.Next() },
	})
	h.Bind(&Handler[*Event]{
		Func:     func(e *Event) error { calls += "4"; return e.Next() },
		Priority: -2,
	})
	h.Bind(&Handler[*Event]{
		Func:     func(e *Event) error { calls += "5"; return e.Next() },
		Priority: -1,
	})
	h.Bind(&Handler[*Event]{
		Func: func(e *Event) error { calls += "6"; return e.Next() },
	})
	h.Bind(&Handler[*Event]{
		Func: func(e *Event) error { calls += "7"; e.Next(); return errors.New("test") }, // error shouldn't stop the chain
	})

	h.Trigger(
		&Event{},
		func(e *Event) error { calls += "8"; return e.Next() },
		func(e *Event) error { calls += "9"; return nil }, // skip next
		func(e *Event) error { calls += "10"; return e.Next() },
	)

	if total := len(h.handlers); total != 7 {
		t.Fatalf("Expected %d handlers, found %d", 7, total)
	}

	expectedCalls := "45123'6789"

	if calls != expectedCalls {
		t.Fatalf("Expected calls sequence %q, got %q", expectedCalls, calls)
	}
}

func TestHookLength(t *testing.T) {
	h := Hook[*Event]{}

	if l := h.Length(); l != 0 {
		t.Fatalf("Expected 0 hook handlers, got %d", l)
	}

	h.BindFunc(func(e *Event) error { return e.Next() })
	h.BindFunc(func(e *Event) error { return e.Next() })

	if l := h.Length(); l != 2 {
		t.Fatalf("Expected 2 hook handlers, got %d", l)
	}
}

func TestHookUnbind(t *testing.T) {
	h := Hook[*Event]{}

	calls := ""

	id0 := h.BindFunc(func(e *Event) error { calls += "0"; return e.Next() })
	id1 := h.BindFunc(func(e *Event) error { calls += "1"; return e.Next() })
	h.BindFunc(func(e *Event) error { calls += "2"; return e.Next() })
	h.Bind(&Handler[*Event]{
		Func: func(e *Event) error { calls += "3"; return e.Next() },
	})

	h.Unbind("missing") // should do nothing and not panic

	if total := len(h.handlers); total != 4 {
		t.Fatalf("Expected %d handlers, got %d", 4, total)
	}

	h.Unbind(id1, id0)

	if total := len(h.handlers); total != 2 {
		t.Fatalf("Expected %d handlers, got %d", 2, total)
	}

	err := h.Trigger(&Event{}, func(e *Event) error { calls += "4"; return e.Next() })
	if err != nil {
		t.Fatal(err)
	}

	expectedCalls := "234"

	if calls != expectedCalls {
		t.Fatalf("Expected calls sequence %q, got %q", expectedCalls, calls)
	}
}

func TestHookUnbindAll(t *testing.T) {
	h := Hook[*Event]{}

	h.UnbindAll() // should do nothing and not panic

	h.BindFunc(func(e *Event) error { return nil })
	h.BindFunc(func(e *Event) error { return nil })

	if total := len(h.handlers); total != 2 {
		t.Fatalf("Expected %d handlers before UnbindAll, found %d", 2, total)
	}

	h.UnbindAll()

	if total := len(h.handlers); total != 0 {
		t.Fatalf("Expected no handlers after UnbindAll, found %d", total)
	}
}

func TestHookTriggerErrorPropagation(t *testing.T) {
	err := errors.New("test")

	scenarios := []struct {
		name          string
		handlers      []func(*Event) error
		expectedError error
	}{
		{
			"without error",
			[]func(*Event) error{
				func(e *Event) error { return e.Next() },
				func(e *Event) error { return e.Next() },
			},
			nil,
		},
		{
			"with error",
			[]func(*Event) error{
				func(e *Event) error { return e.Next() },
				func(e *Event) error { e.Next(); return err },
				func(e *Event) error { return e.Next() },
			},
			err,
		},
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			h := Hook[*Event]{}
			for _, handler := range s.handlers {
				h.BindFunc(handler)
			}
			result := h.Trigger(&Event{})
			if result != s.expectedError {
				t.Fatalf("Expected %v, got %v", s.expectedError, result)
			}
		})
	}
}

func TestHookSetHandlersReplaces(t *testing.T) {
	h := Hook[*Event]{}
	h.BindFunc(func(e *Event) error { return e.Next() })

	h.SetHandlers([]*Handler[*Event]{
		{Id: "a", Priority: 2, Func: func(e *Event) error { return e.Next() }},
		{Id: "b", Priority: 1, Func: func(e *Event) error { return e.Next() }},
	})

	// SetHandlers sorts by priority.
	if h.handlers[0].Id != "b" || h.handlers[1].Id != "a" {
		t.Fatalf("expected priority order [b, a], got [%s, %s]", h.handlers[0].Id, h.handlers[1].Id)
	}
}

func TestHookSetSortedHandlersReplaces(t *testing.T) {
	h := Hook[*Event]{}
	h.BindFunc(func(e *Event) error { return e.Next() })

	sorted := []*Handler[*Event]{
		{Id: "x", Func: func(e *Event) error { return e.Next() }},
		{Id: "y", Func: func(e *Event) error { return e.Next() }},
	}
	h.SetSortedHandlers(sorted)

	if h.Length() != 2 {
		t.Fatalf("expected 2 handlers, got %d", h.Length())
	}
}

func TestHookTriggerWithOneOff(t *testing.T) {
	calls := ""

	h := Hook[*Event]{}
	h.BindFunc(func(e *Event) error { calls += "m"; return e.Next() })

	e := &Event{}
	if err := h.TriggerWithOneOff(e, func(e *Event) error { calls += "a"; return e.Next() }); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if calls != "ma" {
		t.Fatalf("expected %q, got %q", "ma", calls)
	}
}

func TestEventDataGetSet(t *testing.T) {
	t.Parallel()

	e := &EventData{}
	e.Set("k", "v")

	got := e.Get("k")
	if got != "v" {
		t.Fatalf("expected %q, got %v", "v", got)
	}
}

func TestEventDataGetAll(t *testing.T) {
	t.Parallel()

	e := &EventData{}
	e.Set("a", 1)
	e.Set("b", 2)

	all := e.GetAll()
	if len(all) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(all))
	}
}

func TestEventDataSetAll(t *testing.T) {
	t.Parallel()

	e := &EventData{}
	e.SetAll(map[string]any{"x": 10, "y": 20})

	if e.Get("x") != 10 || e.Get("y") != 20 {
		t.Fatalf("SetAll did not store values correctly")
	}
}
