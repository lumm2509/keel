package grpc

import "github.com/lumm2509/keel/runtime/hook"

// Route is a type alias for hook.Route.
type Route[T Resolver] = hook.Route[T]
