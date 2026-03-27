package ws

import "github.com/lumm2509/keel/runtime/hook"

type Route[T hook.Resolver] struct {
	excludedMiddlewares map[string]struct{}

	Action      func(e T) error
	Path        string
	Middlewares []*hook.Handler[T]
}

func (route *Route[T]) BindFunc(middlewareFuncs ...func(e T) error) *Route[T] {
	for _, m := range middlewareFuncs {
		route.Middlewares = append(route.Middlewares, &hook.Handler[T]{Func: m})
	}

	return route
}

func (route *Route[T]) Bind(middlewares ...*hook.Handler[T]) *Route[T] {
	route.Middlewares = append(route.Middlewares, middlewares...)

	if route.excludedMiddlewares != nil {
		for _, m := range middlewares {
			if m.Id != "" {
				delete(route.excludedMiddlewares, m.Id)
			}
		}
	}

	return route
}

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

		if route.excludedMiddlewares == nil {
			route.excludedMiddlewares = map[string]struct{}{}
		}
		route.excludedMiddlewares[middlewareId] = struct{}{}
	}

	return route
}
