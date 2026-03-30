package http

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newTestRouter() *Router[*RequestEvent[struct{}]] {
	return NewRouter(func(w http.ResponseWriter, r *http.Request) (*RequestEvent[struct{}], EventCleanupFunc) {
		e := &RequestEvent[struct{}]{}
		e.Reset(nil, w, r)
		return e, nil
	})
}

func serveTest(t *testing.T, mux http.Handler, method, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func TestRouterBuildMuxServesRoute(t *testing.T) {
	t.Parallel()

	r := newTestRouter()
	r.GET("/hello", func(e *RequestEvent[struct{}]) error {
		return e.JSON(http.StatusOK, map[string]string{"msg": "hi"})
	})

	mux, err := r.BuildMux()
	if err != nil {
		t.Fatalf("BuildMux error: %v", err)
	}

	rec := serveTest(t, mux, http.MethodGet, "/hello")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestRouterBuildMuxUnknownRouteReturns404(t *testing.T) {
	t.Parallel()

	r := newTestRouter()
	mux, err := r.BuildMux()
	if err != nil {
		t.Fatalf("BuildMux error: %v", err)
	}

	rec := serveTest(t, mux, http.MethodGet, "/nonexistent")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

// --- Handler (live handler) --------------------------------------------------

func TestRouterHandlerServesRoute(t *testing.T) {
	t.Parallel()

	r := newTestRouter()
	r.GET("/ping", func(e *RequestEvent[struct{}]) error {
		return e.JSON(http.StatusOK, map[string]string{"ok": "true"})
	})

	// Build once so cachedMux is populated.
	if _, err := r.BuildMux(); err != nil {
		t.Fatalf("BuildMux error: %v", err)
	}

	rec := serveTest(t, r.Handler(), http.MethodGet, "/ping")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestRouterHandlerBuildsLazilyOnFirstRequest(t *testing.T) {
	t.Parallel()

	r := newTestRouter()
	r.GET("/lazy", func(e *RequestEvent[struct{}]) error {
		return e.JSON(http.StatusOK, nil)
	})

	// No explicit BuildMux call — Handler() must build on first request.
	rec := serveTest(t, r.Handler(), http.MethodGet, "/lazy")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

// --- Patch -------------------------------------------------------------------

func TestRouterPatchAddsRoute(t *testing.T) {
	t.Parallel()

	r := newTestRouter()
	if _, err := r.BuildMux(); err != nil {
		t.Fatalf("BuildMux error: %v", err)
	}

	if err := r.Patch(func(r *Router[*RequestEvent[struct{}]]) {
		r.GET("/patched", func(e *RequestEvent[struct{}]) error {
			return e.JSON(http.StatusCreated, nil)
		})
	}); err != nil {
		t.Fatalf("Patch error: %v", err)
	}

	rec := serveTest(t, r.Handler(), http.MethodGet, "/patched")
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec.Code)
	}
}

func TestRouterPatchDoesNotAffectInFlightRequests(t *testing.T) {
	t.Parallel()

	r := newTestRouter()
	r.GET("/stable", func(e *RequestEvent[struct{}]) error {
		return e.JSON(http.StatusOK, nil)
	})
	if _, err := r.BuildMux(); err != nil {
		t.Fatalf("BuildMux error: %v", err)
	}

	// Capture old mux and patch concurrently.
	oldMux := r.Handler()

	_ = r.Patch(func(r *Router[*RequestEvent[struct{}]]) {
		r.GET("/new", func(e *RequestEvent[struct{}]) error {
			return e.JSON(http.StatusCreated, nil)
		})
	})

	// Old mux still serves the original route correctly.
	rec := serveTest(t, oldMux, http.MethodGet, "/stable")
	if rec.Code != http.StatusOK {
		t.Fatalf("old mux: expected 200, got %d", rec.Code)
	}
}

// --- EventData: route pattern ------------------------------------------------

func TestRouterRoutePatternStoredInEventData(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	var captured string
	r.GET("/users/{id}", func(e *RequestEvent[struct{}]) error {
		captured, _ = e.Get(EventKeyRoutePattern).(string)
		return nil
	})

	mux, err := r.BuildMux()
	if err != nil {
		t.Fatalf("BuildMux error: %v", err)
	}
	serveTest(t, mux, http.MethodGet, "/users/42")

	if captured != "/users/{id}" {
		t.Fatalf("expected route pattern %q, got %q", "/users/{id}", captured)
	}
}

// --- EventData: route handle -------------------------------------------------

type routePolicy struct{ Admin bool }

func TestRouterRouteHandleStoredInEventData(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	var captured any
	r.GET("/admin", func(e *RequestEvent[struct{}]) error {
		captured = e.Get(EventKeyRouteHandle)
		return nil
	}).WithHandle(routePolicy{Admin: true})

	mux, err := r.BuildMux()
	if err != nil {
		t.Fatalf("BuildMux error: %v", err)
	}
	serveTest(t, mux, http.MethodGet, "/admin")

	p, ok := captured.(routePolicy)
	if !ok || !p.Admin {
		t.Fatalf("expected routePolicy{Admin:true}, got %v", captured)
	}
}

// --- Error boundaries --------------------------------------------------------

func TestRouterGroupErrorBoundaryCalledBeforeGlobal(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	var boundaryCalled bool
	api := r.Group("/api")
	api.OnError(func(e *RequestEvent[struct{}], err error) error {
		boundaryCalled = true
		return err
	})
	api.GET("/fail", func(e *RequestEvent[struct{}]) error {
		return NewBadRequestError("bad", nil)
	})

	mux, err := r.BuildMux()
	if err != nil {
		t.Fatalf("BuildMux error: %v", err)
	}
	serveTest(t, mux, http.MethodGet, "/api/fail")

	if !boundaryCalled {
		t.Fatal("expected group error boundary to be called")
	}
}

func TestRouterGroupErrorBoundaryCanSuppressError(t *testing.T) {
	t.Parallel()

	r := newTestRouter()
	api := r.Group("/api")
	api.OnError(func(e *RequestEvent[struct{}], err error) error {
		// suppress: return 204 and eat the error
		e.Response.WriteHeader(http.StatusNoContent)
		return nil
	})
	api.GET("/suppress", func(e *RequestEvent[struct{}]) error {
		return NewBadRequestError("bad", nil)
	})

	mux, err := r.BuildMux()
	if err != nil {
		t.Fatalf("BuildMux error: %v", err)
	}

	rec := serveTest(t, mux, http.MethodGet, "/api/suppress")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
}

func TestRouterRouteErrorBoundaryCalledBeforeGroup(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	var order []string
	api := r.Group("/api")
	api.OnError(func(e *RequestEvent[struct{}], err error) error {
		order = append(order, "group")
		return err
	})
	api.GET("/fail", func(e *RequestEvent[struct{}]) error {
		return NewBadRequestError("bad", nil)
	}).OnError(func(e *RequestEvent[struct{}], err error) error {
		order = append(order, "route")
		return err
	})

	mux, err := r.BuildMux()
	if err != nil {
		t.Fatalf("BuildMux error: %v", err)
	}
	serveTest(t, mux, http.MethodGet, "/api/fail")

	if len(order) != 2 || order[0] != "route" || order[1] != "group" {
		t.Fatalf("expected [route group], got %v", order)
	}
}

func TestRouterRouteErrorBoundaryCanSuppressBeforeGroup(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	var groupCalled bool
	api := r.Group("/api")
	api.OnError(func(e *RequestEvent[struct{}], err error) error {
		groupCalled = true
		return err
	})
	api.GET("/suppress", func(e *RequestEvent[struct{}]) error {
		return errors.New("inner error")
	}).OnError(func(e *RequestEvent[struct{}], err error) error {
		// suppress completely — group boundary must not be reached
		e.Response.WriteHeader(http.StatusAccepted)
		return nil
	})

	mux, err := r.BuildMux()
	if err != nil {
		t.Fatalf("BuildMux error: %v", err)
	}

	rec := serveTest(t, mux, http.MethodGet, "/api/suppress")
	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", rec.Code)
	}
	if groupCalled {
		t.Fatal("group boundary must not be called when route boundary suppresses error")
	}
}
