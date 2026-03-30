package main

import (
	"errors"
	"log"
	"log/slog"
	"net/http"
	"time"

	"github.com/lumm2509/keel"
	"github.com/lumm2509/keel/config"
	"github.com/lumm2509/keel/tools/observability"
)

type MyApp struct {
	Name    string
	Version string
	IsDev   bool
}

type RoutePolicy struct {
	RequireAuth bool
	RateLimit   int // requests per minute, 0 = unlimited
}

var (
	startedAtKey = keel.NewContextKey[time.Time]()
	sessionKey   = keel.NewContextKey[string]()
)

func main() {
	cfg := &config.ConfigModule{}

	myApp := &MyApp{
		Name:    "example",
		Version: "0.2.0",
		IsDev:   true,
	}

	app := keel.New(
		keel.WithConfig[MyApp](cfg),
		keel.WithContext(myApp),
	)

	app.BindFunc(observability.Middleware[MyApp](nil /* uses slog.Default() */))
	app.BindFunc(func(c *keel.Context[MyApp]) error {
		startedAtKey.Set(c, time.Now().UTC())
		return c.Next()
	})

	app.BindFunc(func(c *keel.Context[MyApp]) error {
		policy, ok := keel.RouteHandleAs[RoutePolicy](c)
		if !ok || !policy.RequireAuth {
			return c.Next()
		}

		session, hasCookie := c.Cookies().Get("session")
		if !hasCookie || session == "" {
			return keel.NewUnauthorizedError("missing session", nil)
		}
		sessionKey.Set(c, session)

		return c.Next()
	})

	app.GET("/", func(c *keel.Context[MyApp]) error {
		return c.JSON(http.StatusOK, map[string]any{
			"service": c.App.Name,
			"version": c.App.Version,
			"isDev":   c.App.IsDev,
		})
	})

	app.GET("/health", func(c *keel.Context[MyApp]) error {
		startedAt, _ := startedAtKey.Get(c)
		requestID, _ := observability.RequestIDKey.Get(c)
		return c.JSON(http.StatusOK, map[string]any{
			"ok":        true,
			"requestId": requestID,
			"startedAt": startedAt,
		})
	})

	app.POST("/login", func(c *keel.Context[MyApp]) error {
		token := "demo-token-" + c.App.Name
		c.Cookies().Set("session", token,
			keel.WithHTTPOnly(),
			keel.WithSecure(),
			keel.WithSameSite(http.SameSiteLaxMode),
			keel.WithMaxAge(86400),
		)
		return c.JSON(http.StatusOK, map[string]string{"ok": "true"})
	})

	app.POST("/logout", func(c *keel.Context[MyApp]) error {
		c.Cookies().Delete("session", keel.WithPath("/"))
		return c.JSON(http.StatusOK, map[string]string{"ok": "true"})
	})

	api := app.Group("/api")

	api.OnError(func(c *keel.Context[MyApp], err error) error {
		var apiErr *keel.ApiError
		if errors.As(err, &apiErr) {
			return c.JSON(apiErr.Status, map[string]any{
				"api_error": apiErr.Message,
			})
		}
		return err // unknown errors propagate to the global handler
	})

	api.GET("/status", func(c *keel.Context[MyApp]) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	api.GET("/me", func(c *keel.Context[MyApp]) error {
		session, _ := sessionKey.Get(c)
		return c.JSON(http.StatusOK, map[string]any{
			"name":    c.App.Name,
			"session": session,
		})
	}).WithHandle(RoutePolicy{RequireAuth: true, RateLimit: 60})

	app.GET("/reports", keel.LazyHandler(func() keel.HandlerFunc[MyApp] {
		slog.Info("reports handler: one-time initialization")
		buildTime := time.Now().UTC()

		return func(c *keel.Context[MyApp]) error {
			return c.JSON(http.StatusOK, map[string]any{
				"handler_built_at": buildTime,
				"service":          c.App.Name,
			})
		}
	}))

	app.OnBootstrap().BindFunc(func(e *keel.BootstrapEvent[MyApp]) error {
		slog.Info("application bootstrapped", "name", e.App.Name)

		return app.Router.Patch(func(r *keel.Router[MyApp]) {
			r.GET("/plugin/ping", func(c *keel.Context[MyApp]) error {
				return c.JSON(http.StatusOK, map[string]string{"plugin": "pong"})
			})
		})
	})

	app.OnServe().BindFunc(func(e *keel.ServeEvent[MyApp]) error {
		slog.Info("HTTP server starting", "addr", e.Server.Addr)
		return e.Next()
	})

	app.OnTerminate().BindFunc(func(e *keel.TerminateEvent[MyApp]) error {
		slog.Info("application shutting down", "restart", e.IsRestart)
		return e.Next()
	})

	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
