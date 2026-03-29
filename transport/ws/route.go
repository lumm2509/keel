package ws

import "github.com/lumm2509/keel/runtime/hook"

// Route is a type alias for hook.Route.
type Route[T hook.Resolver] = hook.Route[T]
