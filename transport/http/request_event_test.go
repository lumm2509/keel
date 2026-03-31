package http

import (
	"net/http"
	"net/http/httptest"
	"net/netip"
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

func TestRequestInfoCloneIsDeepCopy(t *testing.T) {
	t.Parallel()

	original := &RequestInfo{
		Method:  "POST",
		Context: "ctx",
		Query:   map[string]string{"q": "v"},
		Headers: map[string]string{"h": "hv"},
		Body:    map[string]any{"b": 1},
	}
	clone := original.Clone()

	if clone == original {
		t.Fatal("Clone() returned the same pointer")
	}
	if clone.Method != original.Method || clone.Context != original.Context {
		t.Fatalf("scalar fields not copied: %+v", clone)
	}

	// Mutating clone must not affect original.
	clone.Query["q"] = "changed"
	if original.Query["q"] != "v" {
		t.Fatal("Clone() Query is not a deep copy")
	}
	clone.Headers["h"] = "changed"
	if original.Headers["h"] != "hv" {
		t.Fatal("Clone() Headers is not a deep copy")
	}
	clone.Body["b"] = 2
	if original.Body["b"] != 1 {
		t.Fatal("Clone() Body is not a deep copy")
	}
}

func TestRequestInfoCloneOfNilIsNil(t *testing.T) {
	t.Parallel()

	var info *RequestInfo
	if info.Clone() != nil {
		t.Fatal("Clone() of nil should return nil")
	}
}

func TestRequestEventParamReturnsPathValue(t *testing.T) {
	t.Parallel()

	// Param() delegates to r.PathValue which is set by the stdlib mux.
	// We can verify the delegation by using a request where PathValue is
	// pre-populated via SetPathValue (Go 1.22+).
	req := httptest.NewRequest(http.MethodGet, "/users/42", nil)
	req.SetPathValue("id", "42")

	e := &RequestEvent[struct{}]{}
	e.Reset(nil, httptest.NewRecorder(), req)

	if got := e.Param("id"); got != "42" {
		t.Fatalf("Param(\"id\") = %q, want %q", got, "42")
	}
}

func TestRequestEventParamMissingReturnsEmpty(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/users/42", nil)
	e := &RequestEvent[struct{}]{}
	e.Reset(nil, httptest.NewRecorder(), req)

	// No path value named "missing" — should return "" without panic.
	if got := e.Param("missing"); got != "" {
		t.Fatalf("Param for unknown name = %q, want empty", got)
	}
}

func TestRequestEventClientIPWithoutProvider(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "1.2.3.4:5678"
	// Spoof an X-Forwarded-For that must be ignored when not a trusted proxy.
	req.Header.Set("X-Forwarded-For", "9.9.9.9")

	e := &RequestEvent[struct{}]{}
	e.Reset(nil, httptest.NewRecorder(), req)

	// App is nil (struct{}), does not implement TrustedProxyProvider.
	if got := e.ClientIP(); got != "1.2.3.4" {
		t.Fatalf("ClientIP() = %q, want %q", got, "1.2.3.4")
	}
}

type trustedProxyApp struct {
	ranges []netip.Prefix
}

func (a *trustedProxyApp) TrustedProxyRanges() []netip.Prefix { return a.ranges }

func TestRequestEventClientIPWithTrustedProxyProvider(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "127.0.0.1:1234"
	req.Header.Set("X-Forwarded-For", "203.0.113.5")

	trusted := mustParsePrefix(t, "127.0.0.0/8")
	app := &trustedProxyApp{ranges: []netip.Prefix{trusted}}

	e := &RequestEvent[trustedProxyApp]{}
	e.Reset(app, httptest.NewRecorder(), req)

	if got := e.ClientIP(); got != "203.0.113.5" {
		t.Fatalf("ClientIP() = %q, want %q via proxy header", got, "203.0.113.5")
	}
}

func TestRequestInfoCachedOnSecondCall(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodPost, "/?x=1", strings.NewReader(`{"k":"v"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Body = &RereadableReadCloser{ReadCloser: req.Body, Lazy: true}

	e := &RequestEvent[struct{}]{}
	e.Reset(nil, httptest.NewRecorder(), req)

	first, err := e.RequestInfo()
	if err != nil {
		t.Fatalf("first RequestInfo: %v", err)
	}
	second, err := e.RequestInfo()
	if err != nil {
		t.Fatalf("second RequestInfo: %v", err)
	}
	if first != second {
		t.Fatal("second call to RequestInfo() should return the cached pointer")
	}
}

func mustParsePrefix(t *testing.T, s string) netip.Prefix {
	t.Helper()
	p, err := netip.ParsePrefix(s)
	if err != nil {
		t.Fatalf("ParsePrefix(%q): %v", s, err)
	}
	return p
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
