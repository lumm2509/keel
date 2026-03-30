package http

import (
	"testing"

	"github.com/lumm2509/keel/runtime/hook"
)

func h(id string, priority int) *hook.Handler[*hook.Event] {
	return &hook.Handler[*hook.Event]{Id: id, Priority: priority}
}

func ids(handlers []*hook.Handler[*hook.Event]) []string {
	out := make([]string, len(handlers))
	for i, hh := range handlers {
		out[i] = hh.Id
	}
	return out
}

func TestMergeIncludedHandlersNoPanic(t *testing.T) {
	excluded := map[string]struct{}{"a": {}, "b": {}}

	// All left items excluded, right empty — used to panic.
	left := []*hook.Handler[*hook.Event]{h("a", 0), h("b", 1)}
	result := MergeIncludedHandlers(left, excluded, nil, nil)
	if len(result) != 0 {
		t.Fatalf("expected empty result, got %v", ids(result))
	}

	// All right items excluded, left empty.
	right := []*hook.Handler[*hook.Event]{h("a", 0), h("b", 1)}
	result = MergeIncludedHandlers(nil, nil, right, excluded)
	if len(result) != 0 {
		t.Fatalf("expected empty result, got %v", ids(result))
	}

	// Both sides fully excluded.
	result = MergeIncludedHandlers(left, excluded, right, excluded)
	if len(result) != 0 {
		t.Fatalf("expected empty result, got %v", ids(result))
	}
}

func TestMergeIncludedHandlersPriorityOrder(t *testing.T) {
	left := []*hook.Handler[*hook.Event]{h("l1", 1), h("l3", 3)}
	right := []*hook.Handler[*hook.Event]{h("r2", 2), h("r4", 4)}

	result := MergeIncludedHandlers(left, nil, right, nil)
	want := []string{"l1", "r2", "l3", "r4"}
	got := ids(result)

	for i, id := range want {
		if got[i] != id {
			t.Fatalf("expected order %v, got %v", want, got)
		}
	}
}

func TestMergeIncludedHandlersExclusion(t *testing.T) {
	left := []*hook.Handler[*hook.Event]{h("l1", 1), h("l2", 2)}
	right := []*hook.Handler[*hook.Event]{h("r1", 1), h("r2", 2)}
	leftEx := map[string]struct{}{"l1": {}}
	rightEx := map[string]struct{}{"r2": {}}

	result := MergeIncludedHandlers(left, leftEx, right, rightEx)
	want := []string{"r1", "l2"}
	got := ids(result)

	if len(got) != len(want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
	for i, id := range want {
		if got[i] != id {
			t.Fatalf("expected order %v, got %v", want, got)
		}
	}
}
