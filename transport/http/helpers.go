package http

import "github.com/lumm2509/keel/runtime/hook"

// AppendSortedHandlers inserts handlers into dst maintaining ascending Priority order.
func AppendSortedHandlers[T hook.Resolver](dst []*hook.Handler[T], handlers ...*hook.Handler[T]) []*hook.Handler[T] {
	for _, handler := range handlers {
		insertAt := len(dst)
		for i, existing := range dst {
			if existing.Priority > handler.Priority {
				insertAt = i
				break
			}
		}

		dst = append(dst, nil)
		copy(dst[insertAt+1:], dst[insertAt:])
		dst[insertAt] = handler
	}

	return dst
}

// MergeIncludedHandlers merges two sorted handler slices, skipping any handler
// whose Id appears in the corresponding excluded map.
func MergeIncludedHandlers[T hook.Resolver](left []*hook.Handler[T], leftExcluded map[string]struct{}, right []*hook.Handler[T], rightExcluded map[string]struct{}) []*hook.Handler[T] {
	result := make([]*hook.Handler[T], 0, len(left)+len(right))

	i, j := 0, 0
	for i < len(left) || j < len(right) {
		for i < len(left) {
			if _, ok := leftExcluded[left[i].Id]; ok {
				i++
				continue
			}
			break
		}

		for j < len(right) {
			if _, ok := rightExcluded[right[j].Id]; ok {
				j++
				continue
			}
			break
		}

		switch {
		case i >= len(left):
			result = append(result, right[j])
			j++
		case j >= len(right):
			result = append(result, left[i])
			i++
		case left[i].Priority <= right[j].Priority:
			result = append(result, left[i])
			i++
		default:
			result = append(result, right[j])
			j++
		}
	}

	return result
}
