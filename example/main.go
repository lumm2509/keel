package main

import (
	"context"
	"log"
	"log/slog"
	stdhttp "net/http"
	"time"

	"github.com/lumm2509/keel"
	"github.com/lumm2509/keel/config"
	"github.com/lumm2509/keel/container"
	transporthttp "github.com/lumm2509/keel/transport/http"
)

type Cradle struct {
	Name string
}

func main() {
	cfg := &config.ConfigModule{
		Projectconfig: config.ProjectConfigOptions{
			IsDev:       true,
			DatabaseUrl: ptr("postgres://postgres:postgres@localhost:5432/keel_example?sslmode=disable"),
		},
		Logger: slog.Default(),
	}

	ctr := container.LoadBasecontainer(cfg, &Cradle{Name: "example"})

	app := keel.New(keel.Config[Cradle]{
		Container: ctr,
		BindRoutes: func(ctr container.Container[Cradle]) (*transporthttp.Router[*container.RequestEvent[Cradle]], error) {
			router := transporthttp.NewRouter(func(w stdhttp.ResponseWriter, r *stdhttp.Request) (*container.RequestEvent[Cradle], transporthttp.EventCleanupFunc) {
				return &container.RequestEvent[Cradle]{
					Container: ctr,
					Event: transporthttp.Event{
						Response: w,
						Request:  r,
					},
				}, nil
			})

			router.GET("/foo", func(e *container.RequestEvent[Cradle]) error {
				return e.JSON(200, map[string]any{
					"route":  "foo",
					"cradle": e.Container.Cradle().Name,
				})
			})

			router.GET("/bar", func(e *container.RequestEvent[Cradle]) error {
				return e.JSON(200, map[string]any{
					"route": "bar",
					"time":  time.Now().UTC().Format(time.RFC3339),
				})
			})

			return router, nil
		},
		HMR: func(ctx context.Context) error {
			<-ctx.Done()
			return ctx.Err()
		},
	})

	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}

func ptr[T any](v T) *T {
	return &v
}
