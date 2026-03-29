package keel

import (
	transporthttp "github.com/lumm2509/keel/transport/http"
)

type Context[Cradle any] = transporthttp.RequestEvent[Cradle]
type HandlerFunc[Cradle any] func(*Context[Cradle]) error

// Group is a type alias for the HTTP router group — zero overhead, same type throughout.
type Group[Cradle any] = transporthttp.RouterGroup[*transporthttp.RequestEvent[Cradle]]
