// Package observability provides a request logging and panic recovery middleware
// for keel applications. It is an optional tool — import it only if you need it.
package observability

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/lumm2509/keel"
	transporthttp "github.com/lumm2509/keel/transport/http"
)

const requestIDHeader = "X-Request-ID"

var (
	RequestIDKey = keel.NewContextKey[string]()
	StartTimeKey = keel.NewContextKey[time.Time]()
	LoggerKey    = keel.NewContextKey[*slog.Logger]()
)

func Middleware[T any](logger *slog.Logger) func(*transporthttp.RequestEvent[T]) error {
	if logger == nil {
		logger = slog.Default()
	}
	return func(c *transporthttp.RequestEvent[T]) (err error) {
		start := time.Now().UTC()
		StartTimeKey.Set(c, start)

		requestID := c.Request.Header.Get(requestIDHeader)
		if requestID == "" {
			requestID = newRequestID()
		}
		RequestIDKey.Set(c, requestID)
		c.Request.Header.Set(requestIDHeader, requestID)
		c.Response.Header().Set(requestIDHeader, requestID)

		routePattern, _ := c.Get(transporthttp.EventKeyRoutePattern).(string)
		contextLogger := logger.With(
			"request_id", requestID,
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"route", routePattern,
		)
		LoggerKey.Set(c, contextLogger)

		defer func() {
			routePattern, _ := c.Get(transporthttp.EventKeyRoutePattern).(string)
			duration := time.Since(start)
			attrs := []slog.Attr{
				slog.String("event", "http_request_completed"),
				slog.String("request_id", requestID),
				slog.String("method", c.Request.Method),
				slog.String("path", c.Request.URL.Path),
				slog.String("route", routePattern),
				slog.Int("status", statusForLog(c)),
				slog.Float64("duration_ms", durationMs(duration)),
				slog.String("ip", c.ClientIP()),
				slog.String("user_agent", c.Request.UserAgent()),
				slog.Int("bytes_written", c.BytesWritten()),
			}
			if err != nil {
				attrs = append(attrs, slog.String("error", err.Error()))
			}

			contextLogger.LogAttrs(
				c.Request.Context(),
				completionLevel(statusForLog(c), err),
				"http request completed",
				attrs...,
			)
		}()

		defer func() {
			if recovered := recover(); recovered != nil {
				panicErr := fmt.Errorf("panic: %v", recovered)
				routePattern, _ := c.Get(transporthttp.EventKeyRoutePattern).(string)
				duration := time.Since(start)
				attrs := []slog.Attr{
					slog.String("event", "http_request_panic"),
					slog.String("request_id", requestID),
					slog.String("method", c.Request.Method),
					slog.String("path", c.Request.URL.Path),
					slog.String("route", routePattern),
					slog.Int("status", http.StatusInternalServerError),
					slog.Float64("duration_ms", durationMs(duration)),
					slog.String("ip", c.ClientIP()),
					slog.String("user_agent", c.Request.UserAgent()),
					slog.Int("bytes_written", c.BytesWritten()),
					slog.String("error", panicErr.Error()),
					slog.String("stack_trace", string(debug.Stack())),
				}

				contextLogger.LogAttrs(
					c.Request.Context(),
					slog.LevelError,
					"http request panic",
					attrs...,
				)

				if !c.Written() {
					transporthttp.ErrorHandler(
						c.Response,
						c.Request,
						transporthttp.NewInternalServerError("Internal Server Error", nil),
					)
				}

				err = panicErr
			}
		}()

		return c.Next()
	}
}

func newRequestID() string {
	var buf [12]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return fmt.Sprintf("%d", time.Now().UTC().UnixNano())
	}
	return hex.EncodeToString(buf[:])
}

func durationMs(d time.Duration) float64 {
	return float64(d) / float64(time.Millisecond)
}

func statusForLog[T any](c *transporthttp.RequestEvent[T]) int {
	if s := c.Status(); s > 0 {
		return s
	}
	if c.Written() {
		return http.StatusOK
	}
	return 0
}

func completionLevel(status int, err error) slog.Level {
	if err != nil || status >= http.StatusInternalServerError {
		return slog.LevelError
	}
	if status >= http.StatusBadRequest {
		return slog.LevelWarn
	}
	return slog.LevelInfo
}
