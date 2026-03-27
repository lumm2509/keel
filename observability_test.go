package keel

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync"
	"testing"

	"github.com/lumm2509/keel/config"
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

func (h *captureHandler) Enabled(context.Context, slog.Level) bool {
	return true
}

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

func (h *captureHandler) WithGroup(string) slog.Handler {
	return h
}

func TestDefaultObservabilityCapturesRequestContext(t *testing.T) {
	t.Parallel()

	handler := &captureHandler{}
	handler.state = &captureState{}
	logger := slog.New(handler)
	cfg := newTestConfig(logger)

	app := Default[struct{}](WithConfig[struct{}](cfg))

	app.Get("/users/{id}", func(c *Context[struct{}]) error {
		if c.RequestID() != "req-123" {
			t.Fatalf("expected request ID to be propagated")
		}
		if c.StartTime().IsZero() {
			t.Fatalf("expected request start time to be set")
		}
		if c.ClientIP() != "203.0.113.10" {
			t.Fatalf("expected forwarded client IP, got %q", c.ClientIP())
		}
		if c.RoutePattern() != "/users/{id}" {
			t.Fatalf("expected route pattern, got %q", c.RoutePattern())
		}

		c.Logger().Info("handler log")
		return c.JSON(http.StatusCreated, map[string]string{"ok": "true"})
	})

	mux, err := app.router.BuildMux()
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/users/42", nil)
	req.Header.Set(requestIDHeader, "req-123")
	req.Header.Set("X-Forwarded-For", "203.0.113.10")
	res := httptest.NewRecorder()

	mux.ServeHTTP(res, req)

	if res.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, res.Code)
	}
	if res.Header().Get(requestIDHeader) != "req-123" {
		t.Fatalf("expected response request ID header to be set")
	}

	completion := findRecord(t, handler.state.records, "http_request_completed")
	if completion.attrs["request_id"] != "req-123" {
		t.Fatalf("expected request_id attr, got %#v", completion.attrs["request_id"])
	}
	if completion.attrs["route"] != "/users/{id}" {
		t.Fatalf("expected route attr, got %#v", completion.attrs["route"])
	}
	if got := intAttr(t, completion.attrs["status"]); got != http.StatusCreated {
		t.Fatalf("expected status attr, got %#v", completion.attrs["status"])
	}
	if completion.attrs["bytes_written"] == nil {
		t.Fatalf("expected bytes_written attr")
	}

	handlerLog := findMessage(t, handler.state.records, "handler log")
	if handlerLog.attrs["request_id"] != "req-123" {
		t.Fatalf("expected contextual request_id on handler logger")
	}
	if handlerLog.attrs["route"] != "/users/{id}" {
		t.Fatalf("expected contextual route on handler logger")
	}
}

func TestDefaultObservabilityRecoversPanics(t *testing.T) {
	t.Parallel()

	handler := &captureHandler{}
	handler.state = &captureState{}
	logger := slog.New(handler)
	cfg := newTestConfig(logger)

	app := Default[struct{}](WithConfig[struct{}](cfg))
	app.Get("/panic", func(c *Context[struct{}]) error {
		panic("boom")
	})

	mux, err := app.router.BuildMux()
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	res := httptest.NewRecorder()

	mux.ServeHTTP(res, req)

	if res.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, res.Code)
	}

	panicRecord := findRecord(t, handler.state.records, "http_request_panic")
	if panicRecord.level != slog.LevelError {
		t.Fatalf("expected panic log level error, got %v", panicRecord.level)
	}
	if panicRecord.attrs["stack_trace"] == "" {
		t.Fatalf("expected stack trace to be logged")
	}

	completion := findRecord(t, handler.state.records, "http_request_completed")
	if got := intAttr(t, completion.attrs["status"]); got != http.StatusInternalServerError {
		t.Fatalf("expected completion status attr, got %#v", completion.attrs["status"])
	}
	if completion.attrs["error"] == nil {
		t.Fatalf("expected completion error attr")
	}
}

func newTestConfig(logger *slog.Logger) *config.ConfigModule {
	return &config.ConfigModule{
		Projectconfig: config.ProjectConfigOptions{},
		Logger:        logger,
	}
}

func findRecord(t *testing.T, records []capturedRecord, event string) capturedRecord {
	t.Helper()

	for _, record := range records {
		if record.attrs["event"] == event {
			return record
		}
	}

	t.Fatalf("missing record for event %q", event)
	return capturedRecord{}
}

func findMessage(t *testing.T, records []capturedRecord, msg string) capturedRecord {
	t.Helper()

	for _, record := range records {
		if record.msg == msg {
			return record
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
