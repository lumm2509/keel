package keel

import (
	stdhttp "net/http"

	"github.com/lumm2509/keel/container"
	transporthttp "github.com/lumm2509/keel/transport/http"
)

type Context[Cradle any] struct {
	event *container.RequestEvent[Cradle]
}

func newContext[Cradle any](event *container.RequestEvent[Cradle]) *Context[Cradle] {
	return &Context[Cradle]{event: event}
}

func (c *Context[Cradle]) Cradle() *Cradle {
	return c.event.Container.Cradle()
}

func (c *Context[Cradle]) Request() *stdhttp.Request {
	return c.event.Request
}

func (c *Context[Cradle]) Response() stdhttp.ResponseWriter {
	return c.event.Response
}

func (c *Context[Cradle]) JSON(status int, data any) error {
	return c.event.JSON(status, data)
}

func (c *Context[Cradle]) String(status int, data string) error {
	return c.event.String(status, data)
}

func (c *Context[Cradle]) HTML(status int, data string) error {
	return c.event.HTML(status, data)
}

func (c *Context[Cradle]) XML(status int, data any) error {
	return c.event.XML(status, data)
}

func (c *Context[Cradle]) Redirect(status int, url string) error {
	return c.event.Redirect(status, url)
}

type routeRegistration[Cradle any] struct {
	method  string
	path    string
	handler func(*Context[Cradle]) error
}

func (a *App[Cradle]) Group(prefix string) *Group[Cradle] {
	return &Group[Cradle]{app: a, prefix: prefix}
}

func (a *App[Cradle]) Any(path string, handler func(*Context[Cradle]) error) {
	a.routes = append(a.routes, routeRegistration[Cradle]{path: path, handler: handler})
}

func (a *App[Cradle]) GET(path string, handler func(*Context[Cradle]) error) {
	a.routes = append(a.routes, routeRegistration[Cradle]{method: stdhttp.MethodGet, path: path, handler: handler})
}

func (a *App[Cradle]) POST(path string, handler func(*Context[Cradle]) error) {
	a.routes = append(a.routes, routeRegistration[Cradle]{method: stdhttp.MethodPost, path: path, handler: handler})
}

func (a *App[Cradle]) PUT(path string, handler func(*Context[Cradle]) error) {
	a.routes = append(a.routes, routeRegistration[Cradle]{method: stdhttp.MethodPut, path: path, handler: handler})
}

func (a *App[Cradle]) DELETE(path string, handler func(*Context[Cradle]) error) {
	a.routes = append(a.routes, routeRegistration[Cradle]{method: stdhttp.MethodDelete, path: path, handler: handler})
}

func (a *App[Cradle]) PATCH(path string, handler func(*Context[Cradle]) error) {
	a.routes = append(a.routes, routeRegistration[Cradle]{method: stdhttp.MethodPatch, path: path, handler: handler})
}

func (a *App[Cradle]) HEAD(path string, handler func(*Context[Cradle]) error) {
	a.routes = append(a.routes, routeRegistration[Cradle]{method: stdhttp.MethodHead, path: path, handler: handler})
}

func (a *App[Cradle]) OPTIONS(path string, handler func(*Context[Cradle]) error) {
	a.routes = append(a.routes, routeRegistration[Cradle]{method: stdhttp.MethodOptions, path: path, handler: handler})
}

func (a *App[Cradle]) bindRegisteredRoutes(ctr container.Container[Cradle]) (*transporthttp.Router[*container.RequestEvent[Cradle]], error) {
	router := transporthttp.NewRouter(func(w stdhttp.ResponseWriter, r *stdhttp.Request) (*container.RequestEvent[Cradle], transporthttp.EventCleanupFunc) {
		return &container.RequestEvent[Cradle]{
			Container: ctr,
			Event: transporthttp.Event{
				Response: w,
				Request:  r,
			},
		}, nil
	})

	for _, route := range a.routes {
		registerRoute(router.RouterGroup, route)
	}

	return router, nil
}

func (a *App[Cradle]) composeBindRoutes() func(container.Container[Cradle]) (*transporthttp.Router[*container.RequestEvent[Cradle]], error) {
	advancedBindRoutes := a.bindRoutes

	return func(ctr container.Container[Cradle]) (*transporthttp.Router[*container.RequestEvent[Cradle]], error) {
		if advancedBindRoutes == nil {
			return a.bindRegisteredRoutes(ctr)
		}

		router, err := advancedBindRoutes(ctr)
		if err != nil {
			return nil, err
		}

		for _, route := range a.routes {
			registerRoute(router.RouterGroup, route)
		}

		return router, nil
	}
}

type Group[Cradle any] struct {
	app    *App[Cradle]
	prefix string
}

func (g *Group[Cradle]) Any(path string, handler func(*Context[Cradle]) error) {
	g.app.Any(g.prefix+path, handler)
}

func (g *Group[Cradle]) GET(path string, handler func(*Context[Cradle]) error) {
	g.app.GET(g.prefix+path, handler)
}

func (g *Group[Cradle]) POST(path string, handler func(*Context[Cradle]) error) {
	g.app.POST(g.prefix+path, handler)
}

func (g *Group[Cradle]) PUT(path string, handler func(*Context[Cradle]) error) {
	g.app.PUT(g.prefix+path, handler)
}

func (g *Group[Cradle]) DELETE(path string, handler func(*Context[Cradle]) error) {
	g.app.DELETE(g.prefix+path, handler)
}

func (g *Group[Cradle]) PATCH(path string, handler func(*Context[Cradle]) error) {
	g.app.PATCH(g.prefix+path, handler)
}

func (g *Group[Cradle]) HEAD(path string, handler func(*Context[Cradle]) error) {
	g.app.HEAD(g.prefix+path, handler)
}

func (g *Group[Cradle]) OPTIONS(path string, handler func(*Context[Cradle]) error) {
	g.app.OPTIONS(g.prefix+path, handler)
}

func registerRoute[Cradle any](group *transporthttp.RouterGroup[*container.RequestEvent[Cradle]], route routeRegistration[Cradle]) {
	group.Route(route.method, route.path, func(event *container.RequestEvent[Cradle]) error {
		return route.handler(newContext(event))
	})
}
