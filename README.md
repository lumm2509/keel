<p align="center">
  <a href="https://github.com/lumm2509/keel" target="_blank" rel="noopener">
    <img src=".github/gopher-keel.png" alt="keel" />
  </a>
</p>

# keel

`keel` is the Go backend base I got tired of rebuilding over and over.

It is not trying to be the new everything-framework, and it is definitely not pretending to solve every backend problem known to mankind.
What it **does** try to give me is a sane starting point for real apps: typed app state, HTTP routing, request context, lifecycle hooks, and shared runtime capabilities without scaffolding the same junk every single time.

A lot of this exists for a very simple reason: I use it.
Some parts are core, some are experimental, and some are here because they are useful even if they are not part of the main pitch yet.

---

## The normal path

If you're using `keel` the normal way, the flow is:

`New -> register routes -> Start`

That's the main story.
Not because every other path is illegal, but because frameworks get stupid fast when they try to have 4 "official" ways of doing the same thing.

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
        keel.WithContext(&Cradle{Name: "myapp"}),
    )

    app.BindFunc(func(c *keel.Context[Cradle]) error {
        c.Set("scope", "public")
        return c.Next()
    })

    app.GET("/health", func(c *keel.Context[Cradle]) error {
        return c.JSON(http.StatusOK, map[string]any{
            "ok":   true,
            "name": c.App.Name,
        })
    })

    if err := app.Start(); err != nil {
        log.Fatal(err)
    }
}
```

---

## What this actually is

At its core, `keel` is built around a few things:

* a typed app context
* an app/runtime boundary
* a request event/context
* lifecycle hooks
* a small HTTP facade on top of stdlib-first plumbing

That is the useful part.

Everything else in the repo falls into one of these buckets:

* core
* useful but not central
* experimental
* legacy / still being figured out because this started as a fork and I am not going to pretend otherwise

---

## What this is not

This is **not**:

* a BaaS
* an admin panel
* an auth platform
* a plugin marketplace
* a magical architecture cure for bad decisions
* a purity project made to impress framework tourists on the internet

It is a reusable Go backend base meant to save real setup time.

---

## Core packages

These are the parts that matter most:

| Package          | Status   | Why it exists                            |
| ---------------- | -------- | ---------------------------------------- |
| `keel`           | core     | main app facade and lifecycle entrypoint |
| `transport/http` | core     | HTTP routing/runtime plumbing            |
| `runtime/hook`   | core     | hook chain and lifecycle composition     |
| `container`      | core     | shared app/runtime resources             |
| `config`         | core-ish | runtime config the app actually uses     |
| `infra/store`    | core-ish | shared in-memory/runtime state           |
| `infra/database` | core-ish | DB integration used across apps          |

---

## Useful, but not the main pitch

These stay because they are useful for actual development, not because I need them to look elegant in a README.

| Package            | Status       | Notes                                            |
| ------------------ | ------------ | ------------------------------------------------ |
| `dal`              | experimental | useful direction, not part of the main story yet |
| `dml`              | experimental | same deal                                        |
| `infra/filesystem` | experimental | important in real projects, not core-facing yet  |
| `transport/grpc`   | experimental | I use gRPC, so yes, it stays                     |
| `transport/ws`     | experimental | same for WebSockets                              |
| `runtime/cron`     | supporting   | useful runtime capability                        |
| `commands`         | supporting   | CLI/runtime helpers                              |
| `apis`             | supporting   | internal serving composition                     |

---

## Invariants

These are the parts that will bite you if you ignore them.

* The normal route registration path is the facade. If something lower-level exists, that does **not** automatically make it part of the main public story.
* Hook and middleware flow is manual-chain based. If you do not call `Next()`, the chain does not continue. Revolutionary, I know.
* The app context is app/runtime scope, not an excuse to turn every handler into a dependency landfill.
* `ConfigModule` is not supposed to become a trash bag for every feature that might someday exist.
* Not every capability in the repo is equally public, equally stable, or equally important. That is intentional.

---

## Why some things are experimental

Because I would rather keep a useful capability around and be honest that it is still settling, than cut it just to make the repo look artificially clean.

Some things here are not “finished.”
That does **not** automatically mean they are mistakes.
Sometimes it just means they are useful, real, and not fully locked yet.

---

## Extension philosophy

`keel` should have one normal path and a few intentional escape hatches.

That means:

* the facade is the normal path
* lower-level packages exist on purpose
* not every lower-level package is part of the README headline
* flexibility is good
* ambiguity is not
