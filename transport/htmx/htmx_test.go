package htmx_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/lumm2509/keel/transport/htmx"
)

// newEvent builds a test htmx.Event from a recorder and a request pre-loaded
// with the given headers.
func newEvent(t *testing.T, reqHeaders map[string]string) (*htmx.Event, *httptest.ResponseRecorder) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	for k, v := range reqHeaders {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	return htmx.New(req, rec), rec
}

// ── Request inspection ───────────────────────────────────────────────────────

func TestIsHTMX(t *testing.T) {
	t.Parallel()

	e, _ := newEvent(t, map[string]string{htmx.HdrRequest: "true"})
	if !e.IsHTMX() {
		t.Fatal("expected IsHTMX() true")
	}

	e2, _ := newEvent(t, nil)
	if e2.IsHTMX() {
		t.Fatal("expected IsHTMX() false for plain request")
	}
}

func TestIsBoosted(t *testing.T) {
	t.Parallel()

	e, _ := newEvent(t, map[string]string{htmx.HdrBoosted: "true"})
	if !e.IsBoosted() {
		t.Fatal("expected IsBoosted() true")
	}
}

func TestIsHistoryRestore(t *testing.T) {
	t.Parallel()

	e, _ := newEvent(t, map[string]string{htmx.HdrHistoryRestoreReq: "true"})
	if !e.IsHistoryRestore() {
		t.Fatal("expected IsHistoryRestore() true")
	}
}

func TestCurrentURL(t *testing.T) {
	t.Parallel()

	e, _ := newEvent(t, map[string]string{htmx.HdrCurrentURL: "https://example.com/page"})
	if got := e.CurrentURL(); got != "https://example.com/page" {
		t.Fatalf("CurrentURL: got %q", got)
	}
}

func TestPrompt(t *testing.T) {
	t.Parallel()

	e, _ := newEvent(t, map[string]string{htmx.HdrPrompt: "delete me"})
	if got := e.Prompt(); got != "delete me" {
		t.Fatalf("Prompt: got %q", got)
	}
}

func TestTarget(t *testing.T) {
	t.Parallel()

	e, _ := newEvent(t, map[string]string{htmx.HdrTarget: "main-content"})
	if got := e.Target(); got != "main-content" {
		t.Fatalf("Target: got %q", got)
	}
}

func TestTriggerID(t *testing.T) {
	t.Parallel()

	e, _ := newEvent(t, map[string]string{htmx.HdrTrigger: "btn-submit"})
	if got := e.TriggerID(); got != "btn-submit" {
		t.Fatalf("TriggerID: got %q", got)
	}
}

func TestTriggerName(t *testing.T) {
	t.Parallel()

	e, _ := newEvent(t, map[string]string{htmx.HdrTriggerName: "submit-button"})
	if got := e.TriggerName(); got != "submit-button" {
		t.Fatalf("TriggerName: got %q", got)
	}
}

// ── Response header setters ──────────────────────────────────────────────────

func TestLocation(t *testing.T) {
	t.Parallel()

	e, rec := newEvent(t, nil)
	e.Location("/items")
	if got := rec.Header().Get(htmx.HdrLocation); got != "/items" {
		t.Fatalf("Location header: got %q", got)
	}
}

func TestLocationSpec(t *testing.T) {
	t.Parallel()

	e, rec := newEvent(t, nil)
	e.LocationSpec(htmx.LocationSpec{
		Path:   "/items",
		Target: "#list",
		Swap:   htmx.SwapOuterHTML,
	})

	raw := rec.Header().Get(htmx.HdrLocation)
	var spec htmx.LocationSpec
	if err := json.Unmarshal([]byte(raw), &spec); err != nil {
		t.Fatalf("LocationSpec header is not valid JSON: %v — got %q", err, raw)
	}
	if spec.Path != "/items" {
		t.Fatalf("LocationSpec.Path: got %q", spec.Path)
	}
	if spec.Target != "#list" {
		t.Fatalf("LocationSpec.Target: got %q", spec.Target)
	}
	if spec.Swap != htmx.SwapOuterHTML {
		t.Fatalf("LocationSpec.Swap: got %q", spec.Swap)
	}
}

func TestLocationSpecPanicsOnBadInput(t *testing.T) {
	t.Parallel()

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for non-serialisable LocationSpec values, got none")
		}
	}()

	e, _ := newEvent(t, nil)
	e.LocationSpec(htmx.LocationSpec{
		Path:   "/x",
		Values: map[string]any{"ch": make(chan int)}, // not serialisable
	})
}

func TestPushURL(t *testing.T) {
	t.Parallel()

	e, rec := newEvent(t, nil)
	e.PushURL("/items")
	if got := rec.Header().Get(htmx.HdrPushURL); got != "/items" {
		t.Fatalf("PushURL header: got %q", got)
	}
}

func TestPreventPushURL(t *testing.T) {
	t.Parallel()

	e, rec := newEvent(t, nil)
	e.PreventPushURL()
	if got := rec.Header().Get(htmx.HdrPushURL); got != "false" {
		t.Fatalf("PreventPushURL header: got %q", got)
	}
}

func TestRedirect(t *testing.T) {
	t.Parallel()

	e, rec := newEvent(t, nil)
	e.Redirect("/login")
	if got := rec.Header().Get(htmx.HdrRedirect); got != "/login" {
		t.Fatalf("Redirect header: got %q", got)
	}
}

func TestRefresh(t *testing.T) {
	t.Parallel()

	e, rec := newEvent(t, nil)
	e.Refresh()
	if got := rec.Header().Get(htmx.HdrRefresh); got != "true" {
		t.Fatalf("Refresh header: got %q", got)
	}
}

func TestReplaceURL(t *testing.T) {
	t.Parallel()

	e, rec := newEvent(t, nil)
	e.ReplaceURL("/items")
	if got := rec.Header().Get(htmx.HdrReplaceURL); got != "/items" {
		t.Fatalf("ReplaceURL header: got %q", got)
	}
}

func TestPreventReplaceURL(t *testing.T) {
	t.Parallel()

	e, rec := newEvent(t, nil)
	e.PreventReplaceURL()
	if got := rec.Header().Get(htmx.HdrReplaceURL); got != "false" {
		t.Fatalf("PreventReplaceURL header: got %q", got)
	}
}

func TestReswap(t *testing.T) {
	t.Parallel()

	e, rec := newEvent(t, nil)
	e.Reswap(htmx.SwapBeforeEnd)
	if got := rec.Header().Get(htmx.HdrReswap); got != htmx.SwapBeforeEnd {
		t.Fatalf("Reswap header: got %q", got)
	}
}

func TestRetarget(t *testing.T) {
	t.Parallel()

	e, rec := newEvent(t, nil)
	e.Retarget("#results")
	if got := rec.Header().Get(htmx.HdrRetarget); got != "#results" {
		t.Fatalf("Retarget header: got %q", got)
	}
}

func TestReselect(t *testing.T) {
	t.Parallel()

	e, rec := newEvent(t, nil)
	e.Reselect(".card")
	if got := rec.Header().Get(htmx.HdrReselect); got != ".card" {
		t.Fatalf("Reselect header: got %q", got)
	}
}

func TestTriggerSingleEvent(t *testing.T) {
	t.Parallel()

	e, rec := newEvent(t, nil)
	e.Trigger("itemAdded")
	if got := rec.Header().Get(htmx.HdrTrigger); got != "itemAdded" {
		t.Fatalf("Trigger header: got %q", got)
	}
}

func TestTriggerMultipleEvents(t *testing.T) {
	t.Parallel()

	e, rec := newEvent(t, nil)
	e.Trigger("itemAdded", "listUpdated")
	if got := rec.Header().Get(htmx.HdrTrigger); got != "itemAdded, listUpdated" {
		t.Fatalf("Trigger header: got %q", got)
	}
}

func TestTriggerDetail(t *testing.T) {
	t.Parallel()

	e, rec := newEvent(t, nil)
	e.TriggerDetail(map[string]any{
		"itemAdded": map[string]any{"id": 42},
	})

	raw := rec.Header().Get(htmx.HdrTrigger)
	var got map[string]any
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatalf("TriggerDetail header is not valid JSON: %v — got %q", err, raw)
	}
	if _, ok := got["itemAdded"]; !ok {
		t.Fatalf("TriggerDetail: missing key 'itemAdded' in %q", raw)
	}
}

func TestTriggerDetailPanicsOnBadInput(t *testing.T) {
	t.Parallel()

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for non-serialisable map value, got none")
		}
	}()

	e, _ := newEvent(t, nil)
	e.TriggerDetail(map[string]any{"bad": make(chan int)})
}

func TestTriggerAfterSettle(t *testing.T) {
	t.Parallel()

	e, rec := newEvent(t, nil)
	e.TriggerAfterSettle("animationDone")
	if got := rec.Header().Get(htmx.HdrTriggerAfterSettle); got != "animationDone" {
		t.Fatalf("TriggerAfterSettle header: got %q", got)
	}
}

func TestTriggerAfterSettleDetailPanicsOnBadInput(t *testing.T) {
	t.Parallel()

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for non-serialisable map value, got none")
		}
	}()

	e, _ := newEvent(t, nil)
	e.TriggerAfterSettleDetail(map[string]any{"bad": make(chan int)})
}

func TestTriggerAfterSwap(t *testing.T) {
	t.Parallel()

	e, rec := newEvent(t, nil)
	e.TriggerAfterSwap("swapDone")
	if got := rec.Header().Get(htmx.HdrTriggerAfterSwap); got != "swapDone" {
		t.Fatalf("TriggerAfterSwap header: got %q", got)
	}
}

func TestTriggerAfterSwapDetailPanicsOnBadInput(t *testing.T) {
	t.Parallel()

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for non-serialisable map value, got none")
		}
	}()

	e, _ := newEvent(t, nil)
	e.TriggerAfterSwapDetail(map[string]any{"bad": make(chan int)})
}

// ── Fluent chaining ──────────────────────────────────────────────────────────

func TestFluentChainSetsMultipleHeaders(t *testing.T) {
	t.Parallel()

	_, rec := newEvent(t, nil)
	htmx.New(httptest.NewRequest(http.MethodGet, "/", nil), rec).
		PushURL("/items").
		Retarget("#list").
		Trigger("refreshed")

	if got := rec.Header().Get(htmx.HdrPushURL); got != "/items" {
		t.Fatalf("PushURL: got %q", got)
	}
	if got := rec.Header().Get(htmx.HdrRetarget); got != "#list" {
		t.Fatalf("Retarget: got %q", got)
	}
	if got := rec.Header().Get(htmx.HdrTrigger); got != "refreshed" {
		t.Fatalf("Trigger: got %q", got)
	}
}

// ── Response writers ─────────────────────────────────────────────────────────

func TestHTML(t *testing.T) {
	t.Parallel()

	e, rec := newEvent(t, nil)
	if err := e.HTML(http.StatusOK, "<p>hello</p>"); err != nil {
		t.Fatalf("HTML() error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("HTML status: got %d", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "text/html; charset=utf-8" {
		t.Fatalf("HTML Content-Type: got %q", got)
	}
	if got := rec.Body.String(); got != "<p>hello</p>" {
		t.Fatalf("HTML body: got %q", got)
	}
}

func TestHTMLDoesNotOverrideExistingContentType(t *testing.T) {
	t.Parallel()

	e, rec := newEvent(t, nil)
	rec.Header().Set("Content-Type", "text/html; charset=iso-8859-1")
	_ = e.HTML(http.StatusOK, "x")
	if got := rec.Header().Get("Content-Type"); got != "text/html; charset=iso-8859-1" {
		t.Fatalf("HTML should not override existing Content-Type, got %q", got)
	}
}

func TestNoContent(t *testing.T) {
	t.Parallel()

	e, rec := newEvent(t, nil)
	if err := e.NoContent(); err != nil {
		t.Fatalf("NoContent() error: %v", err)
	}
	if rec.Code != http.StatusNoContent {
		t.Fatalf("NoContent status: got %d", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Fatalf("NoContent body must be empty, got %q", rec.Body.String())
	}
}

func TestStopPolling(t *testing.T) {
	t.Parallel()

	e, rec := newEvent(t, nil)
	if err := e.StopPolling(); err != nil {
		t.Fatalf("StopPolling() error: %v", err)
	}
	if rec.Code != htmx.StatusStopPolling {
		t.Fatalf("StopPolling status: got %d, want %d", rec.Code, htmx.StatusStopPolling)
	}
}

func TestHTMLAfterHeaderSetters(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodPost, "/items", nil)
	rec := httptest.NewRecorder()

	err := htmx.New(req, rec).
		PushURL("/items").
		Trigger("itemAdded").
		HTML(http.StatusOK, "<li>new item</li>")

	if err != nil {
		t.Fatalf("chain error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d", rec.Code)
	}
	if got := rec.Header().Get(htmx.HdrPushURL); got != "/items" {
		t.Fatalf("PushURL after HTML: got %q", got)
	}
	if got := rec.Header().Get(htmx.HdrTrigger); got != "itemAdded" {
		t.Fatalf("Trigger after HTML: got %q", got)
	}
	if got := rec.Body.String(); got != "<li>new item</li>" {
		t.Fatalf("body: got %q", got)
	}
}

// ── Standalone helpers ───────────────────────────────────────────────────────

func TestIsHTMXRequest(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if htmx.IsHTMXRequest(req) {
		t.Fatal("expected false for plain request")
	}
	req.Header.Set(htmx.HdrRequest, "true")
	if !htmx.IsHTMXRequest(req) {
		t.Fatal("expected true after setting HX-Request")
	}
}

func TestSetPushURL(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	htmx.SetPushURL(rec, "/page")
	if got := rec.Header().Get(htmx.HdrPushURL); got != "/page" {
		t.Fatalf("SetPushURL: got %q", got)
	}
}

func TestSetRetarget(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	htmx.SetRetarget(rec, "#main")
	if got := rec.Header().Get(htmx.HdrRetarget); got != "#main" {
		t.Fatalf("SetRetarget: got %q", got)
	}
}

func TestSetReswap(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	htmx.SetReswap(rec, htmx.SwapOuterHTML)
	if got := rec.Header().Get(htmx.HdrReswap); got != htmx.SwapOuterHTML {
		t.Fatalf("SetReswap: got %q", got)
	}
}

func TestSetTrigger(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	htmx.SetTrigger(rec, "a", "b")
	if got := rec.Header().Get(htmx.HdrTrigger); got != "a, b" {
		t.Fatalf("SetTrigger: got %q", got)
	}
}
