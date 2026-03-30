package http

import "github.com/lumm2509/keel/runtime/hook"

// Route represents a single registered route with its action and optional middleware chain.
type Route[T hook.Resolver] struct {
	ExcludedMiddlewares map[string]struct{}

	Action       func(e T) error
	Method       string
	Path         string
	Middlewares  []*hook.Handler[T]
	Handle       any                 // arbitrary metadata readable by middleware via RouteHandleAs
	ErrorHandler func(T, error) error // error boundary for this route; nil = propagate
}

// BindFunc registers one or multiple middleware functions to the current route.
func (route *Route[T]) BindFunc(middlewareFuncs ...func(e T) error) *Route[T] {
	for _, m := range middlewareFuncs {
		route.Middlewares = AppendSortedHandlers(route.Middlewares, &hook.Handler[T]{Func: m})
	}

	return route
}

// Bind registers one or multiple middleware handlers to the current route.
func (route *Route[T]) Bind(middlewares ...*hook.Handler[T]) *Route[T] {
	route.Middlewares = AppendSortedHandlers(route.Middlewares, middlewares...)

	if route.ExcludedMiddlewares != nil {
		for _, m := range middlewares {
			if m.Id != "" {
				delete(route.ExcludedMiddlewares, m.Id)
			}
		}
	}

	return route
}

// Unbind removes one or more middlewares with the specified id(s) from the current route.
func (route *Route[T]) Unbind(middlewareIds ...string) *Route[T] {
	for _, middlewareId := range middlewareIds {
		if middlewareId == "" {
			continue
		}

		for i := len(route.Middlewares) - 1; i >= 0; i-- {
			if route.Middlewares[i].Id == middlewareId {
				route.Middlewares = append(route.Middlewares[:i], route.Middlewares[i+1:]...)
			}
		}

		if route.ExcludedMiddlewares == nil {
			route.ExcludedMiddlewares = map[string]struct{}{}
		}
		route.ExcludedMiddlewares[middlewareId] = struct{}{}
	}

	return route
}

// WithHandle attaches arbitrary metadata to the route.
// Middleware can retrieve it via keel.RouteHandleAs.
func (route *Route[T]) WithHandle(h any) *Route[T] {
	route.Handle = h
	return route
}

// OnError registers an error boundary for this route.
// When the route's handler chain returns an error, fn is called before falling
// through to any parent group boundary or the global error handler.
// Return nil from fn to suppress the error, or return a different error to
// replace it.
func (route *Route[T]) OnError(fn func(T, error) error) *Route[T] {
	route.ErrorHandler = fn
	return route
}
