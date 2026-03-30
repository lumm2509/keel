package keel

import (
	"fmt"
	"sync/atomic"
)

type kvGetter interface {
	Get(key string) any
}

type kvSetter interface {
	Set(key string, value any)
}

var contextKeyCounter atomic.Uint64

type ContextKey[T any] struct {
	key string
}

func NewContextKey[T any]() ContextKey[T] {
	return ContextKey[T]{
		key: fmt.Sprintf("__ck_%d", contextKeyCounter.Add(1)),
	}
}

func (k ContextKey[T]) Get(e kvGetter) (T, bool) {
	v := e.Get(k.key)
	if v == nil {
		var zero T
		return zero, false
	}
	val, ok := v.(T)
	return val, ok
}

func (k ContextKey[T]) MustGet(e kvGetter) T {
	val, ok := k.Get(e)
	if !ok {
		panic(fmt.Sprintf("ContextKey.MustGet: key %q not set", k.key))
	}
	return val
}

// Set stores value under this key in the event's data store.
func (k ContextKey[T]) Set(e kvSetter, value T) {
	e.Set(k.key, value)
}
