package keel

import (
	transporthttp "github.com/lumm2509/keel/transport/http"
)

type Context[Cradle any] = transporthttp.RequestEvent[Cradle]
type Handler[Cradle any] func(*Context[Cradle]) error
type Middleware[Cradle any] func(*Context[Cradle]) error

type Group[Cradle any] struct {
	inner *transporthttp.RouterGroup[*transporthttp.RequestEvent[Cradle]]
}

func (g *Group[Cradle]) Use(middlewares ...Middleware[Cradle]) *Group[Cradle] {
	for _, middleware := range middlewares {
		g.inner.BindFunc(middleware)
	}
	return g
}

func (g *Group[Cradle]) Get(path string, handler Handler[Cradle]) *Group[Cradle] {
	g.inner.GET(path, handler)
	return g
}

func (g *Group[Cradle]) GET(path string, handler Handler[Cradle]) *Group[Cradle] {
	return g.Get(path, handler)
}

func (g *Group[Cradle]) Post(path string, handler Handler[Cradle]) *Group[Cradle] {
	g.inner.POST(path, handler)
	return g
}

func (g *Group[Cradle]) POST(path string, handler Handler[Cradle]) *Group[Cradle] {
	return g.Post(path, handler)
}

func (g *Group[Cradle]) Put(path string, handler Handler[Cradle]) *Group[Cradle] {
	g.inner.PUT(path, handler)
	return g
}

func (g *Group[Cradle]) PUT(path string, handler Handler[Cradle]) *Group[Cradle] {
	return g.Put(path, handler)
}

func (g *Group[Cradle]) Patch(path string, handler Handler[Cradle]) *Group[Cradle] {
	g.inner.PATCH(path, handler)
	return g
}

func (g *Group[Cradle]) PATCH(path string, handler Handler[Cradle]) *Group[Cradle] {
	return g.Patch(path, handler)
}

func (g *Group[Cradle]) Delete(path string, handler Handler[Cradle]) *Group[Cradle] {
	g.inner.DELETE(path, handler)
	return g
}

func (g *Group[Cradle]) DELETE(path string, handler Handler[Cradle]) *Group[Cradle] {
	return g.Delete(path, handler)
}

func (g *Group[Cradle]) Head(path string, handler Handler[Cradle]) *Group[Cradle] {
	g.inner.HEAD(path, handler)
	return g
}

func (g *Group[Cradle]) HEAD(path string, handler Handler[Cradle]) *Group[Cradle] {
	return g.Head(path, handler)
}

func (g *Group[Cradle]) Options(path string, handler Handler[Cradle]) *Group[Cradle] {
	g.inner.OPTIONS(path, handler)
	return g
}

func (g *Group[Cradle]) OPTIONS(path string, handler Handler[Cradle]) *Group[Cradle] {
	return g.Options(path, handler)
}

func (g *Group[Cradle]) Group(prefix string, fn func(*Group[Cradle])) *Group[Cradle] {
	child := &Group[Cradle]{inner: g.inner.Group(prefix)}
	if fn != nil {
		fn(child)
	}
	return child
}
