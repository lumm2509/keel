package keel

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	transporthttp "github.com/lumm2509/keel/transport/http"
)

const requestIDHeader = "X-Request-ID"

// EventKeyRequestID is the EventData key used by Observability to store the request ID.
const EventKeyRequestID = "requestID"

// EventKeyStartTime is the EventData key used by Observability to store the request start time.
const EventKeyStartTime = "startTime"

// EventKeyLogger is the EventData key used by Observability to store the contextual logger.
const EventKeyLogger = "logger"

func Default[T any](options ...Option[T]) *App[T] {
	app := New(options...)
	var logger *slog.Logger
	if app.config != nil && app.config.Logger != nil {
		logger = app.config.Logger
	}
	app.BindFunc(Observability[T](logger))
	return app
}

// Observability returns a middleware that sets a request ID, records start time,
// stores a contextual logger, and logs request completion. All data is stored in
// EventData and can be accessed via c.Get(EventKeyRequestID), c.Get(EventKeyLogger), etc.
func Observability[T any](logger *slog.Logger) HandlerFunc[T] {
	if logger == nil {
		logger = slog.Default()
	}
	return func(c *Context[T]) (err error) {
		start := time.Now().UTC()
		c.Set(EventKeyStartTime, start)

		requestID := c.Request.Header.Get(requestIDHeader)
		if requestID == "" {
			requestID = newRequestID()
		}
		c.Set(EventKeyRequestID, requestID)
		c.Request.Header.Set(requestIDHeader, requestID)
		c.Response.Header().Set(requestIDHeader, requestID)

		routePattern, _ := c.Get(transporthttp.EventKeyRoutePattern).(string)
		contextLogger := logger.With(
			"request_id", requestID,
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"route", routePattern,
		)
		c.Set(EventKeyLogger, contextLogger)

		defer func() {
			routePattern, _ := c.Get(transporthttp.EventKeyRoutePattern).(string)
			duration := time.Since(start)
			attrs := []any{
				"event", "http_request_completed",
				"request_id", requestID,
				"method", c.Request.Method,
				"path", c.Request.URL.Path,
				"route", routePattern,
				"status", statusForLog(c),
				"duration_ms", durationMs(duration),
				"ip", c.ClientIP(),
				"user_agent", c.Request.UserAgent(),
				"bytes_written", c.BytesWritten(),
			}
			if err != nil {
				attrs = append(attrs, "error", err.Error())
			}

			contextLogger.LogAttrs(
				c.Request.Context(),
				completionLevel(statusForLog(c), err),
				"http request completed",
				toAttrs(attrs)...,
			)
		}()

		defer func() {
			if recovered := recover(); recovered != nil {
				panicErr := fmt.Errorf("panic: %v", recovered)
				routePattern, _ := c.Get(transporthttp.EventKeyRoutePattern).(string)
				duration := time.Since(start)
				attrs := []any{
					"event", "http_request_panic",
					"request_id", requestID,
					"method", c.Request.Method,
					"path", c.Request.URL.Path,
					"route", routePattern,
					"status", http.StatusInternalServerError,
					"duration_ms", durationMs(duration),
					"ip", c.ClientIP(),
					"user_agent", c.Request.UserAgent(),
					"bytes_written", c.BytesWritten(),
					"error", panicErr.Error(),
					"stack_trace", string(debug.Stack()),
				}

				contextLogger.LogAttrs(
					c.Request.Context(),
					slog.LevelError,
					"http request panic",
					toAttrs(attrs)...,
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

func durationMs(duration time.Duration) float64 {
	return float64(duration) / float64(time.Millisecond)
}

func statusForLog[T any](c *Context[T]) int {
	if status := c.Status(); status > 0 {
		return status
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

func toAttrs(args []any) []slog.Attr {
	attrs := make([]slog.Attr, 0, len(args)/2)
	for i := 0; i+1 < len(args); i += 2 {
		key, ok := args[i].(string)
		if !ok {
			continue
		}
		attrs = append(attrs, slog.Any(key, args[i+1]))
	}

	return attrs
}
