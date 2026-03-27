package grpc

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"reflect"
	"testing"

	"github.com/lumm2509/keel/runtime/hook"
	ggrpc "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

type testEvent struct {
	Event
	trace *[]string
}

func newTestEvent(ctx context.Context, req []byte, info MethodInfo) (*testEvent, EventCleanupFunc) {
	trace := []string{}
	return &testEvent{
		Event: Event{
			Context: ctx,
			Method:  info,
			request: append([]byte(nil), req...),
		},
		trace: &trace,
	}, nil
}

func newBufconnClient(t *testing.T, server *ggrpc.Server) *ggrpc.ClientConn {
	t.Helper()

	listener := bufconn.Listen(1024 * 1024)

	go func() {
		_ = server.Serve(listener)
	}()

	t.Cleanup(func() {
		server.Stop()
		_ = listener.Close()
	})

	conn, err := ggrpc.NewClient(
		"passthrough:///bufnet",
		ggrpc.WithTransportCredentials(insecure.NewCredentials()),
		ggrpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return listener.Dial()
		}),
		ggrpc.WithDefaultCallOptions(ggrpc.ForceCodec(jsonCodec{}), ggrpc.CallContentSubtype(jsonCodec{}.Name())),
	)
	if err != nil {
		t.Fatalf("new grpc client: %v", err)
	}

	t.Cleanup(func() {
		_ = conn.Close()
	})

	return conn
}

func TestRouterBuildServer(t *testing.T) {
	router := NewRouter(newTestEvent)

	api := router.Group("/keel.test.EchoService")
	api.Bind(&hook.Handler[*testEvent]{
		Id: "root",
		Func: func(e *testEvent) error {
			*e.trace = append(*e.trace, "root")
			return e.Next()
		},
	})

	route := api.Route("Echo", func(e *testEvent) error {
		var req struct {
			Name string `json:"name"`
		}
		if err := e.BindJSON(&req); err != nil {
			return err
		}

		*e.trace = append(*e.trace, "action")

		return e.JSON(map[string]any{
			"message": "hi " + req.Name,
			"trace":   *e.trace,
			"method":  e.Method.FullMethod,
		})
	})
	route.BindFunc(func(e *testEvent) error {
		*e.trace = append(*e.trace, "route")
		return e.Next()
	})

	server, err := router.BuildServer()
	if err != nil {
		t.Fatalf("BuildServer() error = %v", err)
	}

	conn := newBufconnClient(t, server)

	var resp json.RawMessage
	if err := conn.Invoke(context.Background(), "/keel.test.EchoService/Echo", json.RawMessage(`{"name":"alice"}`), &resp); err != nil {
		t.Fatalf("invoke grpc method: %v", err)
	}

	var data struct {
		Message string   `json:"message"`
		Method  string   `json:"method"`
		Trace   []string `json:"trace"`
	}
	if err := json.Unmarshal(resp, &data); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if data.Message != "hi alice" {
		t.Fatalf("expected message %q, got %q", "hi alice", data.Message)
	}

	if data.Method != "/keel.test.EchoService/Echo" {
		t.Fatalf("expected full method %q, got %q", "/keel.test.EchoService/Echo", data.Method)
	}

	expectedTrace := []string{"root", "route", "action"}
	if !reflect.DeepEqual(expectedTrace, data.Trace) {
		t.Fatalf("expected trace %v, got %v", expectedTrace, data.Trace)
	}
}

func TestRouterErrorHandler(t *testing.T) {
	router := NewRouter(func(ctx context.Context, req []byte, info MethodInfo) (*Event, EventCleanupFunc) {
		return &Event{
			Context: ctx,
			Method:  info,
			request: append([]byte(nil), req...),
		}, nil
	})
	router.Group("/keel.test.FailService").Route("Fail", func(e *Event) error {
		return errors.New("boom")
	})

	server, err := router.BuildServer()
	if err != nil {
		t.Fatalf("BuildServer() error = %v", err)
	}

	conn := newBufconnClient(t, server)

	var resp json.RawMessage
	err = conn.Invoke(context.Background(), "/keel.test.FailService/Fail", json.RawMessage(`{}`), &resp)
	if err == nil {
		t.Fatalf("expected grpc invoke to fail")
	}

	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected grpc status error, got %v", err)
	}

	if st.Code() != codes.Internal {
		t.Fatalf("expected grpc code %v, got %v", codes.Internal, st.Code())
	}

	if st.Message() != "boom" {
		t.Fatalf("expected grpc message %q, got %q", "boom", st.Message())
	}
}

func TestRouterGroupUnbind(t *testing.T) {
	router := NewRouter(newTestEvent)

	group := router.Group("/keel.test.TraceService")
	group.Bind(
		&hook.Handler[*testEvent]{
			Id: "skip",
			Func: func(e *testEvent) error {
				*e.trace = append(*e.trace, "skip")
				return e.Next()
			},
		},
		&hook.Handler[*testEvent]{
			Id: "keep",
			Func: func(e *testEvent) error {
				*e.trace = append(*e.trace, "keep")
				return e.Next()
			},
		},
	)

	route := group.Route("Trace", func(e *testEvent) error {
		return e.JSON(*e.trace)
	})
	route.Unbind("skip")

	server, err := router.BuildServer()
	if err != nil {
		t.Fatalf("BuildServer() error = %v", err)
	}

	conn := newBufconnClient(t, server)

	var resp json.RawMessage
	if err := conn.Invoke(context.Background(), "/keel.test.TraceService/Trace", json.RawMessage(`{}`), &resp); err != nil {
		t.Fatalf("invoke grpc method: %v", err)
	}

	var trace []string
	if err := json.Unmarshal(resp, &trace); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	expected := []string{"keep"}
	if !reflect.DeepEqual(expected, trace) {
		t.Fatalf("expected trace %v, got %v", expected, trace)
	}
}

func TestRouterHasRoute(t *testing.T) {
	router := NewRouter(newTestEvent)
	router.Group("/keel.test.EchoService").Route("Echo", func(e *testEvent) error {
		return nil
	})

	if !router.HasRoute("/keel.test.EchoService/Echo") {
		t.Fatalf("expected route to be found")
	}
}

func TestCleanupCalled(t *testing.T) {
	called := false

	router := NewRouter(func(ctx context.Context, req []byte, info MethodInfo) (*Event, EventCleanupFunc) {
		return &Event{
				Context: ctx,
				Method:  info,
				request: append([]byte(nil), req...),
			}, func() {
				called = true
			}
	})
	router.Group("/keel.test.CleanupService").Route("Run", func(e *Event) error {
		return e.JSON(map[string]bool{"ok": true})
	})

	server, err := router.BuildServer()
	if err != nil {
		t.Fatalf("BuildServer() error = %v", err)
	}

	conn := newBufconnClient(t, server)

	var resp json.RawMessage
	if err := conn.Invoke(context.Background(), "/keel.test.CleanupService/Run", json.RawMessage(`{}`), &resp); err != nil {
		t.Fatalf("invoke grpc method: %v", err)
	}

	if !called {
		t.Fatalf("expected cleanup function to be called")
	}
}
