# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What is Keel

Keel is a lightweight, opinionated Go backend framework — a reusable base for building real applications. It provides typed app state, HTTP routing, request context management, lifecycle hooks, and shared runtime utilities. It is **not** a BaaS, admin panel, or auth platform; it is a practical foundation that avoids framework bloat.

Philosophy: stdlib-first, strongly typed via generics, minimal forced abstractions, single "normal path".

## Commands

```bash
make lint        # Run golangci-lint
make test        # Run tests with coverage
make test-report # Generate HTML coverage report
make jstypes     # Generate JS VM types
```

To run a single test:
```bash
go test ./transport/http/... -run TestFoo
```

Go version: 1.26.1

## Architecture

### Core startup flow

```
keel.New(options...)           # Create App[T] with typed context
  app.OnBootstrap().Bind(...)  # Register lifecycle hooks
  app.BindFunc(...)            # Register global middleware
  app.GET/POST/Group(...)      # Register routes
  app.Start()                  # Run CLI (serve/develop), graceful shutdown
```

### Request lifecycle

Every HTTP request produces a `RequestEvent[T]` (pooled) that carries `Response`, `Request`, `App *T`, and a key-value `EventData` store. Middleware and handlers form an explicit chain — **you must call `e.Next()`** to continue. Not calling it short-circuits the chain.

### Package map

| Path | Responsibility |
|------|---------------|
| `keel.go` | `App[T]` struct, lifecycle hooks (`OnBootstrap`, `OnServe`, `OnTerminate`), routing facade |
| `transport/http/` | Router, `RequestEvent[T]`, route groups, binding/response helpers, CORS |
| `runtime/hook/` | Generic concurrent-safe `Hook[T]` and handler chain |
| `runtime/cron/` | Crontab-style job scheduler (1-min tick) |
| `infra/store/` | Thread-safe generic in-memory KV store |
| `infra/database/` | pgx/v5 PostgreSQL connection pooling with retry logic |
| `infra/filesystem/` | File upload, image resize/crop, S3 + local blob backends |
| `infra/security/` | Token gen/validation, password hashing, proxy IP resolution |
| `config/` | `ConfigModule` — HTTP settings, data dir, TLS, slog.Logger |
| `apis/` | Server init, CORS middleware, TLS cert management, startup banner |
| `commands/` | Cobra CLI commands (`develop` with optional HMR) |
| `pkg/types/` | Domain types with SQL drivers: `DateTime`, `GeoPoint`, JSON wrappers |
| `pkg/search/` | Query/filter parsing |
| `pkg/inflector/` | String inflection (snake_case, plural, etc.) |
| `pkg/subscriptions/` | Pub/sub event system |

### Type-safe request context keys

```go
var startTimeKey = keel.NewContextKey[time.Time]()
startTimeKey.Set(c, time.Now())
t, ok := startTimeKey.Get(c)
```

Use `ContextKey[T]` for per-request data, not the app context `T`.

### Route metadata

```go
type RoutePolicy struct{ RequireAuth bool }

app.GET("/admin", handler, func(h *keel.RouteHandle) {
    h.Metadata = &RoutePolicy{RequireAuth: true}
})

// In middleware:
policy, ok := keel.RouteHandleAs[RoutePolicy](c)
```

### Hook chain pattern

```go
h := &hook.Hook[*MyEvent]{}
h.Bind(&hook.Handler[*MyEvent]{
    Id:       "myHandler",
    Priority: 10,
    Func: func(e *MyEvent) error {
        // ... work ...
        return e.Next() // must call to continue chain
    },
})
h.Trigger(&MyEvent{...})
```

## Key design rules

- **Use the facade** — `app.GET()`, `app.Group()`, etc. are the normal path; avoid reaching into lower-level packages directly.
- **`e.Next()` is mandatory** — omitting it silently terminates the chain; this is intentional, not a bug.
- **Keep `T` lean** — the app context is for shared singletons (DB, config), not per-request state.
- **`ConfigModule` is not a catch-all** — don't add fields just because they might be useful.
- Some packages under `pkg/` are intentionally experimental but kept because they're useful in real apps.

## Example app

See `example/main.go` for a complete working example of app setup, middleware, route groups, and lifecycle hooks.
