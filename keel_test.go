package keel

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/lumm2509/keel/config"
	"github.com/lumm2509/keel/container"
	transporthttp "github.com/lumm2509/keel/transport/http"
)

func TestNewWithCradleBuildsDefaultContainer(t *testing.T) {
	t.Parallel()

	type cradle struct {
		Name string
	}

	app := New(WithCradle(cradle{Name: "demo"}))

	if app.Container() == nil {
		t.Fatalf("expected default container to be created")
	}

	if got := app.Container().Cradle().Name; got != "demo" {
		t.Fatalf("expected cradle name %q, got %q", "demo", got)
	}

	if got := app.Cradle().Name; got != "demo" {
		t.Fatalf("expected app cradle name %q, got %q", "demo", got)
	}
}

func TestAppExposesRuntimeSurfaceWithoutRequiringContainerInHandlers(t *testing.T) {
	t.Parallel()

	type cradle struct{}

	cfg := &config.ConfigModule{}
	app := New(WithConfig[cradle](cfg))

	if app.Config() != cfg {
		t.Fatalf("expected App.Config() to return the configured module")
	}

	if app.Logger() == nil {
		t.Fatalf("expected App.Logger() to be available")
	}

	if app.Store() == nil {
		t.Fatalf("expected App.Store() to be available")
	}

	if app.Cron() == nil {
		t.Fatalf("expected App.Cron() to be available")
	}

	if app.SubscriptionsBroker() == nil {
		t.Fatalf("expected App.SubscriptionsBroker() to be available")
	}
}

func TestBindRegisteredRoutesWrapsRequestEventWithContext(t *testing.T) {
	t.Parallel()

	type cradle struct {
		Name string
	}

	app := New(WithCradle(cradle{Name: "demo"}))
	app.GET("/hello", func(c *Context[cradle]) error {
		if c.Request().Method != http.MethodGet {
			t.Fatalf("expected GET request, got %s", c.Request().Method)
		}

		return c.JSON(http.StatusOK, map[string]string{
			"hello": c.Cradle().Name,
		})
	})

	router, err := app.bindRegisteredRoutes(app.Container())
	if err != nil {
		t.Fatalf("bindRegisteredRoutes() error = %v", err)
	}

	mux, err := router.BuildMux()
	if err != nil {
		t.Fatalf("BuildMux() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/hello", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if body["hello"] != "demo" {
		t.Fatalf("expected hello payload %q, got %q", "demo", body["hello"])
	}
}

func TestComposeBindRoutesKeepsAdvancedAndSimpleRoutes(t *testing.T) {
	t.Parallel()

	type cradle struct{}

	app := New(Config[cradle]{
		BindRoutes: func(ctr container.Container[cradle]) (*transporthttp.Router[*container.RequestEvent[cradle]], error) {
			router := transporthttp.NewRouter(func(w http.ResponseWriter, r *http.Request) (*container.RequestEvent[cradle], transporthttp.EventCleanupFunc) {
				return &container.RequestEvent[cradle]{
					Container: ctr,
					Event: transporthttp.Event{
						Response: w,
						Request:  r,
					},
				}, nil
			})

			router.GET("/advanced", func(e *container.RequestEvent[cradle]) error {
				return e.JSON(http.StatusOK, map[string]string{"route": "advanced"})
			})

			return router, nil
		},
	})

	app.GET("/simple", func(c *Context[cradle]) error {
		return c.JSON(http.StatusOK, map[string]string{"route": "simple"})
	})

	router, err := app.composeBindRoutes()(app.Container())
	if err != nil {
		t.Fatalf("composeBindRoutes() error = %v", err)
	}

	mux, err := router.BuildMux()
	if err != nil {
		t.Fatalf("BuildMux() error = %v", err)
	}

	for _, path := range []string{"/advanced", "/simple"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected %s status %d, got %d", path, http.StatusOK, rec.Code)
		}
	}
}
