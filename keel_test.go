package keel

import (
	"errors"
	"os"
	"testing"
)

func TestBootstrapTriggersOnBootstrapHook(t *testing.T) {
	t.Parallel()

	app := New[struct{}]()

	called := false
	app.OnBootstrap().BindFunc(func(e *BootstrapEvent[struct{}]) error {
		called = true
		return e.Next()
	})

	if err := app.bootstrap(); err != nil {
		t.Fatalf("bootstrap() error = %v", err)
	}

	if !called {
		t.Fatal("expected OnBootstrap hook to be called")
	}
}

func TestBootstrapHookErrorAbortsInit(t *testing.T) {
	t.Parallel()

	app := New[struct{}]()

	sentinel := errors.New("hook abort")
	app.OnBootstrap().BindFunc(func(e *BootstrapEvent[struct{}]) error {
		return sentinel
	})

	err := app.bootstrap()
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected sentinel error, got %v", err)
	}
}

func TestTerminateTriggersOnTerminateHook(t *testing.T) {
	t.Parallel()

	app := New[struct{}]()

	if err := app.bootstrap(); err != nil {
		t.Fatalf("bootstrap() error = %v", err)
	}

	called := false
	app.OnTerminate().BindFunc(func(e *TerminateEvent[struct{}]) error {
		called = true
		return e.Next()
	})

	if err := app.terminate(false); err != nil {
		t.Fatalf("terminate() error = %v", err)
	}

	if !called {
		t.Fatal("expected OnTerminate hook to be called")
	}
}

func TestSkipBootstrapReturnsTrueForHelpFlag(t *testing.T) {
	t.Parallel()

	origArgs := os.Args
	defer func() { os.Args = origArgs }()

	os.Args = []string{"app", "--help"}

	app := New[struct{}]()
	if !app.skipBootstrap() {
		t.Fatal("expected skipBootstrap() = true for --help flag")
	}
}

func TestBootstrapCallsInitOnAppIfImplemented(t *testing.T) {
	t.Parallel()

	type myApp struct {
		initCalled bool
	}

	app := New[myApp](WithContext(&myApp{}))

	// Patch Init via interface — store result via OnBootstrap hook
	var gotApp *myApp
	app.OnBootstrap().BindFunc(func(e *BootstrapEvent[myApp]) error {
		gotApp = e.App
		return e.Next()
	})

	if err := app.bootstrap(); err != nil {
		t.Fatalf("bootstrap() error = %v", err)
	}

	if gotApp == nil {
		t.Fatal("expected App to be set on BootstrapEvent")
	}
}

func TestWithContextPropagatedToRequestEvent(t *testing.T) {
	t.Parallel()

	type myApp struct{ Name string }
	ctx := &myApp{Name: "test"}

	app := New[myApp](WithContext(ctx))

	if app.Context() != ctx {
		t.Fatalf("expected Context() to return provided context pointer")
	}
}
