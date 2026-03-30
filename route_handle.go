package keel

import (
	transporthttp "github.com/lumm2509/keel/transport/http"
)

func RouteHandleAs[H any, T any](c *Context[T]) (H, bool) {
	v := c.Get(transporthttp.EventKeyRouteHandle)
	if v == nil {
		var zero H
		return zero, false
	}
	h, ok := v.(H)
	return h, ok
}
