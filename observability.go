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

func Default[Cradle any](options ...Option[Cradle]) *App[Cradle] {
	app := New(options...)
	app.BindFunc(Observability[Cradle]())
	return app
}

func Observability[Cradle any]() HandlerFunc[Cradle] {
	return func(c *Context[Cradle]) (err error) {
		start := time.Now().UTC()
		c.SetStartTime(start)

		requestID := c.Request.Header.Get(requestIDHeader)
		if requestID == "" {
			requestID = newRequestID()
		}
		c.SetRequestID(requestID)
		c.SetLogger(c.Container.Logger())

		defer func() {
			attrs := []any{
				"event", "http_request_completed",
				"request_id", c.RequestID(),
				"method", c.Request.Method,
				"path", c.Request.URL.Path,
				"route", c.RoutePattern(),
				"status", statusForLog(c),
				"duration_ms", durationMs(c.Duration()),
				"ip", c.ClientIP(),
				"user_agent", c.Request.UserAgent(),
				"bytes_written", c.BytesWritten(),
			}
			if err != nil {
				attrs = append(attrs, "error", err.Error())
			}

			c.Logger().LogAttrs(
				c.Request.Context(),
				completionLevel(statusForLog(c), err),
				"http request completed",
				toAttrs(attrs)...,
			)
		}()

		defer func() {
			if recovered := recover(); recovered != nil {
				panicErr := fmt.Errorf("panic: %v", recovered)
				attrs := []any{
					"event", "http_request_panic",
					"request_id", c.RequestID(),
					"method", c.Request.Method,
					"path", c.Request.URL.Path,
					"route", c.RoutePattern(),
					"status", http.StatusInternalServerError,
					"duration_ms", durationMs(c.Duration()),
					"ip", c.ClientIP(),
					"user_agent", c.Request.UserAgent(),
					"bytes_written", c.BytesWritten(),
					"error", panicErr.Error(),
					"stack_trace", string(debug.Stack()),
				}

				c.Logger().LogAttrs(
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

func statusForLog[Cradle any](c *Context[Cradle]) int {
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
