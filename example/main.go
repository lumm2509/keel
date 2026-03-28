package main

import (
	"log"
	"net/http"
	"time"

	"github.com/lumm2509/keel"
	"github.com/lumm2509/keel/config"
)

type Cradle struct {
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

	app := keel.New(
		keel.WithConfig[Cradle](cfg),
		keel.WithCradle(Cradle{
			Name:    "example",
			Version: "0.1.0",
		}),
	)

	app.Use(func(c *keel.Context[Cradle]) error {
		c.Set("requestStartedAt", time.Now().UTC())
		c.Set("requestMethod", c.Request.Method)
		return c.Next()
	})

	app.Get("/", func(c *keel.Context[Cradle]) error {
		return c.JSON(http.StatusOK, map[string]any{
			"service": c.Cradle().Name,
			"version": c.Cradle().Version,
			"env":     env,
			"isDev":   app.Config().Projectconfig.IsDev,
		})
	})

	app.Get("/health", func(c *keel.Context[Cradle]) error {
		return c.JSON(http.StatusOK, map[string]any{
			"ok":        true,
			"method":    c.Get("requestMethod"),
			"startedAt": c.Get("requestStartedAt"),
			"hasDB":     c.Container.DataBase() != nil,
		})
	})

	app.Group("/api", func(api *keel.Group[Cradle]) {
		api.Use(func(c *keel.Context[Cradle]) error {
			c.Set("scope", "api")
			return c.Next()
		})

		api.Get("/me", func(c *keel.Context[Cradle]) error {
			return c.JSON(http.StatusOK, map[string]any{
				"name":    c.Cradle().Name,
				"version": c.Cradle().Version,
				"scope":   c.Get("scope"),
			})
		})

		api.Group("/admin", func(admin *keel.Group[Cradle]) {
			admin.Get("/summary", func(c *keel.Context[Cradle]) error {
				return c.JSON(http.StatusOK, map[string]any{
					"storeSize": c.Container.Store().Length(),
					"isDev":     c.Container.IsDev(),
					"brokerUp":  c.Container.SubscriptionsBroker() != nil,
				})
			})
		})
	})

	app.OnBootstrap().BindFunc(func(e *keel.BootstrapEvent[Cradle]) error {
		e.Container.Store().Set("bootedAt", time.Now().UTC())
		return e.Next()
	})

	app.OnServe().BindFunc(func(e *keel.ServeEvent[Cradle]) error {
		e.Container.Logger().Info("HTTP server starting", "addr", e.Server.Addr)
		return e.Next()
	})

	app.OnTerminate().BindFunc(func(e *keel.TerminateEvent[Cradle]) error {
		e.Container.Logger().Info("application shutting down", "restart", e.IsRestart)
		return e.Next()
	})

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
