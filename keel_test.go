package keel

import (
	"testing"
)

func TestNewWithCradleBuildsDefaultContainer(t *testing.T) {
	t.Parallel()

	type cradle struct {
		Name string
	}

	app := New(WithCradle(cradle{Name: "demo"}))

	if app.container == nil {
		t.Fatalf("expected default container to be created")
	}

	if got := app.container.Cradle().Name; got != "demo" {
		t.Fatalf("expected cradle name %q, got %q", "demo", got)
	}

}
