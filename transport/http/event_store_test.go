package http

import "testing"

func TestEventDataGetSet(t *testing.T) {
	t.Parallel()

	e := &EventData{}
	e.Set("k", "v")

	got := e.Get("k")
	if got != "v" {
		t.Fatalf("expected %q, got %v", "v", got)
	}
}

func TestEventDataGetAll(t *testing.T) {
	t.Parallel()

	e := &EventData{}
	e.Set("a", 1)
	e.Set("b", 2)

	all := e.GetAll()
	if len(all) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(all))
	}
}

func TestEventDataSetAll(t *testing.T) {
	t.Parallel()

	e := &EventData{}
	e.SetAll(map[string]any{"x": 10, "y": 20})

	if e.Get("x") != 10 || e.Get("y") != 20 {
		t.Fatalf("SetAll did not store values correctly")
	}
}
