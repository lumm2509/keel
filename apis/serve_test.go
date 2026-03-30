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
	cfg := &config.ConfigModule{
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
