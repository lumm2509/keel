package http

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRequestEventReleaseClearsCachedRequestInfo(t *testing.T) {
	t.Parallel()

	newRequest := func(target, body string) *http.Request {
		req := httptest.NewRequest(http.MethodPost, target, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Body = &RereadableReadCloser{
			ReadCloser: req.Body,
			Lazy:       true,
		}
		return req
	}

	event := &RequestEvent[struct{}]{}
	event.Reset(nil, httptest.NewRecorder(), newRequest("/first?foo=bar", `{"alpha":1}`))

	info, err := event.RequestInfo()
	if err != nil {
		t.Fatalf("first RequestInfo failed: %v", err)
	}
	if info.Query["foo"] != "bar" {
		t.Fatalf("expected first query to be cached, got %v", info.Query)
	}
	if got, ok := info.Body["alpha"]; !ok || got.(float64) != 1 {
		t.Fatalf("expected first body to be cached, got %v", info.Body)
	}

	event.Release()
	event.Reset(nil, httptest.NewRecorder(), newRequest("/second", `{}`))

	info, err = event.RequestInfo()
	if err != nil {
		t.Fatalf("second RequestInfo failed: %v", err)
	}
	if len(info.Query) != 0 {
		t.Fatalf("expected query map to be cleared, got %v", info.Query)
	}
	if _, ok := info.Body["alpha"]; ok {
		t.Fatalf("expected body map to be cleared, got %v", info.Body)
	}
	if info.Method != http.MethodPost {
		t.Fatalf("expected method %q, got %q", http.MethodPost, info.Method)
	}
}

func TestRequestEventRequestInfoPreservesBodyForSubsequentBindBody(t *testing.T) {
	t.Parallel()

	body := `{"name":"test","value":42}`

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Body = &RereadableReadCloser{
		ReadCloser: req.Body,
		Lazy:       true,
	}

	event := &RequestEvent[struct{}]{}
	event.Reset(nil, httptest.NewRecorder(), req)

	// Simulate a middleware calling RequestInfo().
	info, err := event.RequestInfo()
	if err != nil {
		t.Fatalf("RequestInfo failed: %v", err)
	}
	if got, ok := info.Body["name"]; !ok || got.(string) != "test" {
		t.Fatalf("expected body[name]=%q from RequestInfo, got %v", "test", info.Body)
	}

	// The handler must still be able to read the body after RequestInfo consumed it.
	var dst struct {
		Name  string  `json:"name"`
		Value float64 `json:"value"`
	}
	if err := event.BindBody(&dst); err != nil {
		t.Fatalf("BindBody after RequestInfo failed: %v", err)
	}
	if dst.Name != "test" || dst.Value != 42 {
		t.Fatalf("expected {name:test value:42}, got %+v", dst)
	}
}

func BenchmarkRequestEventRequestInfo(b *testing.B) {
	newEvent := func() *RequestEvent[struct{}] {
		req := httptest.NewRequest(http.MethodPost, "/api/items?foo=bar&baz=qux", strings.NewReader(`{"alpha":1,"beta":"test"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Test-Header", "value")
		req.Body = &RereadableReadCloser{
			ReadCloser: req.Body,
			Lazy:       true,
		}

		event := &RequestEvent[struct{}]{}
		event.Reset(nil, httptest.NewRecorder(), req)
		return event
	}

	b.Run("current", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			event := newEvent()
			if _, err := event.RequestInfo(); err != nil {
				b.Fatal(err)
			}
			event.Release()
		}
	})
}
