package keel

import (
	transporthttp "github.com/lumm2509/keel/transport/http"
)

type Context[T any] = transporthttp.RequestEvent[T]
type HandlerFunc[T any] func(*Context[T]) error

type Group[T any] = transporthttp.RouterGroup[*transporthttp.RequestEvent[T]]

type Router[T any] = transporthttp.Router[*transporthttp.RequestEvent[T]]
