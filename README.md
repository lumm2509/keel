<p align="center">
    <a href="https://github.com/lumm2509/keel" target="_blank" rel="noopener">
        <img src=".github/gopher-keel.png" alt="keel - backend toolkit for Go" />
    </a>
</p>

# keel

`keel` is a Go backend toolkit for building HTTP APIs around a typed app container, explicit request events, and a small routing facade.

The short version: you can start with `keel.New(...)`, register routes, and run a server without wiring everything by hand. The less short version: this is not trying to hide the underlying parts forever. The higher-level API is just a thinner entry point over `container`, `transport/http`, and the hook layer.

It fits when you want:

- typed app dependencies in handlers
- a simple HTTP entry point
- the option to drop into lower-level pieces without rewriting everything

It does not fit when you want:

- a finished full-stack framework
- a frozen public API
- a toolkit that protects you from every bad architectural decision

## What This Actually Is

The right mental model is:

- `keel.App[C]` is the main entry point for the common path
- `Cradle` is your typed application dependency bundle
- `container.Container[C]` holds shared app state and resources
- `keel.Context[C]` is really a `container.RequestEvent[C]`
- routing and middleware execution are built on top of `transport/http`
- lifecycle events are handled through the hook layer

So this is not a giant abstraction layer. It is closer to a set of backend building blocks with a convenient facade on top.

What it abstracts:

- app bootstrap and shutdown flow
- route registration
- typed handler access to app dependencies
- a base container with logger, store, cron, subscriptions, and optional DB

What it does not abstract:

- your domain model
- your module boundaries
- persistence design
- production operations
- the need to understand regular Go HTTP behavior

## What It Is Not

- Not a magic framework.
- Not a DI system with deep wiring rules.
- Not a replacement for understanding `net/http`.
- Not a complete backend product with admin, auth, data model, and ops already solved.
- Not stable enough yet to pretend everything public is final.

## Design Choices

### Keep the common path simple

The top-level API is intentionally small:

- `keel.New(...)`
- `app.Use(...)`
- `app.Get(...)`
- `app.Run()`

This is the fast path. It exists because writing the same setup code over and over is boring.

### Keep the lower-level parts public

If you need more control, the lower-level packages are still there:

- `container`
- `transport/http`
- the hook layer
- `apis`

This optimizes for control and inspectability over a perfectly polished surface. The trade-off is that the public API boundary is not especially strict yet.

### Use explicit flow in hooks and middleware

Hook and middleware chains advance via `e.Next()`.

That is deliberate. It keeps execution order obvious and avoids hidden control flow. It also means forgetting `Next()` can stop a chain in ways that are technically correct and operationally annoying.

### Stay close to the stdlib

The server is `http.Server`, routing is based on `http.ServeMux`, and the project does not try to invent a separate universe for HTTP.

That keeps the system easier to reason about, but it also means you inherit the normal stdlib constraints.

## System Invariants

Read this before changing core behavior:

- Do not mix the route facade (`app.Get`, `app.Use`, etc.) with `BindRoutes`. The app rejects that combination.
- Do not expect the app to run without routes. If neither facade routes nor `BindRoutes` are provided, `develop` fails.
- Do not break the `bootstrap -> serve -> terminate` lifecycle assumptions unless you are intentionally redesigning startup behavior.
- Do not forget that hook and middleware execution is manual-chain based. `Next()` is not optional if you want the chain to continue.
- Do not assume the container always has a DB. `DataBase()` may be `nil`, and that is a valid state.
- Do not treat `ConfigModule` as an architectural layer. It is just a config aggregate.
- Do not casually change middleware or hook ordering. Priority and registration order matter.
- Do not break `RequestEvent` behavior unless you want to chase logging, request parsing, and route metadata regressions afterward.
- Do not remove the router catch-all behavior lightly. It exists so group middleware still runs on not-found paths.

## How To Think About It

Use this split:

1. `keel.App`, `keel.Context`, `keel.Group` for the normal app path.
2. `container` for shared app state and resources.
3. `transport/http` and the hook layer for lower-level composition.

If you are extending the project:

- add behavior at the lowest layer that actually owns the problem
- only expose it through `App` if it improves the common path
- avoid turning `App` into a bag of special cases

Common ways to make a mess:

- putting domain logic into middleware because it is convenient
- turning `Cradle` into a global junk drawer
- assuming config and resources are always fully present
- changing hook flow without understanding who calls `Next()`

## Minimal Example

```go
package main

import (
	"log"
	"net/http"

	"github.com/lumm2509/keel"
)

type Cradle struct {
	Name string
}

func main() {
	app := keel.New(
		keel.WithCradle(Cradle{Name: "example"}),
	)

	app.Use(func(c *keel.Context[Cradle]) error {
		c.Set("scope", "public")
		return c.Next()
	})

	app.Get("/health", func(c *keel.Context[Cradle]) error {
		return c.JSON(http.StatusOK, map[string]any{
			"ok":    true,
			"name":  c.Cradle().Name,
			"scope": c.Get("scope"),
			"hasDB": c.Container.DataBase() != nil,
		})
	})

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
```

For a slightly larger example, see `example/main.go`.

## Project Status

Current status: experimental.

Things that look reasonably solid:

- the basic `App + Cradle + Container + Router` shape
- the hook implementation
- the HTTP transport layer
- several utility and infrastructure packages

Things still in motion:

- the final public API shape
- consistency between the top-level package and lower-level packages
- documentation
- cleanup of older naming and repo history

Things that may still be broken or half-finished:

- some integration points across the full repo
- parts of the public surface that are still being aligned
- leftover assumptions from earlier iterations of the project

This repository is not at the stage where “public” automatically means “settled”.

## Contributing

Useful contributions:

- targeted fixes with clear scope
- tests for actual request flow and hook behavior
- docs that explain real constraints and trade-offs
- cleanup that reduces confusion between the facade and the lower-level APIs

Likely bad contributions:

- adding another abstraction layer without solving a concrete problem
- large feature additions on top of unstable core behavior
- PRs that make the API broader without making the system clearer
- documentation that sells the project harder than the code can support

If you touch hooks, routing, or container behavior, explain the full execution impact. If you change public API, explain the trade-off. If a change mostly increases surface area, it probably needs a better reason.

## Local Development

Useful commands:

```sh
make test
make lint
go run ./example
```

Just do not assume the existence of a command means the whole repo is already polished. It is not there yet.
