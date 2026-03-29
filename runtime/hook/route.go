package hook

// Route represents a single registered route with its action and optional middleware chain.
type Route[T Resolver] struct {
	ExcludedMiddlewares map[string]struct{}

	Action      func(e T) error
	Method      string
	Path        string
	Middlewares []*Handler[T]
}

// BindFunc registers one or multiple middleware functions to the current route.
func (route *Route[T]) BindFunc(middlewareFuncs ...func(e T) error) *Route[T] {
	for _, m := range middlewareFuncs {
		route.Middlewares = appendSortedHandlers(route.Middlewares, &Handler[T]{Func: m})
	}

	return route
}

// Bind registers one or multiple middleware handlers to the current route.
func (route *Route[T]) Bind(middlewares ...*Handler[T]) *Route[T] {
	route.Middlewares = appendSortedHandlers(route.Middlewares, middlewares...)

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
