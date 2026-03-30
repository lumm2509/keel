package keel

import (
	"net/http"
	"time"

	transporthttp "github.com/lumm2509/keel/transport/http"
)

// CookieSetOption is a functional option applied when writing a cookie.
type CookieSetOption = transporthttp.CookieSetOption

// WithMaxAge sets the cookie's Max-Age in seconds. Use -1 to delete, 0 for session-only.
func WithMaxAge(seconds int) CookieSetOption { return transporthttp.WithMaxAge(seconds) }

// WithExpires sets an explicit expiry time on the cookie.
func WithExpires(t time.Time) CookieSetOption { return transporthttp.WithExpires(t) }

// WithPath sets the cookie's Path attribute.
func WithPath(path string) CookieSetOption { return transporthttp.WithPath(path) }

// WithDomain sets the cookie's Domain attribute.
func WithDomain(domain string) CookieSetOption { return transporthttp.WithDomain(domain) }

// WithSecure marks the cookie as Secure (HTTPS-only).
func WithSecure() CookieSetOption { return transporthttp.WithSecure() }

// WithHTTPOnly marks the cookie as HttpOnly (not accessible via JavaScript).
func WithHTTPOnly() CookieSetOption { return transporthttp.WithHTTPOnly() }

// WithSameSite sets the cookie's SameSite policy.
func WithSameSite(mode http.SameSite) CookieSetOption { return transporthttp.WithSameSite(mode) }
