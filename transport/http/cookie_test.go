package http

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func newCookieEvent(cookies ...*http.Cookie) (*RequestEvent[struct{}], *httptest.ResponseRecorder) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	e := &RequestEvent[struct{}]{}
	e.Reset(nil, rec, req)
	return e, rec
}

func TestCookieStoreGetFound(t *testing.T) {
	t.Parallel()

	e, _ := newCookieEvent(&http.Cookie{Name: "session", Value: "abc123"})

	val, ok := e.Cookies().Get("session")
	if !ok {
		t.Fatal("expected ok=true")
	}
	if val != "abc123" {
		t.Fatalf("expected %q, got %q", "abc123", val)
	}
}

func TestCookieStoreGetMissing(t *testing.T) {
	t.Parallel()

	e, _ := newCookieEvent()

	val, ok := e.Cookies().Get("missing")
	if ok {
		t.Fatal("expected ok=false")
	}
	if val != "" {
		t.Fatalf("expected empty string, got %q", val)
	}
}

func TestCookieStoreAll(t *testing.T) {
	t.Parallel()

	e, _ := newCookieEvent(
		&http.Cookie{Name: "a", Value: "1"},
		&http.Cookie{Name: "b", Value: "2"},
	)

	all := e.Cookies().All()
	if len(all) != 2 {
		t.Fatalf("expected 2 cookies, got %d", len(all))
	}
}

func TestCookieStoreSet(t *testing.T) {
	t.Parallel()

	e, rec := newCookieEvent()
	e.Cookies().Set("token", "xyz")

	header := rec.Header().Get("Set-Cookie")
	if !strings.Contains(header, "token=xyz") {
		t.Fatalf("expected Set-Cookie header with token=xyz, got %q", header)
	}
}

func TestCookieStoreSetWithOptions(t *testing.T) {
	t.Parallel()

	e, rec := newCookieEvent()
	e.Cookies().Set("session", "s1",
		WithHTTPOnly(),
		WithSecure(),
		WithPath("/api"),
		WithMaxAge(3600),
		WithSameSite(http.SameSiteStrictMode),
	)

	header := rec.Header().Get("Set-Cookie")
	for _, want := range []string{"session=s1", "HttpOnly", "Secure", "Path=/api", "Max-Age=3600", "SameSite=Strict"} {
		if !strings.Contains(header, want) {
			t.Fatalf("expected %q in Set-Cookie header, got %q", want, header)
		}
	}
}

func TestCookieStoreSetWithExpires(t *testing.T) {
	t.Parallel()

	e, rec := newCookieEvent()
	future := time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC)
	e.Cookies().Set("exp", "v", WithExpires(future))

	header := rec.Header().Get("Set-Cookie")
	if !strings.Contains(header, "2099") {
		t.Fatalf("expected 2099 in Expires, got %q", header)
	}
}

func TestCookieStoreSetWithDomain(t *testing.T) {
	t.Parallel()

	e, rec := newCookieEvent()
	e.Cookies().Set("d", "v", WithDomain("example.com"))

	header := rec.Header().Get("Set-Cookie")
	if !strings.Contains(header, "Domain=example.com") {
		t.Fatalf("expected Domain=example.com, got %q", header)
	}
}

func TestCookieStoreDelete(t *testing.T) {
	t.Parallel()

	e, rec := newCookieEvent(&http.Cookie{Name: "session", Value: "old"})
	e.Cookies().Delete("session", WithPath("/"))

	header := rec.Header().Get("Set-Cookie")
	if !strings.Contains(header, "session=") {
		t.Fatalf("expected Set-Cookie for session, got %q", header)
	}
	if !strings.Contains(header, "Max-Age=0") {
		t.Fatalf("expected Max-Age=0 (delete), got %q", header)
	}
}
