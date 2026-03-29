package apis

import (
	"net/http"
	"strings"
)

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
				header.Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With, X-Request-ID")
			}
		}

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
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
