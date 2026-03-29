package ws

import (
	"regexp"

	"github.com/lumm2509/keel/runtime/hook"
)

// RouterGroup represents a collection of websocket routes and nested groups
// that share a common prefix and middleware chain.
type RouterGroup[T hook.Resolver] struct {
	ExcludedMiddlewares map[string]struct{}
	Children            []any // *Route[T] or *RouterGroup[T]

	Prefix      string
	Middlewares []*hook.Handler[T]
}

func (group *RouterGroup[T]) Group(prefix string) *RouterGroup[T] {
	newGroup := &RouterGroup[T]{Prefix: prefix}
	group.Children = append(group.Children, newGroup)
	return newGroup
}

func (group *RouterGroup[T]) BindFunc(middlewareFuncs ...func(e T) error) *RouterGroup[T] {
	for _, m := range middlewareFuncs {
		group.Middlewares = hook.AppendSortedHandlers(group.Middlewares, &hook.Handler[T]{Func: m})
	}
	return group
}

func (group *RouterGroup[T]) Bind(middlewares ...*hook.Handler[T]) *RouterGroup[T] {
	group.Middlewares = hook.AppendSortedHandlers(group.Middlewares, middlewares...)

	if group.ExcludedMiddlewares != nil {
		for _, m := range middlewares {
			if m.Id != "" {
				delete(group.ExcludedMiddlewares, m.Id)
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

		for i := len(group.Children) - 1; i >= 0; i-- {
			switch v := group.Children[i].(type) {
			case *RouterGroup[T]:
				v.Unbind(middlewareId)
			case *Route[T]:
				v.Unbind(middlewareId)
			}
		}

		if group.ExcludedMiddlewares == nil {
			group.ExcludedMiddlewares = map[string]struct{}{}
		}
		group.ExcludedMiddlewares[middlewareId] = struct{}{}
	}
	return group
}

func (group *RouterGroup[T]) Route(path string, action func(e T) error) *Route[T] {
	route := &Route[T]{
		Path:   path,
		Action: action,
	}
	group.Children = append(group.Children, route)
	return route
}

func (group *RouterGroup[T]) Handle(path string, action func(e T) error) *Route[T] {
	return group.Route(path, action)
}

func (group *RouterGroup[T]) HasRoute(path string) bool {
	return group.hasRoute(path, nil)
}

func (group *RouterGroup[T]) hasRoute(path string, parents []*RouterGroup[T]) bool {
	for _, child := range group.Children {
		switch v := child.(type) {
		case *RouterGroup[T]:
			if v.hasRoute(path, append(parents, group)) {
				return true
			}
		case *Route[T]:
			var result string

			for _, p := range parents {
				result += p.Prefix
			}

			result += group.Prefix
			result += v.Path

			if result == path || stripWildcard(result) == stripWildcard(path) {
				return true
			}
		}
	}
	return false
}

var wildcardPlaceholderRegex = regexp.MustCompile(`/{.+\.\.\.}$`)

func stripWildcard(pattern string) string {
	return wildcardPlaceholderRegex.ReplaceAllString(pattern, "/")
}
