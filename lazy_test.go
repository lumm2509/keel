package keel

import (
	"errors"
	"sync/atomic"
	"testing"
)

func TestLazyHandlerFactoryCalledOnce(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	h := LazyHandler(func() HandlerFunc[struct{}] {
		calls.Add(1)
		return func(c *Context[struct{}]) error { return nil }
	})

	c := newTestContext()
	for i := 0; i < 5; i++ {
		if err := h(c); err != nil {
			t.Fatalf("call %d: unexpected error: %v", i, err)
		}
	}

	if calls.Load() != 1 {
		t.Fatalf("expected factory to be called once, got %d", calls.Load())
	}
}

func TestLazyHandlerActionExecuted(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("sentinel")
	h := LazyHandler(func() HandlerFunc[struct{}] {
		return func(c *Context[struct{}]) error { return sentinel }
	})

	c := newTestContext()
	if err := h(c); !errors.Is(err, sentinel) {
		t.Fatalf("expected sentinel error, got %v", err)
	}
}
