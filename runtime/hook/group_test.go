package hook

import (
	"errors"
	"net/http"
	"slices"
	"testing"
)

func TestRouterGroupCreatesChildWithPrefix(t *testing.T) {
	t.Parallel()

	g := &RouterGroup[*Event]{}
	sub := g.Group("/api")

	if sub.Prefix != "/api" {
		t.Fatalf("expected prefix %q, got %q", "/api", sub.Prefix)
	}
	if len(g.Children) != 1 {
		t.Fatalf("expected 1 child, got %d", len(g.Children))
	}
	if g.Children[0] != sub {
		t.Fatal("child pointer mismatch")
	}
}

// --- BindFunc / Bind / Unbind ------------------------------------------------

func TestRouterGroupBindFuncAppendsMiddlewares(t *testing.T) {
	t.Parallel()

	var calls string
	g := &RouterGroup[*Event]{}
	g.BindFunc(func(e *Event) error { calls += "a"; return nil })
	g.BindFunc(func(e *Event) error { calls += "b"; return nil })

	if len(g.Middlewares) != 2 {
		t.Fatalf("expected 2 middlewares, got %d", len(g.Middlewares))
	}
	for _, m := range g.Middlewares {
		_ = m.Func(nil)
	}
	if calls != "ab" {
		t.Fatalf("expected %q, got %q", "ab", calls)
	}
}

func TestRouterGroupBindAppendsHandlers(t *testing.T) {
	t.Parallel()

	g := &RouterGroup[*Event]{
		ExcludedMiddlewares: map[string]struct{}{"existing": {}},
	}
	g.Bind(
		&Handler[*Event]{Id: "existing", Func: func(e *Event) error { return nil }},
		&Handler[*Event]{Id: "new", Func: func(e *Event) error { return nil }},
	)

	if len(g.Middlewares) != 2 {
		t.Fatalf("expected 2, got %d", len(g.Middlewares))
	}
	if _, stillExcluded := g.ExcludedMiddlewares["existing"]; stillExcluded {
		t.Fatal("Bind should remove re-added id from ExcludedMiddlewares")
	}
}

func TestRouterGroupBindPriorityOrder(t *testing.T) {
	t.Parallel()

	g := &RouterGroup[*Event]{}
	g.Bind(
		&Handler[*Event]{Id: "late", Priority: 10},
		&Handler[*Event]{Id: "early", Priority: -10},
		&Handler[*Event]{Id: "mid", Priority: 0},
	)

	ids := []string{g.Middlewares[0].Id, g.Middlewares[1].Id, g.Middlewares[2].Id}
	if !slices.Equal(ids, []string{"early", "mid", "late"}) {
		t.Fatalf("unexpected order: %v", ids)
	}
}

func TestRouterGroupUnbindRemovesAndExcludes(t *testing.T) {
	t.Parallel()

	g := &RouterGroup[*Event]{}
	g.Bind(
		&Handler[*Event]{Id: "a", Func: func(e *Event) error { return nil }},
		&Handler[*Event]{Id: "b", Func: func(e *Event) error { return nil }},
		&Handler[*Event]{Id: "c", Func: func(e *Event) error { return nil }},
	)
	g.Unbind("") // no-op
	g.Unbind("a", "c")

	if len(g.Middlewares) != 1 || g.Middlewares[0].Id != "b" {
		t.Fatalf("expected only 'b' to remain, got %v", g.Middlewares)
	}
	if _, ok := g.ExcludedMiddlewares["a"]; !ok {
		t.Fatal("expected 'a' in ExcludedMiddlewares")
	}
	if _, ok := g.ExcludedMiddlewares["c"]; !ok {
		t.Fatal("expected 'c' in ExcludedMiddlewares")
	}
}

func TestRouterGroupUnbindPropagatesIntoChildren(t *testing.T) {
	t.Parallel()

	g := &RouterGroup[*Event]{}
	child := g.Group("/sub")
	child.Bind(&Handler[*Event]{Id: "shared", Func: func(e *Event) error { return nil }})

	route := g.Route(http.MethodGet, "/r", func(e *Event) error { return nil })
	route.Bind(&Handler[*Event]{Id: "shared", Func: func(e *Event) error { return nil }})

	g.Unbind("shared")

	if len(child.Middlewares) != 0 {
		t.Fatal("expected child middleware to be removed")
	}
	if len(route.Middlewares) != 0 {
		t.Fatal("expected route middleware to be removed")
	}
}

// --- OnError -----------------------------------------------------------------

func TestRouterGroupOnError(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("sentinel")
	g := &RouterGroup[*Event]{}
	g.OnError(func(e *Event, err error) error { return sentinel })

	if g.ErrorHandler == nil {
		t.Fatal("expected ErrorHandler to be set")
	}
	if got := g.ErrorHandler(nil, errors.New("x")); !errors.Is(got, sentinel) {
		t.Fatalf("expected sentinel, got %v", got)
	}
}

// --- Route registration ------------------------------------------------------

func TestRouterGroupRouteRegistersCorrectly(t *testing.T) {
	t.Parallel()

	g := &RouterGroup[*Event]{}
	r := g.Route(http.MethodPost, "/items", func(e *Event) error { return nil })

	if r.Method != http.MethodPost {
		t.Fatalf("expected POST, got %q", r.Method)
	}
	if r.Path != "/items" {
		t.Fatalf("expected /items, got %q", r.Path)
	}
	if len(g.Children) != 1 || g.Children[0] != r {
		t.Fatal("route not appended to children")
	}
}

func TestRouterGroupHTTPMethodAliases(t *testing.T) {
	t.Parallel()

	g := &RouterGroup[*Event]{}
	action := func(e *Event) error { return nil }

	cases := []struct {
		route  *Route[*Event]
		method string
	}{
		{g.Any("/", action), ""},
		{g.GET("/", action), http.MethodGet},
		{g.POST("/", action), http.MethodPost},
		{g.DELETE("/", action), http.MethodDelete},
		{g.PATCH("/", action), http.MethodPatch},
		{g.PUT("/", action), http.MethodPut},
		{g.HEAD("/", action), http.MethodHead},
		{g.OPTIONS("/", action), http.MethodOptions},
		{g.SEARCH("/", action), "SEARCH"},
	}

	for _, c := range cases {
		if c.route.Method != c.method {
			t.Errorf("expected method %q, got %q", c.method, c.route.Method)
		}
	}
}

// --- HasRoute ----------------------------------------------------------------

func TestRouterGroupHasRoute(t *testing.T) {
	t.Parallel()

	g := &RouterGroup[*Event]{}
	g.GET("/base", nil)
	sub := g.Group("/sub")
	sub.POST("/item", nil)
	sub.GET("/item/{id}", nil)
	g.GET("/wild/", nil)        // same as /wild/{...}
	g.GET("/ex/{test...}", nil) // same as /ex/

	cases := []struct {
		method string
		path   string
		want   bool
	}{
		{http.MethodGet, "/base", true},
		{http.MethodPost, "/base", false},
		{http.MethodPost, "/sub/item", true},
		{http.MethodGet, "/sub/item", false},
		{http.MethodGet, "/sub/item/{id}", true},
		{http.MethodGet, "/wild/{test...}", true},
		{http.MethodGet, "/ex/", true},
		{http.MethodGet, "/missing", false},
	}

	for _, c := range cases {
		got := g.HasRoute(c.method, c.path)
		if got != c.want {
			t.Errorf("HasRoute(%q, %q) = %v, want %v", c.method, c.path, got, c.want)
		}
	}
}
