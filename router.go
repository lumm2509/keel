package keel

import (
	transporthttp "github.com/lumm2509/keel/transport/http"
)

type Context[Cradle any] = transporthttp.RequestEvent[Cradle]
type HandlerFunc[Cradle any] func(*Context[Cradle]) error

type Group[Cradle any] = transporthttp.RouterGroup[*transporthttp.RequestEvent[Cradle]]

type Router[T any] = transporthttp.Router[*transporthttp.RequestEvent[T]]
