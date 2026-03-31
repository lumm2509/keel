// Package htmx provides typed access to HTMX request and response headers.
//
// HTMX communicates with the server exclusively through HTTP headers, both
// to describe the triggering context (request headers) and to instruct the
// client on what to do after the response is received (response headers).
//
// The central type is [Event]. It covers the full response lifecycle: set
// HTMX headers and write the body in one coherent chain:
//
//	return c.HTMX().PushURL("/items").Trigger("listUpdated").HTML(http.StatusOK, fragment)
//
// Obtain an [Event] from a keel handler via c.HTMX() (cached per request), or
// construct one directly for use outside keel:
//
//	e := htmx.New(r, w)
//	if !e.IsHTMX() { ... }
//	return e.Retarget("#list").HTML(http.StatusOK, fragment)
package htmx

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// ── Header name constants ────────────────────────────────────────────────────

// HTMX request headers (sent by the browser).
const (
	HdrRequest           = "HX-Request"               // "true" when issued by htmx
	HdrBoosted           = "HX-Boosted"               // "true" when triggered via hx-boost
	HdrCurrentURL        = "HX-Current-URL"           // current URL of the browser page
	HdrHistoryRestoreReq = "HX-History-Restore-Request" // "true" on a history-cache miss
	HdrPrompt            = "HX-Prompt"                // value from an hx-prompt dialog
	HdrTarget            = "HX-Target"                // id of the swap target element
	HdrTrigger           = "HX-Trigger"               // id of the triggering element (request) / events to fire (response)
	HdrTriggerName       = "HX-Trigger-Name"          // name of the triggering element
)

// HTMX response headers (consumed by the browser).
const (
	HdrLocation           = "HX-Location"             // client-side redirect without page reload
	HdrPushURL            = "HX-Push-Url"             // push URL into the history stack
	HdrRedirect           = "HX-Redirect"             // full client-side redirect (page reload)
	HdrRefresh            = "HX-Refresh"              // "true" to trigger a full page refresh
	HdrReplaceURL         = "HX-Replace-Url"          // replace current URL in history (no new entry)
	HdrReswap             = "HX-Reswap"               // override the swap strategy
	HdrRetarget           = "HX-Retarget"             // override the swap target (CSS selector)
	HdrReselect           = "HX-Reselect"             // select a portion of the response (CSS selector)
	HdrTriggerAfterSettle = "HX-Trigger-After-Settle" // fire client events after settle
	HdrTriggerAfterSwap   = "HX-Trigger-After-Swap"   // fire client events after swap
)

// StatusStopPolling is the HTTP status code that tells HTMX to stop a polling
// request (hx-trigger="every Ns"). Use [Event.StopPolling] to send it.
const StatusStopPolling = 286

// ── Swap strategy constants ──────────────────────────────────────────────────

// Swap strategy values for [Event.Reswap] and the hx-swap HTML attribute.
const (
	SwapInnerHTML   = "innerHTML"
	SwapOuterHTML   = "outerHTML"
	SwapBeforeBegin = "beforebegin"
	SwapAfterBegin  = "afterbegin"
	SwapBeforeEnd   = "beforeend"
	SwapAfterEnd    = "afterend"
	SwapDelete      = "delete"
	SwapNone        = "none"
)

// ── LocationSpec ─────────────────────────────────────────────────────────────

// LocationSpec is the JSON body for [Event.LocationSpec] when a plain URL is
// not enough and a custom target, swap strategy, or request values are needed.
type LocationSpec struct {
	// Path is the URL to navigate to (required).
	Path string `json:"path"`
	// Source is the source element of the request (optional).
	Source string `json:"source,omitempty"`
	// Event is the event that triggered the request (optional).
	Event string `json:"event,omitempty"`
	// Handler is the callback that handles the response HTML (optional).
	Handler string `json:"handler,omitempty"`
	// Target is the CSS selector for the swap target (optional).
	Target string `json:"target,omitempty"`
	// Swap is the swap strategy; use a Swap* constant (optional).
	Swap string `json:"swap,omitempty"`
	// Select is a CSS selector to pick a portion of the response (optional).
	Select string `json:"select,omitempty"`
	// Values are extra values to submit with the request (optional).
	Values map[string]any `json:"values,omitempty"`
	// Headers are extra headers to send (optional).
	Headers map[string]any `json:"headers,omitempty"`
}

// ── Event ────────────────────────────────────────────────────────────────────

// Event is the HTMX view of a single HTTP exchange.
//
// It gives typed read access to all HTMX request headers and fluent write
// access to all HTMX response headers. Response methods (HTML, NoContent,
// StopPolling) terminate the chain and write the status + body so the handler
// can be completed in one expression:
//
//	return c.HTMX().PushURL("/items").Trigger("refresh").HTML(http.StatusOK, fragment)
//
// Inside keel handlers, obtain the cached instance via c.HTMX().
// Outside keel, construct directly with [New].
type Event struct {
	req  *http.Request
	resp http.ResponseWriter
}

// New creates an Event from a raw request and response writer.
// Inside keel handlers use c.HTMX() instead — it returns a cached instance.
func New(req *http.Request, resp http.ResponseWriter) *Event {
	return &Event{req: req, resp: resp}
}

// ── Request inspection ───────────────────────────────────────────────────────

// IsHTMX reports whether the request was issued by the HTMX library
// (HX-Request: true).
func (e *Event) IsHTMX() bool {
	return e.req.Header.Get(HdrRequest) == "true"
}

// IsBoosted reports whether the request was triggered via hx-boost.
func (e *Event) IsBoosted() bool {
	return e.req.Header.Get(HdrBoosted) == "true"
}

// IsHistoryRestore reports whether the request is a history-cache miss.
func (e *Event) IsHistoryRestore() bool {
	return e.req.Header.Get(HdrHistoryRestoreReq) == "true"
}

// CurrentURL returns the browser URL at request time. Empty for non-HTMX requests.
func (e *Event) CurrentURL() string {
	return e.req.Header.Get(HdrCurrentURL)
}

// Prompt returns the value the user entered in an hx-prompt dialog.
// Empty when no prompt was shown.
func (e *Event) Prompt() string {
	return e.req.Header.Get(HdrPrompt)
}

// Target returns the id of the swap target element.
func (e *Event) Target() string {
	return e.req.Header.Get(HdrTarget)
}

// TriggerID returns the id of the element that triggered the request.
func (e *Event) TriggerID() string {
	return e.req.Header.Get(HdrTrigger)
}

// TriggerName returns the name of the element that triggered the request.
func (e *Event) TriggerName() string {
	return e.req.Header.Get(HdrTriggerName)
}

// ── Response header setters (fluent) ─────────────────────────────────────────

// Location sets HX-Location to perform a client-side redirect to url without
// a full page reload. The URL is pushed into the browser history.
func (e *Event) Location(url string) *Event {
	e.resp.Header().Set(HdrLocation, url)
	return e
}

// LocationSpec sets HX-Location to a JSON-encoded [LocationSpec], enabling
// full control over the redirect target, swap strategy, and request values.
//
//	c.HTMX().LocationSpec(htmx.LocationSpec{
//	    Path:   "/items",
//	    Target: "#list",
//	    Swap:   htmx.SwapOuterHTML,
//	})
//
// Panics if spec cannot be marshalled to JSON, which only happens when Values
// or Headers contain non-serialisable types (func, chan, etc.) — a programmer
// error that should be caught in testing.
func (e *Event) LocationSpec(spec LocationSpec) *Event {
	b, err := json.Marshal(spec)
	if err != nil {
		panic(fmt.Sprintf("htmx: LocationSpec: failed to marshal spec: %v", err))
	}
	e.resp.Header().Set(HdrLocation, string(b))
	return e
}

// PushURL pushes url into the browser history stack without triggering a
// navigation. Pass "false" to prevent any push even if hx-push-url is set.
func (e *Event) PushURL(url string) *Event {
	e.resp.Header().Set(HdrPushURL, url)
	return e
}

// PreventPushURL tells HTMX not to push a URL, overriding any element-level
// hx-push-url attribute.
func (e *Event) PreventPushURL() *Event {
	e.resp.Header().Set(HdrPushURL, "false")
	return e
}

// Redirect performs a full client-side redirect to url (triggers a page load).
// Unlike Location, this causes a full page navigation, not an HTMX swap.
func (e *Event) Redirect(url string) *Event {
	e.resp.Header().Set(HdrRedirect, url)
	return e
}

// Refresh instructs the HTMX client to do a full page refresh.
func (e *Event) Refresh() *Event {
	e.resp.Header().Set(HdrRefresh, "true")
	return e
}

// ReplaceURL replaces the current URL in the browser history stack without
// adding a new entry. Pass "false" to prevent any replacement.
func (e *Event) ReplaceURL(url string) *Event {
	e.resp.Header().Set(HdrReplaceURL, url)
	return e
}

// PreventReplaceURL tells HTMX not to replace the URL, overriding any
// element-level hx-replace-url attribute.
func (e *Event) PreventReplaceURL() *Event {
	e.resp.Header().Set(HdrReplaceURL, "false")
	return e
}

// Reswap overrides the swap strategy for this response.
// Use one of the Swap* constants (e.g. [SwapOuterHTML]).
func (e *Event) Reswap(strategy string) *Event {
	e.resp.Header().Set(HdrReswap, strategy)
	return e
}

// Retarget overrides the swap target to the given CSS selector.
func (e *Event) Retarget(selector string) *Event {
	e.resp.Header().Set(HdrRetarget, selector)
	return e
}

// Reselect picks a portion of the response for swapping using a CSS selector.
// Useful when the response contains more HTML than what should be swapped in.
func (e *Event) Reselect(selector string) *Event {
	e.resp.Header().Set(HdrReselect, selector)
	return e
}

// Trigger sets HX-Trigger to fire one or more named events on the client
// immediately after the swap.
//
//	c.HTMX().Trigger("itemAdded", "listUpdated")
func (e *Event) Trigger(events ...string) *Event {
	e.resp.Header().Set(HdrTrigger, strings.Join(events, ", "))
	return e
}

// TriggerDetail sets HX-Trigger to a JSON-encoded map so events can carry
// detail data:
//
//	c.HTMX().TriggerDetail(map[string]any{
//	    "itemAdded": map[string]any{"id": 42},
//	})
//
// Panics if events cannot be marshalled to JSON (programmer error).
func (e *Event) TriggerDetail(events map[string]any) *Event {
	b, err := json.Marshal(events)
	if err != nil {
		panic(fmt.Sprintf("htmx: TriggerDetail: failed to marshal events: %v", err))
	}
	e.resp.Header().Set(HdrTrigger, string(b))
	return e
}

// TriggerAfterSettle fires one or more named events after the DOM settle phase
// (all CSS transitions complete).
func (e *Event) TriggerAfterSettle(events ...string) *Event {
	e.resp.Header().Set(HdrTriggerAfterSettle, strings.Join(events, ", "))
	return e
}

// TriggerAfterSettleDetail is the JSON-encoded variant of [TriggerAfterSettle].
// Panics if events cannot be marshalled to JSON (programmer error).
func (e *Event) TriggerAfterSettleDetail(events map[string]any) *Event {
	b, err := json.Marshal(events)
	if err != nil {
		panic(fmt.Sprintf("htmx: TriggerAfterSettleDetail: failed to marshal events: %v", err))
	}
	e.resp.Header().Set(HdrTriggerAfterSettle, string(b))
	return e
}

// TriggerAfterSwap fires one or more named events after the swap phase.
func (e *Event) TriggerAfterSwap(events ...string) *Event {
	e.resp.Header().Set(HdrTriggerAfterSwap, strings.Join(events, ", "))
	return e
}

// TriggerAfterSwapDetail is the JSON-encoded variant of [TriggerAfterSwap].
// Panics if events cannot be marshalled to JSON (programmer error).
func (e *Event) TriggerAfterSwapDetail(events map[string]any) *Event {
	b, err := json.Marshal(events)
	if err != nil {
		panic(fmt.Sprintf("htmx: TriggerAfterSwapDetail: failed to marshal events: %v", err))
	}
	e.resp.Header().Set(HdrTriggerAfterSwap, string(b))
	return e
}

// ── Response writers ─────────────────────────────────────────────────────────
// These methods terminate the fluent chain by writing status + body.
// They follow the same contract as the equivalent methods on http.Event:
// all HTMX response headers set before this call are already in the header
// map and will be sent as part of the same response.

// HTML writes an HTML fragment response. This is the most common HTMX response.
// Call it at the end of the fluent chain after all header setters:
//
//	return c.HTMX().PushURL("/items").Trigger("listUpdated").HTML(http.StatusOK, fragment)
func (e *Event) HTML(status int, fragment string) error {
	header := e.resp.Header()
	if header.Get("Content-Type") == "" {
		header.Set("Content-Type", "text/html; charset=utf-8")
	}
	e.resp.WriteHeader(status)
	_, err := e.resp.Write([]byte(fragment))
	return err
}

// NoContent writes a response with no body (HTTP 204). Use when the intent is
// expressed entirely through HTMX response headers (e.g. trigger an event,
// push a URL) with no DOM swap needed.
//
//	return c.HTMX().Trigger("cartUpdated").NoContent()
func (e *Event) NoContent() error {
	e.resp.WriteHeader(http.StatusNoContent)
	return nil
}

// StopPolling writes HTTP 286, instructing HTMX to stop a polling request
// (hx-trigger="every Ns"). The response body is ignored by the client.
//
//	return c.HTMX().StopPolling()
func (e *Event) StopPolling() error {
	e.resp.WriteHeader(StatusStopPolling)
	return nil
}

// ── Low-level / standalone helpers ───────────────────────────────────────────
// These accept raw *http.Request / http.ResponseWriter directly. Useful in
// middleware or adapters that do not have access to a keel Event.

// IsHTMXRequest reports whether r was issued by the HTMX library.
func IsHTMXRequest(r *http.Request) bool {
	return r.Header.Get(HdrRequest) == "true"
}

// SetPushURL sets the HX-Push-Url response header.
func SetPushURL(w http.ResponseWriter, url string) {
	w.Header().Set(HdrPushURL, url)
}

// SetRetarget sets the HX-Retarget response header.
func SetRetarget(w http.ResponseWriter, selector string) {
	w.Header().Set(HdrRetarget, selector)
}

// SetReswap sets the HX-Reswap response header.
func SetReswap(w http.ResponseWriter, strategy string) {
	w.Header().Set(HdrReswap, strategy)
}

// SetTrigger sets the HX-Trigger response header to one or more event names.
func SetTrigger(w http.ResponseWriter, events ...string) {
	w.Header().Set(HdrTrigger, strings.Join(events, ", "))
}
