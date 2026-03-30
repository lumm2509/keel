package hook

import (
	"errors"
	"slices"
	"testing"
)

func TestRouteBindFuncAppendsMiddlewares(t *testing.T) {
	t.Parallel()

	var calls string
	r := &Route[*Event]{}
	r.BindFunc(func(e *Event) error { calls += "a"; return nil })
	r.BindFunc(func(e *Event) error { calls += "b"; return nil })

	if len(r.Middlewares) != 2 {
		t.Fatalf("expected 2, got %d", len(r.Middlewares))
	}
	for _, m := range r.Middlewares {
		_ = m.Func(nil)
	}
	if calls != "ab" {
		t.Fatalf("expected %q, got %q", "ab", calls)
	}
}

func TestRouteBindAppendsHandlers(t *testing.T) {
	t.Parallel()

	r := &Route[*Event]{
		ExcludedMiddlewares: map[string]struct{}{"existing": {}},
	}
	r.Bind(
		&Handler[*Event]{Id: "existing", Func: func(e *Event) error { return nil }},
		&Handler[*Event]{Id: "new", Func: func(e *Event) error { return nil }},
	)

	if len(r.Middlewares) != 2 {
		t.Fatalf("expected 2, got %d", len(r.Middlewares))
	}
	if _, stillExcluded := r.ExcludedMiddlewares["existing"]; stillExcluded {
		t.Fatal("Bind should remove re-added id from ExcludedMiddlewares")
	}
}

func TestRouteBindPriorityOrder(t *testing.T) {
	t.Parallel()

	r := &Route[*Event]{}
	r.Bind(
		&Handler[*Event]{Id: "late", Priority: 10},
		&Handler[*Event]{Id: "early", Priority: -10},
		&Handler[*Event]{Id: "mid", Priority: 0},
	)

	ids := []string{r.Middlewares[0].Id, r.Middlewares[1].Id, r.Middlewares[2].Id}
	if !slices.Equal(ids, []string{"early", "mid", "late"}) {
		t.Fatalf("unexpected order: %v", ids)
	}
}

func TestRouteUnbindRemovesAndExcludes(t *testing.T) {
	t.Parallel()

	r := &Route[*Event]{}
	r.Bind(
		&Handler[*Event]{Id: "a", Func: func(e *Event) error { return nil }},
		&Handler[*Event]{Id: "b", Func: func(e *Event) error { return nil }},
	)
	r.Unbind("") // no-op
	r.Unbind("a")

	if len(r.Middlewares) != 1 || r.Middlewares[0].Id != "b" {
		t.Fatalf("expected only 'b' to remain, got %v", r.Middlewares)
	}
	if _, ok := r.ExcludedMiddlewares["a"]; !ok {
		t.Fatal("expected 'a' in ExcludedMiddlewares")
	}
}

func TestRouteWithHandleStoresValue(t *testing.T) {
	t.Parallel()

	type meta struct{ Role string }

	r := &Route[*Event]{}
	r.WithHandle(meta{Role: "admin"})

	got, ok := r.Handle.(meta)
	if !ok {
		t.Fatalf("expected meta, got %T", r.Handle)
	}
	if got.Role != "admin" {
		t.Fatalf("expected Role %q, got %q", "admin", got.Role)
	}
}

func TestRouteOnErrorStoresBoundary(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("sentinel")
	r := &Route[*Event]{}
	r.OnError(func(e *Event, err error) error { return sentinel })

	if r.ErrorHandler == nil {
		t.Fatal("expected ErrorHandler to be set")
	}
	if got := r.ErrorHandler(nil, errors.New("x")); !errors.Is(got, sentinel) {
		t.Fatalf("expected sentinel, got %v", got)
	}
}
