package grpc

import "github.com/lumm2509/keel/runtime/hook"

type RouterGroup[T Resolver] struct {
	excludedMiddlewares map[string]struct{}
	children            []any // Route or RouterGroup

	Prefix      string
	Middlewares []*hook.Handler[T]
}

func (group *RouterGroup[T]) Group(prefix string) *RouterGroup[T] {
	newGroup := &RouterGroup[T]{Prefix: prefix}
	group.children = append(group.children, newGroup)
	return newGroup
}

func (group *RouterGroup[T]) BindFunc(middlewareFuncs ...func(e T) error) *RouterGroup[T] {
	for _, m := range middlewareFuncs {
		group.Middlewares = append(group.Middlewares, &hook.Handler[T]{Func: m})
	}

	return group
}

func (group *RouterGroup[T]) Bind(middlewares ...*hook.Handler[T]) *RouterGroup[T] {
	group.Middlewares = append(group.Middlewares, middlewares...)

	if group.excludedMiddlewares != nil {
		for _, m := range middlewares {
			if m.Id != "" {
				delete(group.excludedMiddlewares, m.Id)
			}
		}
	}

	return group
}

func (group *RouterGroup[T]) Unbind(middlewareIds ...string) *RouterGroup[T] {
	for _, middlewareId := range middlewareIds {
		if middlewareId == "" {
			continue
		}

		for i := len(group.Middlewares) - 1; i >= 0; i-- {
			if group.Middlewares[i].Id == middlewareId {
				group.Middlewares = append(group.Middlewares[:i], group.Middlewares[i+1:]...)
			}
		}

		for i := len(group.children) - 1; i >= 0; i-- {
			switch v := group.children[i].(type) {
			case *RouterGroup[T]:
				v.Unbind(middlewareId)
			case *Route[T]:
				v.Unbind(middlewareId)
			}
		}

		if group.excludedMiddlewares == nil {
			group.excludedMiddlewares = map[string]struct{}{}
		}
		group.excludedMiddlewares[middlewareId] = struct{}{}
	}

	return group
}

func (group *RouterGroup[T]) Route(method string, action func(e T) error) *Route[T] {
	route := &Route[T]{
		Method: method,
		Action: action,
	}

	group.children = append(group.children, route)

	return route
}

func (group *RouterGroup[T]) HasRoute(fullMethod string) bool {
	return group.hasRoute(cleanFullMethod(fullMethod), nil)
}

func (group *RouterGroup[T]) hasRoute(fullMethod string, parents []*RouterGroup[T]) bool {
	for _, child := range group.children {
		switch v := child.(type) {
		case *RouterGroup[T]:
			if v.hasRoute(fullMethod, append(parents, group)) {
				return true
			}
		case *Route[T]:
			if joinFullMethod(parents, group, v.Method) == fullMethod {
				return true
			}
		}
	}

	return false
}
