package hook

import (
	"net/http"
	"regexp"
	"strings"
)

// RouterGroup represents a collection of routes and other sub groups
// that share common pattern prefix and middlewares.
//
// (note: the struct is named RouterGroup instead of Group so that it can
// be embedded in the Router without conflicting with the Group method)
type RouterGroup[T Resolver] struct {
	ExcludedMiddlewares map[string]struct{}
	Children            []any // *Route[T] or *RouterGroup[T]

	Prefix      string
	Middlewares []*Handler[T]
}

// Group creates and registers a new child Group into the current one
// with the specified prefix.
func (group *RouterGroup[T]) Group(prefix string) *RouterGroup[T] {
	newGroup := &RouterGroup[T]{}
	newGroup.Prefix = prefix

	group.Children = append(group.Children, newGroup)

	return newGroup
}

// BindFunc registers one or multiple middleware functions to the current group.
func (group *RouterGroup[T]) BindFunc(middlewareFuncs ...func(e T) error) *RouterGroup[T] {
	for _, m := range middlewareFuncs {
		group.Middlewares = appendSortedHandlers(group.Middlewares, &Handler[T]{Func: m})
	}

	return group
}

// Bind registers one or multiple middleware handlers to the current group.
func (group *RouterGroup[T]) Bind(middlewares ...*Handler[T]) *RouterGroup[T] {
	group.Middlewares = appendSortedHandlers(group.Middlewares, middlewares...)

	if group.ExcludedMiddlewares != nil {
		for _, m := range middlewares {
			if m.Id != "" {
				delete(group.ExcludedMiddlewares, m.Id)
			}
		}
	}

	return group
}

// Unbind removes one or more middlewares with the specified id(s)
// from the current group and its children (if any).
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

// Route registers a single route into the current group.
func (group *RouterGroup[T]) Route(method string, path string, action func(e T) error) *Route[T] {
	route := &Route[T]{
		Method: method,
		Path:   path,
		Action: action,
	}

	group.Children = append(group.Children, route)

	return route
}

// Any is a shorthand for Route with "" as route method (matches any method).
func (group *RouterGroup[T]) Any(path string, action func(e T) error) *Route[T] {
	return group.Route("", path, action)
}

// GET is a shorthand for Route with GET as route method.
func (group *RouterGroup[T]) GET(path string, action func(e T) error) *Route[T] {
	return group.Route(http.MethodGet, path, action)
}

// SEARCH is a shorthand for Route with SEARCH as route method.
func (group *RouterGroup[T]) SEARCH(path string, action func(e T) error) *Route[T] {
	return group.Route("SEARCH", path, action)
}

// POST is a shorthand for Route with POST as route method.
func (group *RouterGroup[T]) POST(path string, action func(e T) error) *Route[T] {
	return group.Route(http.MethodPost, path, action)
}

// DELETE is a shorthand for Route with DELETE as route method.
func (group *RouterGroup[T]) DELETE(path string, action func(e T) error) *Route[T] {
	return group.Route(http.MethodDelete, path, action)
}

// PATCH is a shorthand for Route with PATCH as route method.
func (group *RouterGroup[T]) PATCH(path string, action func(e T) error) *Route[T] {
	return group.Route(http.MethodPatch, path, action)
}

// PUT is a shorthand for Route with PUT as route method.
func (group *RouterGroup[T]) PUT(path string, action func(e T) error) *Route[T] {
	return group.Route(http.MethodPut, path, action)
}

// HEAD is a shorthand for Route with HEAD as route method.
func (group *RouterGroup[T]) HEAD(path string, action func(e T) error) *Route[T] {
	return group.Route(http.MethodHead, path, action)
}

// OPTIONS is a shorthand for Route with OPTIONS as route method.
func (group *RouterGroup[T]) OPTIONS(path string, action func(e T) error) *Route[T] {
	return group.Route(http.MethodOptions, path, action)
}

// HasRoute checks whether the specified route pattern (method + path)
// is registered in the current group or its children.
func (group *RouterGroup[T]) HasRoute(method string, path string) bool {
	pattern := path
	if method != "" {
		pattern = strings.ToUpper(method) + " " + pattern
	}

	return group.hasRoute(pattern, nil)
}

func (group *RouterGroup[T]) hasRoute(pattern string, parents []*RouterGroup[T]) bool {
	for _, child := range group.Children {
		switch v := child.(type) {
		case *RouterGroup[T]:
			if v.hasRoute(pattern, append(parents, group)) {
				return true
			}
		case *Route[T]:
			var result string

			if v.Method != "" {
				result += v.Method + " "
			}

			for _, p := range parents {
				result += p.Prefix
			}

			result += group.Prefix
			result += v.Path

			if result == pattern ||
				stripWildcard(result) == stripWildcard(pattern) {
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
