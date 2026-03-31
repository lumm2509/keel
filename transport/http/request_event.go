package http

import (
	"log/slog"
	"net/http"
	"net/netip"
	"strings"
	"sync"

	"github.com/lumm2509/keel/pkg/inflector"
)

// Common request store keys used by the middlewares and api handlers.
const (
	RequestEventKeyInfoContext = "infoContext"
)

var requestInfoPool = sync.Pool{
	New: func() any {
		return &RequestInfo{
			Query:   make(map[string]string),
			Headers: make(map[string]string),
			Body:    make(map[string]any),
		}
	},
}

// TrustedProxyProvider can be implemented by the App type (T) to enable
// trusted-proxy IP resolution in ClientIP.
type TrustedProxyProvider interface {
	TrustedProxyRanges() []netip.Prefix
}

type RequestInfo struct {
	Query   map[string]string `json:"query"`
	Headers map[string]string `json:"headers"`
	Body    map[string]any    `json:"body"`
	Method  string            `json:"method"`
	Context string            `json:"context"`
}

func acquireRequestInfo() *RequestInfo {
	info := requestInfoPool.Get().(*RequestInfo)
	clearStringMap(info.Query)
	clearStringMap(info.Headers)
	clearAnyMap(info.Body)
	info.Method = ""
	info.Context = ""
	return info
}

func releaseRequestInfo(info *RequestInfo) {
	if info == nil {
		return
	}

	clearStringMap(info.Query)
	clearStringMap(info.Headers)
	clearAnyMap(info.Body)
	info.Method = ""
	info.Context = ""
	requestInfoPool.Put(info)
}

func (info *RequestInfo) Clone() *RequestInfo {
	if info == nil {
		return nil
	}

	clone := &RequestInfo{
		Method:  info.Method,
		Context: info.Context,
		Query:   map[string]string{},
		Headers: map[string]string{},
		Body:    map[string]any{},
	}
	for k, v := range info.Query {
		clone.Query[k] = v
	}
	for k, v := range info.Headers {
		clone.Headers[k] = v
	}
	for k, v := range info.Body {
		clone.Body[k] = v
	}

	return clone
}

// RequestEvent defines the router handler event.
//
// Like EventData, RequestEvent is single-goroutine per request: the middleware
// chain is sequential and the event must not be shared across goroutines.
// The one exception is cachedRequestInfo, which is lazily initialised by
// RequestInfo() and protected by mu so that callers that explicitly pass the
// event to a spawned goroutine do not race on the first call.
// All other fields (App, EventData) carry no such guarantee.
type RequestEvent[T any] struct {
	App               *T
	cachedRequestInfo *RequestInfo
	Event

	mu sync.Mutex // guards cachedRequestInfo lazy init only
}

func clearStringMap[M ~map[string]string](m M) {
	for k := range m {
		delete(m, k)
	}
}

func clearAnyMap[M ~map[string]any](m M) {
	for k := range m {
		delete(m, k)
	}
}

// Param returns the URL path parameter with the given name from the matched route.
//
// For example, given a route registered as GET /users/{id}, calling Param("id")
// returns the value of the {id} segment.
//
// When KEEL_DEBUG is set and the name is not present in the matched route pattern
// (likely a typo), a warning is logged to help catch the mistake early.
func (e *RequestEvent[T]) Param(name string) string {
	val := e.Request.PathValue(name)
	if val == "" && isDebugMode() {
		pattern, _ := e.Get(EventKeyRoutePattern).(string)
		if !strings.Contains(pattern, "{"+name+"}") &&
			!strings.Contains(pattern, "{"+name+"...}") {
			slog.Warn("[keel/debug] Param() called with a name not found in the route pattern",
				"param", name,
				"pattern", pattern,
				"hint", "check for a typo in the param name",
			)
		}
	}
	return val
}

// ClientIP returns the real client IP. If App implements TrustedProxyProvider,
// proxy headers are used when the direct peer is in a trusted range.
func (e *RequestEvent[T]) ClientIP() string {
	if provider, ok := any(e.App).(TrustedProxyProvider); ok {
		return e.RealIPFromTrustedProxies(provider.TrustedProxyRanges())
	}

	return e.RemoteIP()
}

func (e *RequestEvent[T]) Reset(app *T, response http.ResponseWriter, request *http.Request) {
	e.releaseCachedRequestInfo()
	e.App = app
	e.Event = Event{
		Response: response,
		Request:  request,
	}
}

func (e *RequestEvent[T]) Release() {
	e.releaseCachedRequestInfo()
	e.App = nil
	e.Event = Event{}
}

func (e *RequestEvent[T]) releaseCachedRequestInfo() {
	releaseRequestInfo(e.cachedRequestInfo)
	e.cachedRequestInfo = nil
}

// RequestInfo parses the current request into RequestInfo instance.
//
// Note that the returned result is cached to avoid copying the request data multiple times
// but the auth state and other common store items are always refreshed in case they were changed by another handler.
func (e *RequestEvent[T]) RequestInfo() (*RequestInfo, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.cachedRequestInfo != nil {
		infoCtx, _ := e.Get(RequestEventKeyInfoContext).(string)
		if infoCtx != "" {
			e.cachedRequestInfo.Context = infoCtx
		} else {
			e.cachedRequestInfo.Context = "default"
		}
	} else {
		if err := e.initRequestInfo(); err != nil {
			return nil, err
		}
	}

	return e.cachedRequestInfo, nil
}

func (e *RequestEvent[T]) initRequestInfo() error {
	infoCtx, _ := e.Get(RequestEventKeyInfoContext).(string)
	if infoCtx == "" {
		infoCtx = "default"
	}

	// Enable body replay before consuming it so that handlers called after
	// RequestInfo() can still read the body via BindBody().
	e.EnableBodyReread()

	info := acquireRequestInfo()
	info.Context = infoCtx
	info.Method = e.Request.Method

	if err := e.BindBody(&info.Body); err != nil {
		releaseRequestInfo(info)
		return err
	}

	if rawQuery := e.Request.URL.RawQuery; rawQuery != "" {
		query := e.Request.URL.Query()
		for k, v := range query {
			if len(v) > 0 {
				info.Query[k] = v[0]
			}
		}
	}

	// Header names are normalized to snake_case.
	// e.g. "Content-Type" → "content_type", "X-Request-ID" → "x_request_id".
	// Use e.Request.Header directly if you need the original canonical names.
	for k, v := range e.Request.Header {
		if len(v) > 0 {
			info.Headers[inflector.Snakecase(k)] = v[0]
		}
	}

	e.cachedRequestInfo = info

	return nil
}
