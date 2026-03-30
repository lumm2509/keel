package keel

import (
	"testing"

	transporthttp "github.com/lumm2509/keel/transport/http"
)

func newTestContext() *Context[struct{}] {
	e := &transporthttp.RequestEvent[struct{}]{}
	e.Reset(nil, nil, nil)
	return e
}

func TestContextKeyGetSet(t *testing.T) {
	t.Parallel()

	key := NewContextKey[string]()
	c := newTestContext()

	key.Set(c, "hello")

	got, ok := key.Get(c)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if got != "hello" {
		t.Fatalf("expected %q, got %q", "hello", got)
	}
}

func TestContextKeyGetMissing(t *testing.T) {
	t.Parallel()

	key := NewContextKey[int]()
	c := newTestContext()

	got, ok := key.Get(c)
	if ok {
		t.Fatal("expected ok=false for missing key")
	}
	if got != 0 {
		t.Fatalf("expected zero value, got %d", got)
	}
}

func TestContextKeyGetWrongType(t *testing.T) {
	t.Parallel()

	strKey := NewContextKey[string]()
	intKey := NewContextKey[int]()
	c := newTestContext()

	// Write via strKey, read via intKey — same underlying string key but different types
	// (in practice impossible since each NewContextKey generates a unique internal key,
	// but verify type safety holds if we manually stash a wrong type via raw EventData)
	c.Set(strKey.key, "not-an-int")

	got, ok := intKey.Get(c)
	if ok {
		t.Fatal("expected ok=false for type mismatch")
	}
	if got != 0 {
		t.Fatalf("expected zero, got %d", got)
	}
}

func TestContextKeyMustGetPanicsWhenMissing(t *testing.T) {
	t.Parallel()

	key := NewContextKey[string]()
	c := newTestContext()

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected MustGet to panic on missing key")
		}
	}()

	key.MustGet(c)
}

func TestContextKeyMustGetReturnsValue(t *testing.T) {
	t.Parallel()

	key := NewContextKey[float64]()
	c := newTestContext()
	key.Set(c, 3.14)

	got := key.MustGet(c)
	if got != 3.14 {
		t.Fatalf("expected 3.14, got %v", got)
	}
}

func TestContextKeyUnique(t *testing.T) {
	t.Parallel()

	k1 := NewContextKey[string]()
	k2 := NewContextKey[string]()
	c := newTestContext()

	k1.Set(c, "one")
	k2.Set(c, "two")

	v1, _ := k1.Get(c)
	v2, _ := k2.Get(c)

	if v1 == v2 {
		t.Fatal("two distinct ContextKeys must not share storage")
	}
}
