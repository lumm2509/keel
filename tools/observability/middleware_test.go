package observability_test

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"slices"
	"sync"
	"testing"

	"github.com/lumm2509/keel"
	"github.com/lumm2509/keel/tools/observability"
	transporthttp "github.com/lumm2509/keel/transport/http"
)

type capturedRecord struct {
	level slog.Level
	msg   string
	attrs map[string]any
}

type captureHandler struct {
	state *captureState
	attrs []slog.Attr
}

type captureState struct {
	mu      sync.Mutex
	records []capturedRecord
}

func (h *captureHandler) Enabled(context.Context, slog.Level) bool { return true }

func (h *captureHandler) Handle(_ context.Context, record slog.Record) error {
	h.state.mu.Lock()
	defer h.state.mu.Unlock()

	attrs := map[string]any{}
	for _, attr := range h.attrs {
		attrs[attr.Key] = attr.Value.Any()
	}
	record.Attrs(func(attr slog.Attr) bool {
		attrs[attr.Key] = attr.Value.Any()
		return true
	})

	h.state.records = append(h.state.records, capturedRecord{
		level: record.Level,
		msg:   record.Message,
		attrs: attrs,
	})
	return nil
}

func (h *captureHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &captureHandler{state: h.state, attrs: append(slices.Clone(h.attrs), attrs...)}
}

func (h *captureHandler) WithGroup(string) slog.Handler { return h }

// testApp implements TrustedProxyProvider to resolve forwarded IPs from 10.0.0.0/8.
type testApp struct{}

func (*testApp) TrustedProxyRanges() []netip.Prefix {
	prefix, _ := netip.ParsePrefix("10.0.0.0/8")
	return []netip.Prefix{prefix}
}

func newCapture() (*captureHandler, *slog.Logger) {
	h := &captureHandler{state: &captureState{}}
	return h, slog.New(h)
}

func TestMiddlewareCapturesRequestContext(t *testing.T) {
	t.Parallel()

	capture, logger := newCapture()

	app := keel.New[testApp](keel.WithContext(&testApp{}))
	app.BindFunc(observability.Middleware[testApp](logger))

	app.GET("/users/{id}", func(c *keel.Context[testApp]) error {
		requestID, _ := observability.RequestIDKey.Get(c)
		if requestID != "req-123" {
			t.Fatalf("expected request ID to be propagated, got %q", requestID)
		}

		startTime, hasStartTime := observability.StartTimeKey.Get(c)
		if !hasStartTime || startTime.IsZero() {
			t.Fatalf("expected request start time to be set")
		}

		if c.ClientIP() != "203.0.113.10" {
			t.Fatalf("expected forwarded client IP, got %q", c.ClientIP())
		}

		routePattern, _ := c.Get(transporthttp.EventKeyRoutePattern).(string)
		if routePattern != "/users/{id}" {
			t.Fatalf("expected route pattern, got %q", routePattern)
		}

		contextLogger, _ := observability.LoggerKey.Get(c)
		if contextLogger == nil {
			t.Fatal("expected contextual logger to be set")
		}
		contextLogger.Info("handler log")

		return c.JSON(http.StatusCreated, map[string]string{"ok": "true"})
	})

	mux, err := app.Router.BuildMux()
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/users/42", nil)
	req.RemoteAddr = "10.0.0.5:1234"
	req.Header.Set("X-Request-ID", "req-123")
	req.Header.Set("X-Forwarded-For", "203.0.113.10")
	res := httptest.NewRecorder()

	mux.ServeHTTP(res, req)

	if res.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, res.Code)
	}
	if res.Header().Get("X-Request-ID") != "req-123" {
		t.Fatalf("expected response X-Request-ID header to be set")
	}

	completion := findRecord(t, capture.state.records, "http_request_completed")
	if completion.attrs["request_id"] != "req-123" {
		t.Fatalf("expected request_id attr, got %#v", completion.attrs["request_id"])
	}
	if completion.attrs["route"] != "/users/{id}" {
		t.Fatalf("expected route attr, got %#v", completion.attrs["route"])
	}
	if got := intAttr(t, completion.attrs["status"]); got != http.StatusCreated {
		t.Fatalf("expected status attr %d, got %d", http.StatusCreated, got)
	}
	if completion.attrs["bytes_written"] == nil {
		t.Fatalf("expected bytes_written attr")
	}

	handlerLog := findMessage(t, capture.state.records, "handler log")
	if handlerLog.attrs["request_id"] != "req-123" {
		t.Fatalf("expected contextual request_id on handler logger")
	}
	if handlerLog.attrs["route"] != "/users/{id}" {
		t.Fatalf("expected contextual route on handler logger")
	}
}

func TestMiddlewareRecoversPanics(t *testing.T) {
	t.Parallel()

	capture, logger := newCapture()

	app := keel.New[struct{}]()
	app.BindFunc(observability.Middleware[struct{}](logger))
	app.GET("/panic", func(c *keel.Context[struct{}]) error {
		panic("boom")
	})

	mux, err := app.Router.BuildMux()
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	res := httptest.NewRecorder()
	mux.ServeHTTP(res, req)

	if res.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, res.Code)
	}

	panicRecord := findRecord(t, capture.state.records, "http_request_panic")
	if panicRecord.level != slog.LevelError {
		t.Fatalf("expected panic log level error, got %v", panicRecord.level)
	}
	if panicRecord.attrs["stack_trace"] == "" {
		t.Fatalf("expected stack trace to be logged")
	}

	completion := findRecord(t, capture.state.records, "http_request_completed")
	if got := intAttr(t, completion.attrs["status"]); got != http.StatusInternalServerError {
		t.Fatalf("expected completion status %d, got %d", http.StatusInternalServerError, got)
	}
	if completion.attrs["error"] == nil {
		t.Fatalf("expected completion error attr")
	}
}

func findRecord(t *testing.T, records []capturedRecord, event string) capturedRecord {
	t.Helper()
	for _, r := range records {
		if r.attrs["event"] == event {
			return r
		}
	}
	t.Fatalf("missing record for event %q", event)
	return capturedRecord{}
}

func findMessage(t *testing.T, records []capturedRecord, msg string) capturedRecord {
	t.Helper()
	for _, r := range records {
		if r.msg == msg {
			return r
		}
	}
	t.Fatalf("missing record for message %q", msg)
	return capturedRecord{}
}

func intAttr(t *testing.T, value any) int {
	t.Helper()
	switch v := value.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case uint64:
		return int(v)
	default:
		t.Fatalf("unexpected numeric attr type %T (%v)", value, value)
		return 0
	}
}
