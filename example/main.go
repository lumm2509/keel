package main

import (
	"log"
	"log/slog"
	"net/http"
	"time"

	"github.com/lumm2509/keel"
	"github.com/lumm2509/keel/config"
)

type MyApp struct {
	Name    string
	Version string
}

func main() {
	env := "local"

	cfg := &config.ConfigModule{
		Projectconfig: config.ProjectConfigOptions{
			IsDev: true,
		},
	}

	myApp := &MyApp{
		Name:    "example",
		Version: "0.1.0",
	}

	app := keel.New(
		keel.WithConfig[MyApp](cfg),
		keel.WithContext(myApp),
	)

	app.BindFunc(func(c *keel.Context[MyApp]) error {
		c.Set("requestStartedAt", time.Now().UTC())
		c.Set("requestMethod", c.Request.Method)
		return c.Next()
	})

	app.GET("/", func(c *keel.Context[MyApp]) error {
		return c.JSON(http.StatusOK, map[string]any{
			"service": c.App.Name,
			"version": c.App.Version,
			"env":     env,
			"isDev":   app.Config().Projectconfig.IsDev,
		})
	})

	app.GET("/health", func(c *keel.Context[MyApp]) error {
		return c.JSON(http.StatusOK, map[string]any{
			"ok":        true,
			"method":    c.Get("requestMethod"),
			"startedAt": c.Get("requestStartedAt"),
		})
	})

	api := app.Group("/api")
	api.BindFunc(func(c *keel.Context[MyApp]) error {
		c.Set("scope", "api")
		return c.Next()
	})
	api.GET("/me", func(c *keel.Context[MyApp]) error {
		return c.JSON(http.StatusOK, map[string]any{
			"name":    c.App.Name,
			"version": c.App.Version,
			"scope":   c.Get("scope"),
		})
	})

	app.OnBootstrap().BindFunc(func(e *keel.BootstrapEvent[MyApp]) error {
		slog.Info("application bootstrapped", "name", e.App.Name)
		return e.Next()
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
