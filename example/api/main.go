// Package main demonstrates a REST API pattern with keel.
//
// Shows:
//   - ContextKey for typed per-request state
//   - route metadata (WithHandle) + middleware that reads it
//   - group with OnError boundary
//   - BindAndValidate for body binding + validation in one step
//   - LazyHandler for one-time handler initialisation
//   - Router.Patch to add routes after bootstrap
package main

import (
	"errors"
	"log"
	"log/slog"
	"net/http"
	"time"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/go-ozzo/ozzo-validation/v4/is"
	"github.com/lumm2509/keel"
	"github.com/lumm2509/keel/config"
)

// ── App context ──────────────────────────────────────────────────────────────

type App struct {
	Name string
}

// ── Route metadata ───────────────────────────────────────────────────────────

type RoutePolicy struct {
	RequireAuth bool
}

// ── Per-request context keys ─────────────────────────────────────────────────

var (
	requestStartKey = keel.NewContextKey[time.Time]()
	sessionKey      = keel.NewContextKey[string]()
)

// ── Request body ─────────────────────────────────────────────────────────────

type CreateItemBody struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

func (b *CreateItemBody) Validate() error {
	return validation.ValidateStruct(b,
		validation.Field(&b.Name, validation.Required),
		validation.Field(&b.Email, validation.Required, is.Email),
	)
}

// ── main ─────────────────────────────────────────────────────────────────────

func main() {
	app := keel.New(keel.Config[App]{
		Context: &App{Name: "api-example"},
		Module:  &config.Config{},
	})

	// Timing middleware: record when the request started.
	app.BindFunc(func(c *keel.Context[App]) error {
		requestStartKey.Set(c, time.Now())
		return c.Next()
	})

	// Auth middleware: reads route policy, enforces session cookie.
	app.BindFunc(func(c *keel.Context[App]) error {
		policy, ok := keel.RouteHandleAs[RoutePolicy](c)
		if !ok || !policy.RequireAuth {
			return c.Next()
		}
		session, ok := c.Cookies().Get("session")
		if !ok || session == "" {
			return keel.NewUnauthorizedError("", nil)
		}
		sessionKey.Set(c, session)
		return c.Next()
	})

	// ── Public routes ────────────────────────────────────────────────────────

	app.GET("/health", func(c *keel.Context[App]) error {
		start, _ := requestStartKey.Get(c)
		return c.JSON(http.StatusOK, map[string]any{
			"ok":      true,
			"elapsed": time.Since(start).String(),
		})
	})

	// ── /api group with structured error handling ────────────────────────────

	api := app.Group("/api")

	api.OnError(func(c *keel.Context[App], err error) error {
		var apiErr *keel.ApiError
		if errors.As(err, &apiErr) {
			return c.JSON(apiErr.Status, map[string]any{"error": apiErr.Message, "data": apiErr.Data})
		}
		return err // unknown errors reach the global handler → 500
	})

	api.GET("/me", func(c *keel.Context[App]) error {
		session, _ := sessionKey.Get(c)
		return c.JSON(http.StatusOK, map[string]any{"session": session})
	}).WithHandle(RoutePolicy{RequireAuth: true})

	api.POST("/items", func(c *keel.Context[App]) error {
		var body CreateItemBody
		if err := keel.BindAndValidate(c, &body); err != nil {
			return err
		}
		// ... create item ...
		return c.JSON(http.StatusCreated, map[string]any{"name": body.Name, "email": body.Email})
	})

	// ── LazyHandler: one-time initialisation on first request ────────────────

	app.GET("/reports", keel.LazyHandler(func() keel.HandlerFunc[App] {
		slog.Info("reports: one-time init")
		builtAt := time.Now()
		return func(c *keel.Context[App]) error {
			return c.JSON(http.StatusOK, map[string]any{"built_at": builtAt})
		}
	}))

	// ── Router.Patch: add routes after bootstrap ──────────────────────────────

	app.OnBootstrap().BindFunc(func(e *keel.BootstrapEvent[App]) error {
		return app.Router.Patch(func(r *keel.Router[App]) {
			r.GET("/plugin/ping", func(c *keel.Context[App]) error {
				return c.JSON(http.StatusOK, map[string]string{"pong": "true"})
			})
		})
	})

	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
