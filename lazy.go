package keel

import "sync"

func LazyHandler[T any](factory func() HandlerFunc[T]) HandlerFunc[T] {
	fn := sync.OnceValue(factory)
	return func(c *Context[T]) error {
		return fn()(c)
	}
}
