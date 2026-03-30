package http

import (
	"errors"
	"slices"
	"testing"

	"github.com/lumm2509/keel/runtime/hook"
)

func TestRouteBindFunc(t *testing.T) {
	t.Parallel()

	r := Route[*Event]{}

	calls := ""

	// append one function
	r.BindFunc(func(e *Event) error {
		calls += "a"
		return nil
	})

	// append multiple functions
	r.BindFunc(
		func(e *Event) error {
			calls += "b"
			return nil
		},
		func(e *Event) error {
			calls += "c"
			return nil
		},
	)

	if total := len(r.Middlewares); total != 3 {
		t.Fatalf("Expected %d middlewares, got %v", 3, total)
	}

	for _, h := range r.Middlewares {
		_ = h.Func(nil)
	}

	if calls != "abc" {
		t.Fatalf("Expected calls sequence %q, got %q", "abc", calls)
	}
}

func TestRouteBind(t *testing.T) {
	t.Parallel()

	r := Route[*Event]{
		// mock excluded middlewares to check whether the entry will be deleted
		excludedMiddlewares: map[string]struct{}{"test2": {}},
	}

	calls := ""

	// append one handler
	r.Bind(&hook.Handler[*Event]{
		Func: func(e *Event) error {
			calls += "a"
			return nil
		},
	})

	// append multiple handlers
	r.Bind(
		&hook.Handler[*Event]{
			Id: "test1",
			Func: func(e *Event) error {
				calls += "b"
				return nil
			},
		},
		&hook.Handler[*Event]{
			Id: "test2",
			Func: func(e *Event) error {
				calls += "c"
				return nil
			},
		},
	)

	if total := len(r.Middlewares); total != 3 {
		t.Fatalf("Expected %d middlewares, got %v", 3, total)
	}

	for _, h := range r.Middlewares {
		_ = h.Func(nil)
	}

	if calls != "abc" {
		t.Fatalf("Expected calls %q, got %q", "abc", calls)
	}

	// ensures that the previously excluded middleware was removed
	if len(r.excludedMiddlewares) != 0 {
		t.Fatalf("Expected test2 to be removed from the excludedMiddlewares list, got %v", r.excludedMiddlewares)
	}
}

func TestRouteBindPriorityOrder(t *testing.T) {
	t.Parallel()

	r := Route[*Event]{}

	r.Bind(
		&hook.Handler[*Event]{Id: "late", Priority: 10},
		&hook.Handler[*Event]{Id: "early", Priority: -10},
		&hook.Handler[*Event]{Id: "mid1", Priority: 0},
		&hook.Handler[*Event]{Id: "mid2", Priority: 0},
	)

	got := []string{
		r.Middlewares[0].Id,
		r.Middlewares[1].Id,
		r.Middlewares[2].Id,
		r.Middlewares[3].Id,
	}

	expected := []string{"early", "mid1", "mid2", "late"}
	if !slices.Equal(got, expected) {
		t.Fatalf("expected %v, got %v", expected, got)
	}
}

func TestRouteUnbind(t *testing.T) {
	t.Parallel()

	r := Route[*Event]{}

	calls := ""

	// anonymous middlewares
	r.Bind(&hook.Handler[*Event]{
		Func: func(e *Event) error {
			calls += "a"
			return nil // unused value
		},
	})

	// middlewares with id
	r.Bind(&hook.Handler[*Event]{
		Id: "test1",
		Func: func(e *Event) error {
			calls += "b"
			return nil // unused value
		},
	})
	r.Bind(&hook.Handler[*Event]{
		Id: "test2",
		Func: func(e *Event) error {
			calls += "c"
			return nil // unused value
		},
	})
	r.Bind(&hook.Handler[*Event]{
		Id: "test3",
		Func: func(e *Event) error {
			calls += "d"
			return nil // unused value
		},
	})

	// remove
	r.Unbind("") // should be no-op
	r.Unbind("test1", "test3")

	if total := len(r.Middlewares); total != 2 {
		t.Fatalf("Expected %d middlewares, got %v", 2, total)
	}

	for _, h := range r.Middlewares {
		if err := h.Func(nil); err != nil {
			continue
		}
	}

	if calls != "ac" {
		t.Fatalf("Expected calls %q, got %q", "ac", calls)
	}

	// ensure that the id was added in the exclude list
	excluded := []string{"test1", "test3"}
	if len(r.excludedMiddlewares) != len(excluded) {
		t.Fatalf("Expected excludes %v, got %v", excluded, r.excludedMiddlewares)
	}
	for id := range r.excludedMiddlewares {
		if !slices.Contains(excluded, id) {
			t.Fatalf("Expected %q to be marked as excluded", id)
		}
	}
}

func TestRouteWithHandle(t *testing.T) {
	t.Parallel()

	type meta struct{ Role string }

	r := Route[*Event]{}
	r.WithHandle(meta{Role: "admin"})

	got, ok := r.Handle.(meta)
	if !ok {
		t.Fatalf("expected Handle to be meta, got %T", r.Handle)
	}
	if got.Role != "admin" {
		t.Fatalf("expected Role %q, got %q", "admin", got.Role)
	}
}

func TestRouteOnError(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("sentinel")

	r := Route[*Event]{}
	r.OnError(func(e *Event, err error) error {
		return sentinel
	})

	if r.ErrorHandler == nil {
		t.Fatal("expected ErrorHandler to be set")
	}
	if got := r.ErrorHandler(nil, errors.New("any")); !errors.Is(got, sentinel) {
		t.Fatalf("expected sentinel, got %v", got)
	}
}
