package keel

import (
	"testing"

	transporthttp "github.com/lumm2509/keel/transport/http"
)

type testPolicy struct {
	Auth  string
	Limit int
}

func TestRouteHandleAsReturnsHandle(t *testing.T) {
	t.Parallel()

	c := newTestContext()
	c.Set(transporthttp.EventKeyRouteHandle, testPolicy{Auth: "admin", Limit: 100})

	p, ok := RouteHandleAs[testPolicy](c)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if p.Auth != "admin" || p.Limit != 100 {
		t.Fatalf("unexpected policy: %+v", p)
	}
}

func TestRouteHandleAsWrongType(t *testing.T) {
	t.Parallel()

	c := newTestContext()
	c.Set(transporthttp.EventKeyRouteHandle, "not-a-policy")

	_, ok := RouteHandleAs[testPolicy](c)
	if ok {
		t.Fatal("expected ok=false for wrong type")
	}
}

func TestRouteHandleAsMissing(t *testing.T) {
	t.Parallel()

	c := newTestContext()

	_, ok := RouteHandleAs[testPolicy](c)
	if ok {
		t.Fatal("expected ok=false when handle not set")
	}
}
