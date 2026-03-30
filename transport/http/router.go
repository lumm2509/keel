package http

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"

	"github.com/lumm2509/keel/runtime/hook"
)

// EventKeyRoutePattern is the EventData key used by the router to expose the matched route path.
const EventKeyRoutePattern = "routePattern"

// EventKeyRouteHandle is the EventData key used by the router to expose the route's Handle metadata.
const EventKeyRouteHandle = "routeHandle"

type eventDataSetter interface {
	Set(key string, value any)
}

var responseWriterPool = sync.Pool{
	New: func() any {
		return &ResponseWriter{}
	},
}

var rereadableBodyPool = sync.Pool{
	New: func() any {
		return &RereadableReadCloser{Lazy: true}
	},
}

type EventCleanupFunc func()

// EventFactoryFunc defines the function responsible for creating a Route specific event
// based on the provided request handler ServeHTTP data.
//
// Optionally return a clean up function that will be invoked right after the route execution.
type EventFactoryFunc[T hook.Resolver] func(w http.ResponseWriter, r *http.Request) (T, EventCleanupFunc)

// Router defines a thin wrapper around the standard Go [http.ServeMux] by
// adding support for routing sub-groups, middlewares and other common utils.
//
// Example:
//
//	r := NewRouter[*MyEvent](eventFactory)
//
//	// middlewares
//	r.BindFunc(m1, m2)
//
//	// routes
//	r.GET("/test", handler1)
//
//	// sub-routers/groups
//	api := r.Group("/api")
//	api.GET("/admins", handler2)
//
//	// generate a http.ServeMux instance based on the router configurations
//	mux, _ := r.BuildMux()
//
//	http.ListenAndServe("localhost:8090", mux)
type Router[T hook.Resolver] struct {
	*RouterGroup[T]

	eventFactory EventFactoryFunc[T]
	mu           sync.RWMutex
	cachedMux    http.Handler
}

// NewRouter creates a new Router instance with the provided event factory function.
func NewRouter[T hook.Resolver](eventFactory EventFactoryFunc[T]) *Router[T] {
	return &Router[T]{
		RouterGroup:  &RouterGroup[T]{},
		eventFactory: eventFactory,
	}
}

func (r *Router[T]) BuildMux() (http.Handler, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	mux, err := r.buildMuxLocked()
	if err != nil {
		return nil, err
	}

	r.cachedMux = mux
	return mux, nil
}

func (r *Router[T]) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		r.mu.RLock()
		mux := r.cachedMux
		r.mu.RUnlock()

		if mux == nil {
			r.mu.Lock()
			if r.cachedMux == nil {
				built, err := r.buildMuxLocked()
				if err != nil {
					r.mu.Unlock()
					http.Error(w, "router: build error: "+err.Error(), http.StatusInternalServerError)
					return
				}
				r.cachedMux = built
			}
			mux = r.cachedMux
			r.mu.Unlock()
		}

		mux.ServeHTTP(w, req)
	})
}

func (r *Router[T]) Patch(fn func(*Router[T])) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	fn(r)

	mux, err := r.buildMuxLocked()
	if err != nil {
		return err
	}

	r.cachedMux = mux
	return nil
}

// buildMuxLocked is the internal builder. Caller must hold r.mu write lock.
func (r *Router[T]) buildMuxLocked() (*http.ServeMux, error) {
	// Note that some of the default std Go handlers like the [http.NotFoundHandler]
	// cannot be currently extended and requires defining a custom "catch-all" route
	// so that the group middlewares could be executed.
	//
	// https://github.com/golang/go/issues/65648
	if !r.HasRoute("", "/") {
		r.Route("", "/", func(e T) error {
			return NewNotFoundError("", nil)
		})
	}

	mux := http.NewServeMux()

	if err := r.loadMux(mux, r.RouterGroup, routeBuildState[T]{}); err != nil {
		return nil, err
	}

	return mux, nil
}

type routeBuildState[T hook.Resolver] struct {
	prefix       string
	middlewares  []*hook.Handler[T]
	errorHandler func(T, error) error // nearest ancestor error boundary; nil = none
}

func (r *Router[T]) loadMux(mux *http.ServeMux, group *RouterGroup[T], state routeBuildState[T]) error {
	nextState := routeBuildState[T]{
		prefix:       state.prefix + group.Prefix,
		middlewares:  hook.MergeIncludedHandlers(state.middlewares, group.ExcludedMiddlewares, group.Middlewares, group.ExcludedMiddlewares),
		errorHandler: state.errorHandler,
	}
	if group.ErrorHandler != nil {
		nextState.errorHandler = group.ErrorHandler
	}

	for _, child := range group.Children {
		switch v := child.(type) {
		case *RouterGroup[T]:
			if err := r.loadMux(mux, v, nextState); err != nil {
				return err
			}
		case *Route[T]:
			routeHandlers := hook.MergeIncludedHandlers(nextState.middlewares, v.ExcludedMiddlewares, v.Middlewares, v.ExcludedMiddlewares)
			routeHook := &hook.Hook[T]{}
			routeHook.SetSortedHandlers(routeHandlers)
			hasMiddlewares := len(routeHandlers) > 0

			routePattern := nextState.prefix + v.Path
			if v.Method != "" {
				routePattern = v.Method + " " + routePattern
			}

			// Capture per-route values for the closure.
			routeHandle := v.Handle
			routeErrorHandler := v.ErrorHandler
			groupErrorHandler := nextState.errorHandler

			mux.HandleFunc(routePattern, func(resp http.ResponseWriter, req *http.Request) {
				// wrap the response to add write and status tracking
				responseWriter := acquireResponseWriter(resp)
				defer releaseResponseWriter(responseWriter)
				resp = responseWriter

				var body *RereadableReadCloser
				if req.Body != nil {
					// Keep reread support available, but only for paths that explicitly enable it.
					body = acquireRereadableBody(req.Body)
					defer releaseRereadableBody(body)
					req.Body = body
				}

				event, cleanupFunc := r.eventFactory(resp, req)
				if setter, ok := any(event).(eventDataSetter); ok {
					// Store only the path portion (strip "METHOD " prefix if present).
					patternPath := routePattern
					if _, route, ok := strings.Cut(routePattern, " "); ok && strings.HasPrefix(route, "/") {
						patternPath = route
					}
					setter.Set(EventKeyRoutePattern, patternPath)

					if routeHandle != nil {
						setter.Set(EventKeyRouteHandle, routeHandle)
					}
				}

				var err error
				if hasMiddlewares {
					err = routeHook.TriggerWithOneOff(event, v.Action)
				} else {
					err = v.Action(event)
				}

				// Route-level error boundary (most specific).
				if err != nil && routeErrorHandler != nil {
					err = routeErrorHandler(event, err)
				}

				// Group-level error boundary (nearest ancestor).
				if err != nil && groupErrorHandler != nil {
					err = groupErrorHandler(event, err)
				}

				if err != nil {
					ErrorHandler(resp, req, err)
				}

				if cleanupFunc != nil {
					cleanupFunc()
				}
			})
		default:
			return errors.New("invalid Group item type")
		}
	}

	return nil
}

func ErrorHandler(resp http.ResponseWriter, req *http.Request, err error) {
	if err == nil {
		return
	}

	if ok, _ := getWritten(resp); ok {
		return // a response was already written (aka. already handled)
	}

	header := resp.Header()
	if header.Get("Content-Type") == "" {
		header.Set("Content-Type", "application/json")
	}

	apiErr := ToApiError(err)

	resp.WriteHeader(apiErr.Status)

	if req.Method != http.MethodHead {
		if jsonErr := json.NewEncoder(resp).Encode(apiErr); jsonErr != nil {
			log.Println(jsonErr) // truly rare case, log to stderr only for dev purposes
		}
	}
}

func getBytesWritten(rw http.ResponseWriter) (int, error) {
	for {
		switch w := rw.(type) {
		case BytesWrittenTracker:
			return w.BytesWritten(), nil
		case RWUnwrapper:
			rw = w.Unwrap()
		default:
			return 0, http.ErrNotSupported
		}
	}
}

// -------------------------------------------------------------------

type WriteTracker interface {
	// Written reports whether a write operation has occurred.
	Written() bool
}

type StatusTracker interface {
	// Status reports the written response status code.
	Status() int
}

type flushErrorer interface {
	FlushError() error
}

type BytesWrittenTracker interface {
	BytesWritten() int
}

var (
	_ WriteTracker        = (*ResponseWriter)(nil)
	_ StatusTracker       = (*ResponseWriter)(nil)
	_ BytesWrittenTracker = (*ResponseWriter)(nil)
	_ http.Flusher        = (*ResponseWriter)(nil)
	_ http.Hijacker       = (*ResponseWriter)(nil)
	_ http.Pusher         = (*ResponseWriter)(nil)
	_ io.ReaderFrom       = (*ResponseWriter)(nil)
	_ flushErrorer        = (*ResponseWriter)(nil)
)

// ResponseWriter wraps a http.ResponseWriter to track its write state.
type ResponseWriter struct {
	http.ResponseWriter

	written      bool
	status       int
	bytesWritten int
}

func acquireResponseWriter(w http.ResponseWriter) *ResponseWriter {
	rw := responseWriterPool.Get().(*ResponseWriter)
	rw.ResponseWriter = w
	rw.written = false
	rw.status = 0
	rw.bytesWritten = 0
	return rw
}

func releaseResponseWriter(rw *ResponseWriter) {
	if rw == nil {
		return
	}

	rw.ResponseWriter = nil
	rw.written = false
	rw.status = 0
	rw.bytesWritten = 0
	responseWriterPool.Put(rw)
}

func (rw *ResponseWriter) WriteHeader(status int) {
	if rw.written {
		return
	}

	rw.written = true
	rw.status = status
	rw.ResponseWriter.WriteHeader(status)
}

func (rw *ResponseWriter) Write(b []byte) (int, error) {
	if !rw.written {
		rw.WriteHeader(http.StatusOK)
	}

	n, err := rw.ResponseWriter.Write(b)
	rw.bytesWritten += n
	return n, err
}

// Written implements [WriteTracker] and returns whether the current response body has been already written.
func (rw *ResponseWriter) Written() bool {
	return rw.written
}

// Written implements [StatusTracker] and returns the written status code of the current response.
func (rw *ResponseWriter) Status() int {
	return rw.status
}

// BytesWritten reports the total number of bytes written to the response body.
func (rw *ResponseWriter) BytesWritten() int {
	return rw.bytesWritten
}

// Flush implements [http.Flusher] and allows an HTTP handler to flush buffered data to the client.
// This method is no-op if the wrapped writer doesn't support it.
func (rw *ResponseWriter) Flush() {
	_ = rw.FlushError()
}

// FlushError is similar to [Flush] but returns [http.ErrNotSupported]
// if the wrapped writer doesn't support it.
func (rw *ResponseWriter) FlushError() error {
	err := http.NewResponseController(rw.ResponseWriter).Flush()
	if err == nil || !errors.Is(err, http.ErrNotSupported) {
		rw.written = true
	}
	return err
}

// Hijack implements [http.Hijacker] and allows an HTTP handler to take over the current connection.
func (rw *ResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return http.NewResponseController(rw.ResponseWriter).Hijack()
}

// Pusher implements [http.Pusher] to indicate HTTP/2 server push support.
func (rw *ResponseWriter) Push(target string, opts *http.PushOptions) error {
	w := rw.ResponseWriter
	for {
		switch p := w.(type) {
		case http.Pusher:
			return p.Push(target, opts)
		case RWUnwrapper:
			w = p.Unwrap()
		default:
			return http.ErrNotSupported
		}
	}
}

// ReaderFrom implements [io.ReaderFrom] by checking if the underlying writer supports it.
// Otherwise calls [io.Copy].
func (rw *ResponseWriter) ReadFrom(r io.Reader) (n int64, err error) {
	if !rw.written {
		rw.WriteHeader(http.StatusOK)
	}

	w := rw.ResponseWriter
	for {
		switch rf := w.(type) {
		case io.ReaderFrom:
			n, err := rf.ReadFrom(r)
			rw.bytesWritten += int(n)
			return n, err
		case RWUnwrapper:
			w = rf.Unwrap()
		default:
			n, err := io.Copy(rw.ResponseWriter, r)
			rw.bytesWritten += int(n)
			return n, err
		}
	}
}

// Unwrap returns the underlying ResponseWritter instance (usually used by [http.ResponseController]).
func (rw *ResponseWriter) Unwrap() http.ResponseWriter {
	return rw.ResponseWriter
}

func acquireRereadableBody(body io.ReadCloser) *RereadableReadCloser {
	wrapped := rereadableBodyPool.Get().(*RereadableReadCloser)
	wrapped.ReadCloser = body
	wrapped.copy = nil
	wrapped.closeErrors = nil
	wrapped.enabled = false
	wrapped.Lazy = true
	wrapped.MaxMemory = 0
	return wrapped
}

func releaseRereadableBody(body *RereadableReadCloser) {
	if body == nil {
		return
	}

	_ = body.Close()
	body.ReadCloser = nil
	body.copy = nil
	body.closeErrors = nil
	body.enabled = false
	body.Lazy = true
	body.MaxMemory = 0
	rereadableBodyPool.Put(body)
}

func getWritten(rw http.ResponseWriter) (bool, error) {
	for {
		switch w := rw.(type) {
		case WriteTracker:
			return w.Written(), nil
		case RWUnwrapper:
			rw = w.Unwrap()
		default:
			return false, http.ErrNotSupported
		}
	}
}

func getStatus(rw http.ResponseWriter) (int, error) {
	for {
		switch w := rw.(type) {
		case StatusTracker:
			return w.Status(), nil
		case RWUnwrapper:
			rw = w.Unwrap()
		default:
			return 0, http.ErrNotSupported
		}
	}
}
