// Package main demonstrates serving an HTMX-powered UI with keel.
//
// Shows:
//   - c.HTMX().IsHTMX() to distinguish HTMX requests from full-page loads
//   - fluent header chain + HTML() to respond in one expression
//   - PushURL to update the browser URL without a page reload
//   - Trigger to fire client-side events after the swap
//   - Retarget + Reswap to override swap behaviour from the server
//   - NoContent for header-only responses (no DOM swap needed)
//   - StopPolling to end an hx-trigger="every Ns" polling loop
package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/lumm2509/keel"
	"github.com/lumm2509/keel/config"
	"github.com/lumm2509/keel/transport/htmx"
)

type App struct{}

func main() {
	app := keel.New(keel.Config[App]{
		Context: &App{},
		Module:  &config.Config{},
	})

	// Full page load OR htmx partial — server decides which to return.
	app.GET("/", func(c *keel.Context[App]) error {
		if c.HTMX().IsHTMX() {
			// HTMX swap: return only the fragment the browser needs.
			return c.HTMX().
				PushURL("/").
				HTML(http.StatusOK, `<section id="content"><h1>Home</h1></section>`)
		}
		// Full page: return the shell with the content already inside.
		return c.HTML(http.StatusOK, fullPage(`<section id="content"><h1>Home</h1></section>`))
	})

	// Add a new item and update the list in place.
	//
	//  <form hx-post="/items" hx-target="#list" hx-swap="beforeend">
	//    <input name="name" /><button type="submit">Add</button>
	//  </form>
	app.POST("/items", func(c *keel.Context[App]) error {
		name := c.Request.FormValue("name")
		if name == "" {
			return keel.NewBadRequestError("name is required", nil)
		}

		fragment := fmt.Sprintf(`<li>%s</li>`, name)

		// Push the URL and fire an event so other components can react.
		return c.HTMX().
			PushURL("/items").
			Trigger("itemAdded").
			HTML(http.StatusOK, fragment)
	})

	// Delete an item. No DOM swap needed — just trigger a list refresh.
	//
	//  <button hx-delete="/items/42" hx-swap="none">Delete</button>
	app.DELETE("/items/{id}", func(c *keel.Context[App]) error {
		// ... delete from store ...

		// TriggerDetail sends structured data with the event.
		return c.HTMX().
			TriggerDetail(map[string]any{
				"itemDeleted": map[string]any{"id": c.Param("id")},
			}).
			NoContent()
	})

	// Retarget: the element triggers a swap on itself, but the server
	// redirects the swap to a different element and uses a different strategy.
	//
	//  <div hx-get="/alerts" hx-trigger="revealed">...</div>
	app.GET("/alerts", func(c *keel.Context[App]) error {
		return c.HTMX().
			Retarget("#global-alerts").
			Reswap(htmx.SwapAfterBegin).
			HTML(http.StatusOK, `<div class="alert">You have a new message.</div>`)
	})

	// Polling endpoint — returns 286 to stop the poll when done.
	//
	//  <div hx-get="/jobs/42/status" hx-trigger="every 2s">Polling…</div>
	app.GET("/jobs/{id}/status", func(c *keel.Context[App]) error {
		id := c.Param("id")
		done := jobIsDone(id)

		if done {
			// 286 stops HTMX polling and swaps in the final content.
			return c.HTMX().
				Trigger("jobCompleted").
				StopPolling()
		}

		return c.HTMX().HTML(http.StatusOK, `<span>still running…</span>`)
	})

	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}

// ── Helpers ──────────────────────────────────────────────────────────────────

func fullPage(content string) string {
	return fmt.Sprintf(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <title>HTMX example</title>
  <script src="https://unpkg.com/htmx.org@2" defer></script>
</head>
<body hx-boost="true">
  <nav>
    <a href="/">Home</a>
    <a href="/items">Items</a>
  </nav>
  <main id="content">%s</main>
  <div id="global-alerts"></div>
</body>
</html>`, content)
}

func jobIsDone(_ string) bool {
	// Stub: replace with a real status check.
	return false
}
