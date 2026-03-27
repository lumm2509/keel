<p align="center">
    <a href="https://github.com/lumm2509/keel" target="_blank" rel="noopener">
        <img src=".github/gopher-keel.png" alt="keel - backend runtime for Go" />
    </a>
</p>

# keel

Typed backend runtime for Go.

Build an HTTP API with typed dependencies, lifecycle hooks, and escape hatches when you need them.

```go
package main

import (
	"log"

	"github.com/lumm2509/keel"
)

type Cradle struct {
	Name string
}

func main() {
	app := keel.New(keel.WithCradle(Cradle{Name: "example"}))

	app.GET("/hello", func(c *keel.Context[Cradle]) error {
		return c.JSON(200, map[string]string{
			"hello": c.Cradle().Name,
		})
	})

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
```

Status: experimental. API may change before v1.

## Quickstart

The root package is the canonical onboarding path:

- `keel.New(...)`
- `keel.WithCradle(...)`
- `app.GET(...)`
- `app.Run()`

If you need config on day one, add `keel.WithConfig(cfg)`. If you need full runtime control, use `keel.WithContainer(ctr)`.

## What you get

- Typed cradle-backed handlers from the first route.
- A default container and request bridge built for you.
- The same underlying lifecycle and transport primitives used by advanced setups.
- `App` as the visible runtime boundary for the normal path.

## Typed DI

The cradle stays central to the API. `c.Cradle()` gives you typed app dependencies inside each handler without forcing container wiring into the first snippet.

## Advanced Composition

Lower-level APIs remain public for power users:

- `github.com/lumm2509/keel/container`
- `github.com/lumm2509/keel/transport/http`
- `github.com/lumm2509/keel/runtime/hook`

Use those when you want custom router setup, explicit request events, or lifecycle composition beyond the default path.

For the normal path, stay in `keel.App` and `keel.Context`. Reach for `container` only when you are intentionally dropping into advanced composition.

## Project Status

Work in progress.

- Some packages are already useful.
- Public APIs are still moving.
- Docs and examples are being brought back in line with the code.

## Origin

This started as “slightly modify PocketBase for a specific use case”.

That is no longer the product direction, but some naming and comments still reflect that history.
