package apis

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/lumm2509/keel/config"
)

func TestBuildCertManagerWithoutConfigReturnsNil(t *testing.T) {
	t.Parallel()

	manager, err := CertManager(nil, "", nil)
	if err != nil {
		t.Fatalf("CertManager() error = %v", err)
	}

	if manager != nil {
		t.Fatalf("expected nil cert manager, got %#v", manager)
	}
}

func TestBuildCertManagerFailsWhenAutoCertCacheDirHasNoDataDir(t *testing.T) {
	t.Parallel()

	cacheDir := "autocert"
	cfg := &config.Config{
		Http: &config.HttpConfig{
			AutoCert: &config.AutoCertConfig{
				CacheDir: &cacheDir,
			},
		},
	}

	_, err := CertManager(cfg, "", nil)
	if err == nil {
		t.Fatalf("expected CertManager() to fail when cache dir is configured without data dir")
	}

	if err.Error() != "autocert cache dir requires a data dir to be set" {
		t.Fatalf("unexpected error: %v", err)
	}
}


func TestWrapCORSAllowedOrigin(t *testing.T) {
	t.Parallel()

	handler := CORS(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}), []string{"https://app.example"})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "https://app.example")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://app.example" {
		t.Fatalf("expected allow origin header, got %q", got)
	}
}

func TestWrapCORSOptionsDisallowedOrigin(t *testing.T) {
	t.Parallel()

	handler := CORS(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler should not run for OPTIONS preflight")
	}), []string{"https://app.example"})

	req := httptest.NewRequest(http.MethodOptions, "/", nil)
	req.Header.Set("Origin", "https://other.example")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("expected no allow origin header, got %q", got)
	}
}

func TestCORSWildcardAllowsAllOrigins(t *testing.T) {
	t.Parallel()

	for _, allowedOrigins := range [][]string{nil, {"*"}} {
		handler := CORS(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}), allowedOrigins)

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Origin", "https://any.example")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "*" {
			t.Fatalf("allowedOrigins=%v: expected Access-Control-Allow-Origin=*, got %q", allowedOrigins, got)
		}
		if got := rec.Header().Get("Access-Control-Allow-Credentials"); got != "" {
			t.Fatalf("allowedOrigins=%v: expected no credentials header for wildcard, got %q", allowedOrigins, got)
		}
	}
}

func TestCORSNoOriginHeaderPassesThrough(t *testing.T) {
	t.Parallel()

	nextCalled := false
	handler := CORS(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	}), []string{"https://app.example"})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !nextCalled {
		t.Fatal("next handler must be called when there is no Origin header")
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("expected no CORS headers without Origin, got %q", got)
	}
}

func TestCORSPreflightAllowedOriginSetsCredentialsAndMaxAge(t *testing.T) {
	t.Parallel()

	handler := CORS(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next must not run for preflight")
	}), []string{"https://app.example"})

	req := httptest.NewRequest(http.MethodOptions, "/", nil)
	req.Header.Set("Origin", "https://app.example")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Fatalf("expected Allow-Credentials=true for specific origin, got %q", got)
	}
	if got := rec.Header().Get("Access-Control-Max-Age"); got != preflightMaxAge {
		t.Fatalf("expected Max-Age=%s, got %q", preflightMaxAge, got)
	}
}

func TestCORSPreflightReflectsRequestedHeaders(t *testing.T) {
	t.Parallel()

	handler := CORS(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next must not run for preflight")
	}), []string{"https://app.example"})

	req := httptest.NewRequest(http.MethodOptions, "/", nil)
	req.Header.Set("Origin", "https://app.example")
	req.Header.Set("Access-Control-Request-Headers", "X-Custom-Header, Authorization")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Headers"); got != "X-Custom-Header, Authorization" {
		t.Fatalf("expected reflected headers, got %q", got)
	}
}

func TestCORSPreflightFallsBackToDefaultHeaders(t *testing.T) {
	t.Parallel()

	handler := CORS(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next must not run for preflight")
	}), []string{"https://app.example"})

	req := httptest.NewRequest(http.MethodOptions, "/", nil)
	req.Header.Set("Origin", "https://app.example")
	// No Access-Control-Request-Headers set.
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Headers"); got != defaultAllowedHeaders {
		t.Fatalf("expected default headers %q, got %q", defaultAllowedHeaders, got)
	}
}

func TestCORSDisallowedOriginGetPassesThrough(t *testing.T) {
	t.Parallel()

	nextCalled := false
	handler := CORS(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	}), []string{"https://app.example"})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "https://other.example")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !nextCalled {
		t.Fatal("next must run for non-preflight requests even from disallowed origins")
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("expected no CORS headers for disallowed origin, got %q", got)
	}
}

func TestCORSVaryHeaderAlwaysSet(t *testing.T) {
	t.Parallel()

	handler := CORS(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}), []string{"https://app.example"})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "https://app.example")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	vary := rec.Header().Values("Vary")
	if len(vary) < 3 {
		t.Fatalf("expected at least 3 Vary values, got %v", vary)
	}
}

func TestBaseURL(t *testing.T) {
	t.Parallel()

	scenarios := []struct {
		cfg      ServeConfig
		addr     string
		expected string
	}{
		{ServeConfig{}, "", "http://127.0.0.1"},
		{ServeConfig{}, ":8080", "http://:8080"},
		{ServeConfig{}, "localhost:9000", "http://localhost:9000"},
		{ServeConfig{HttpsAddr: ":443"}, ":443", "https://:443"},
		{ServeConfig{HttpsAddr: ":https"}, ":https", "https://127.0.0.1"},
		{ServeConfig{HttpsAddr: ":443", CertificateDomains: []string{"example.com", "www.example.com"}}, ":443", "https://example.com"},
	}

	for _, s := range scenarios {
		got := BaseURL(s.cfg, s.addr)
		if got != s.expected {
			t.Errorf("BaseURL(%v, %q) = %q, want %q", s.cfg, s.addr, got, s.expected)
		}
	}
}
