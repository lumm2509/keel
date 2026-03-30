package http

import (
	"net/http"
	"time"
)

type CookieStore struct {
	r *http.Request
	w http.ResponseWriter
}

// Cookies returns a CookieStore bound to the current request and response.
func (e *Event) Cookies() *CookieStore {
	return &CookieStore{r: e.Request, w: e.Response}
}

func (s *CookieStore) Get(name string) (string, bool) {
	c, err := s.r.Cookie(name)
	if err != nil {
		return "", false
	}
	return c.Value, true
}

func (s *CookieStore) Set(name, value string, opts ...CookieSetOption) {
	c := &http.Cookie{Name: name, Value: value}
	for _, opt := range opts {
		opt(c)
	}
	http.SetCookie(s.w, c)
}

func (s *CookieStore) Delete(name string, opts ...CookieSetOption) {
	c := &http.Cookie{
		Name:   name,
		Value:  "",
		MaxAge: -1,
	}
	for _, opt := range opts {
		opt(c)
	}
	http.SetCookie(s.w, c)
}

// All returns all cookies from the current request as a slice.
func (s *CookieStore) All() []*http.Cookie {
	return s.r.Cookies()
}

// CookieSetOption is a functional option applied when writing a cookie.
type CookieSetOption func(*http.Cookie)

// WithMaxAge sets the cookie's Max-Age in seconds.
// Use -1 to delete, 0 for session-only.
func WithMaxAge(seconds int) CookieSetOption {
	return func(c *http.Cookie) { c.MaxAge = seconds }
}

// WithExpires sets an explicit expiry time on the cookie.
func WithExpires(t time.Time) CookieSetOption {
	return func(c *http.Cookie) { c.Expires = t }
}

// WithPath sets the cookie's Path attribute.
func WithPath(path string) CookieSetOption {
	return func(c *http.Cookie) { c.Path = path }
}

// WithDomain sets the cookie's Domain attribute.
func WithDomain(domain string) CookieSetOption {
	return func(c *http.Cookie) { c.Domain = domain }
}

// WithSecure marks the cookie as Secure (HTTPS-only).
func WithSecure() CookieSetOption {
	return func(c *http.Cookie) { c.Secure = true }
}

// WithHTTPOnly marks the cookie as HttpOnly (not accessible via JavaScript).
func WithHTTPOnly() CookieSetOption {
	return func(c *http.Cookie) { c.HttpOnly = true }
}

// WithSameSite sets the cookie's SameSite policy.
func WithSameSite(mode http.SameSite) CookieSetOption {
	return func(c *http.Cookie) { c.SameSite = mode }
}
