package http

import (
	"database/sql"
	"log/slog"
	"net/http"
	"net/netip"
	"strings"
	"sync"
	"time"

	"github.com/lumm2509/keel/infra/store"
	"github.com/lumm2509/keel/pkg/inflector"
	"github.com/lumm2509/keel/pkg/subscriptions"
)

// Common request store keys used by the middlewares and api handlers.
const (
	RequestEventKeyInfoContext = "infoContext"
)

const (
	RequestInfoContextDefault       = "default"
	RequestInfoContextExpand        = "expand"
	RequestInfoContextRealtime      = "realtime"
	RequestInfoContextProtectedFile = "protectedFile"
	RequestInfoContextBatch         = "batch"
	RequestInfoContextOAuth2        = "oauth2"
	RequestInfoContextOTP           = "otp"
	RequestInfoContextPasswordAuth  = "password"
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

type RequestContainer[Cradle any] interface {
	Cradle() *Cradle
	Logger() *slog.Logger
	Store() *store.Store[string, any]
	IsDev() bool
	DataBase() *sql.DB
	SubscriptionsBroker() *subscriptions.Broker
}

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
type RequestEvent[Cradle any] struct {
	Container         RequestContainer[Cradle]
	cachedRequestInfo *RequestInfo
	Event

	mu           sync.Mutex
	requestID    string
	startTime    time.Time
	loggerBase   *slog.Logger
	logger       *slog.Logger
	routePattern string
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

func (e *RequestEvent[Cradle]) Cradle() *Cradle {
	return e.Container.Cradle()
}

func (e *RequestEvent[Cradle]) RequestID() string {
	return e.requestID
}

func (e *RequestEvent[Cradle]) SetRequestID(id string) {
	e.requestID = id
	if e.Request != nil {
		if id != "" {
			e.Request.Header.Set("X-Request-ID", id)
		} else {
			e.Request.Header.Del("X-Request-ID")
		}
	}
	if e.Response != nil && id != "" {
		e.Response.Header().Set("X-Request-ID", id)
	}
	e.logger = nil
}

func (e *RequestEvent[Cradle]) StartTime() time.Time {
	return e.startTime
}

func (e *RequestEvent[Cradle]) Duration() time.Duration {
	if e.startTime.IsZero() {
		return 0
	}

	return time.Since(e.startTime)
}

func (e *RequestEvent[Cradle]) Logger() *slog.Logger {
	if e.logger != nil {
		return e.logger
	}

	e.rebuildLogger()
	return e.logger
}

func (e *RequestEvent[Cradle]) WithLogAttrs(args ...any) *slog.Logger {
	return e.Logger().With(args...)
}

func (e *RequestEvent[Cradle]) ClientIP() string {
	if provider, ok := any(e.Container).(TrustedProxyProvider); ok {
		return e.RealIPFromTrustedProxies(provider.TrustedProxyRanges())
	}

	return e.RemoteIP()
}

func (e *RequestEvent[Cradle]) RoutePattern() string {
	return e.routePattern
}

func (e *RequestEvent[Cradle]) SetStartTime(start time.Time) {
	e.startTime = start
}

func (e *RequestEvent[Cradle]) SetLogger(logger *slog.Logger) {
	e.loggerBase = logger
	e.logger = nil
}

func (e *RequestEvent[Cradle]) SetRoutePattern(pattern string) {
	if method, route, ok := strings.Cut(pattern, " "); ok && method != "" && strings.HasPrefix(route, "/") {
		e.routePattern = route
	} else {
		e.routePattern = pattern
	}
	e.logger = nil
}

func (e *RequestEvent[Cradle]) Reset(container RequestContainer[Cradle], response http.ResponseWriter, request *http.Request) {
	e.releaseCachedRequestInfo()
	e.Container = container
	e.Event = Event{
		Response: response,
		Request:  request,
	}
	e.requestID = ""
	e.startTime = time.Time{}
	e.loggerBase = nil
	e.logger = nil
	e.routePattern = ""
}

func (e *RequestEvent[Cradle]) Release() {
	e.releaseCachedRequestInfo()
	e.Container = nil
	e.Event = Event{}
	e.requestID = ""
	e.startTime = time.Time{}
	e.loggerBase = nil
	e.logger = nil
	e.routePattern = ""
}

func (e *RequestEvent[Cradle]) releaseCachedRequestInfo() {
	releaseRequestInfo(e.cachedRequestInfo)
	e.cachedRequestInfo = nil
}

func (e *RequestEvent[Cradle]) rebuildLogger() {
	base := e.loggerBase
	if base == nil {
		base = slog.Default()
		if e.Container != nil && e.Container.Logger() != nil {
			base = e.Container.Logger()
		}
	}

	args := make([]any, 0, 8)
	if e.requestID != "" {
		args = append(args, "request_id", e.requestID)
	}
	if e.Request != nil {
		args = append(args, "method", e.Request.Method)
		if e.Request.URL != nil {
			args = append(args, "path", e.Request.URL.Path)
		}
	}
	if e.routePattern != "" {
		args = append(args, "route", e.routePattern)
	}

	e.logger = base.With(args...)
}

// RequestInfo parses the current request into RequestInfo instance.
//
// Note that the returned result is cached to avoid copying the request data multiple times
// but the auth state and other common store items are always refreshed in case they were changed by another handler.
func (e *RequestEvent[C]) RequestInfo() (*RequestInfo, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.cachedRequestInfo != nil {
		infoCtx, _ := e.Get(RequestEventKeyInfoContext).(string)
		if infoCtx != "" {
			e.cachedRequestInfo.Context = infoCtx
		} else {
			e.cachedRequestInfo.Context = RequestInfoContextDefault
		}
	} else {
		if err := e.initRequestInfo(); err != nil {
			return nil, err
		}
	}

	return e.cachedRequestInfo, nil
}

func (e *RequestEvent[C]) initRequestInfo() error {
	infoCtx, _ := e.Get(RequestEventKeyInfoContext).(string)
	if infoCtx == "" {
		infoCtx = RequestInfoContextDefault
	}

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

	for k, v := range e.Request.Header {
		if len(v) > 0 {
			info.Headers[inflector.Snakecase(k)] = v[0]
		}
	}

	e.cachedRequestInfo = info

	return nil
}
