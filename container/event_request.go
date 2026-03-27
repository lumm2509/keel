package container

import (
	"maps"
	"sync"

	"github.com/lumm2509/keel/pkg/inflector"
	"github.com/lumm2509/keel/transport/http"
)

// Common request store keys used by the middlewares and api handlers.
const (
	RequestEventKeyInfoContext = "infoContext"
)

// RequestEvent defines the PocketBase router handler event.
type RequestEvent[Cradle any] struct {
	Container         Container[Cradle]
	cachedRequestInfo *RequestInfo
	http.Event

	mu sync.Mutex
}

func (e *RequestEvent[Cradle]) Cradle() *Cradle {
	return e.Container.Cradle()
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
		// (re)init e.cachedRequestInfo based on the current request event
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

	info := &RequestInfo{
		Context: infoCtx,
		Method:  e.Request.Method,
		Query:   map[string]string{},
		Headers: map[string]string{},
		Body:    map[string]any{},
	}

	if err := e.BindBody(&info.Body); err != nil {
		return err
	}

	// extract the first value of all query params
	query := e.Request.URL.Query()
	for k, v := range query {
		if len(v) > 0 {
			info.Query[k] = v[0]
		}
	}

	// extract the first value of all headers and normalizes the keys
	// ("X-Token" is converted to "x_token")
	for k, v := range e.Request.Header {
		if len(v) > 0 {
			info.Headers[inflector.Snakecase(k)] = v[0]
		}
	}

	e.cachedRequestInfo = info

	return nil
}

// -------------------------------------------------------------------

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

// RequestInfo defines a HTTP request data struct, usually used
// as part of the `@request.*` filter resolver.
//
// The Query and Headers fields contains only the first value for each found entry.
type RequestInfo struct {
	Query   map[string]string `json:"query"`
	Headers map[string]string `json:"headers"`
	Body    map[string]any    `json:"body"`
	Method  string            `json:"method"`
	Context string            `json:"context"`
}

// Clone creates a new shallow copy of the current RequestInfo and its Auth record (if any).
func (info *RequestInfo) Clone() *RequestInfo {
	clone := &RequestInfo{
		Method:  info.Method,
		Context: info.Context,
		Query:   maps.Clone(info.Query),
		Body:    maps.Clone(info.Body),
		Headers: maps.Clone(info.Headers),
	}

	return clone
}
