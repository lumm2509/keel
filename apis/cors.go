package apis

import (
	"net/http"
	"strings"
)

// preflightMaxAge is the value of Access-Control-Max-Age sent on OPTIONS
// responses. Browsers cache preflight results for this many seconds, reducing
// the number of extra round-trips for repeated cross-origin requests.
const preflightMaxAge = "3600"

// defaultAllowedHeaders is the fallback when the client does not send an
// Access-Control-Request-Headers header.
const defaultAllowedHeaders = "Content-Type, Authorization, X-Requested-With, X-Request-ID"

// CORS wraps next with a Cross-Origin Resource Sharing middleware.
//
// allowedOrigins controls which origins are permitted:
//   - nil or empty slice → allow all origins ("*")
//   - ["*"]             → allow all origins ("*")
//   - specific origins  → only those exact origins; credentials are supported
//     because the response echoes back the matched origin instead of "*"
//
// Preflight (OPTIONS) requests are handled and terminated here; the actual
// Access-Control-Request-Headers value is reflected back so that any custom
// header the client declares is automatically allowed.
func CORS(next http.Handler, allowedOrigins []string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := strings.TrimSpace(r.Header.Get("Origin"))
		if origin != "" {
			if allowedOrigin, ok := matchAllowedOrigin(origin, allowedOrigins); ok {
				header := w.Header()
				header.Add("Vary", "Origin")
				header.Add("Vary", "Access-Control-Request-Method")
				header.Add("Vary", "Access-Control-Request-Headers")
				header.Set("Access-Control-Allow-Origin", allowedOrigin)
				header.Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, HEAD, OPTIONS")

				// Reflect the client's requested headers so that any custom
				// header is automatically permitted without hard-coding a list.
				if requested := r.Header.Get("Access-Control-Request-Headers"); requested != "" {
					header.Set("Access-Control-Allow-Headers", requested)
				} else {
					header.Set("Access-Control-Allow-Headers", defaultAllowedHeaders)
				}

				// When a specific origin is matched (not wildcard), credentials
				// (cookies, Authorization header) work because the browser sees
				// its own origin echoed back instead of "*".
				if allowedOrigin != "*" {
					header.Set("Access-Control-Allow-Credentials", "true")
				}

				if r.Method == http.MethodOptions {
					// Cache the preflight result to avoid redundant round-trips.
					header.Set("Access-Control-Max-Age", preflightMaxAge)
					w.WriteHeader(http.StatusNoContent)
					return
				}
			} else if r.Method == http.MethodOptions {
				// Disallowed-origin preflight: short-circuit without CORS headers.
				// The browser will reject the response; next must not run.
				w.WriteHeader(http.StatusNoContent)
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}

func matchAllowedOrigin(origin string, allowedOrigins []string) (string, bool) {
	if len(allowedOrigins) == 0 {
		allowedOrigins = []string{"*"}
	}
	for _, allowed := range allowedOrigins {
		if allowed == "*" {
			return "*", true
		}
		if allowed == origin {
			return origin, true
		}
	}
	return "", false
}
