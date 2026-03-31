// Package main demonstrates the minimal keel app: create, one route, start.
//
// Shows:
//   - keel.New with a typed app context
//   - registering a single GET route
//   - OnBootstrap / OnServe / OnTerminate lifecycle hooks
package main

import (
	"log"
	"log/slog"
	"net/http"

	"github.com/lumm2509/keel"
	"github.com/lumm2509/keel/config"
)

type App struct {
	Name    string
	Version string
}

func main() {
	app := keel.New(keel.Config[App]{
		Context: &App{Name: "basic", Version: "1.0.0"},
		Module:  &config.Config{},
	})

	app.GET("/", func(c *keel.Context[App]) error {
		return c.JSON(http.StatusOK, map[string]string{
			"service": c.App.Name,
			"version": c.App.Version,
		})
	})

	app.OnBootstrap().BindFunc(func(e *keel.BootstrapEvent[App]) error {
		slog.Info("bootstrapped", "app", e.App.Name)
		return e.Next()
	})

	app.OnServe().BindFunc(func(e *keel.ServeEvent[App]) error {
		slog.Info("listening", "addr", e.Server.Addr)
		return e.Next()
	})

	app.OnTerminate().BindFunc(func(e *keel.TerminateEvent[App]) error {
		slog.Info("shutting down", "restart", e.IsRestart)
		return e.Next()
	})

	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
