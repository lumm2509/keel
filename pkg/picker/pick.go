package picker

import (
	"encoding/json"
	"strings"

	"github.com/lumm2509/keel/infra/store"
	"github.com/lumm2509/keel/pkg/tokenizer"
)

// Result is the standard paginated search result returned by a Provider.
// It is defined here so that Pick can apply field selection directly to it
// without needing an external search package dependency.
type Result struct {
	Items      any `json:"items"`
	Page       int `json:"page"`
	PerPage    int `json:"perPage"`
	TotalItems int `json:"totalItems"`
	TotalPages int `json:"totalPages"`
}

var pickFieldTreeCache = store.New[string, *fieldTree](nil)

const maxCachedPickFieldTrees = 256

// Pick converts data into a []any, map[string]any, etc. (using json marshal->unmarshal)
// containing only the fields from the parsed rawFields expression.
//
// rawFields is a comma separated string of the fields to include.
// Nested fields should be listed with dot-notation.
// Fields value modifiers are also supported using the `:modifier(args)` format (see Modifiers).
//
// Example:
//
//	data := map[string]any{"a": 1, "b": 2, "c": map[string]any{"c1": 11, "c2": 22}}
//	Pick(data, "a,c.c1") // map[string]any{"a": 1, "c": map[string]any{"c1": 11}}
func Pick(data any, rawFields string) (any, error) {
	tree, err := parseFieldTree(rawFields)
	if err != nil {
		return nil, err
	}

	switch v := data.(type) {
	case map[string]any:
		return projectPickedValue(v, tree)
	case []map[string]any:
		return projectPickedValue(v, tree)
	case []any:
		return projectPickedValue(v, tree)
	case Result:
		cloned := v
		cloned.Items, err = normalizePickItems(v.Items)
		if err != nil {
			return nil, err
		}
		if err := pickParsedFields(cloned.Items, tree); err != nil {
			return nil, err
		}
		return cloned, nil
	case *Result:
		if v == nil {
			return nil, nil
		}
		cloned := *v
		cloned.Items, err = normalizePickItems(v.Items)
		if err != nil {
			return nil, err
		}
		if err := pickParsedFields(cloned.Items, tree); err != nil {
			return nil, err
		}
		return &cloned, nil
	}

	encoded, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	var decoded any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		return nil, err
	}

	if err := pickParsedFields(decoded, tree); err != nil {
		return nil, err
	}

	return decoded, nil
}

func projectPickedValue(data any, fields *fieldTree) (any, error) {
	if isNoopFieldTree(fields) {
		return cloneJSONLike(data), nil
	}

	switch v := data.(type) {
	case map[string]any:
		return projectPickedMap(v, fields)
	case []map[string]any:
		result := make([]map[string]any, len(v))
		for i := range v {
			item, err := projectPickedMap(v[i], fields)
			if err != nil {
				return nil, err
			}
			result[i] = item
		}
		return result, nil
	case []any:
		if len(v) == 0 {
			return []any{}, nil
		}

		if _, ok := v[0].(map[string]any); !ok {
			return cloneAnySlice(v), nil
		}

		result := make([]any, len(v))
		for i, item := range v {
			mapped, ok := item.(map[string]any)
			if !ok {
				result[i] = item
				continue
			}

			projected, err := projectPickedMap(mapped, fields)
			if err != nil {
				return nil, err
			}
			result[i] = projected
		}
		return result, nil
	default:
		if fields != nil && fields.modifier != nil {
			return fields.modifier.Modify(v)
		}
		return v, nil
	}
}

func projectPickedMap(data map[string]any, fields *fieldTree) (map[string]any, error) {
	if isNoopFieldTree(fields) {
		return cloneMap(data), nil
	}

	result := make(map[string]any, len(data))

	for key, value := range data {
		next := fields.children[key]
		if next == nil {
			next = fields.wildcard
		}
		if next == nil {
			continue
		}

		projected, err := projectPickedValue(value, next)
		if err != nil {
			return nil, err
		}
		result[key] = projected
	}

	return result, nil
}

func isNoopFieldTree(fields *fieldTree) bool {
	return fields == nil || (fields.modifier == nil && len(fields.children) == 0 && fields.wildcard == nil)
}

func parseFieldTree(rawFields string) (*fieldTree, error) {
	if cached, ok := pickFieldTreeCache.GetOk(rawFields); ok {
		return cached, nil
	}

	parsedFields, err := parseFields(rawFields)
	if err != nil {
		return nil, err
	}

	tree := newFieldTree(parsedFields)
	pickFieldTreeCache.SetIfLessThanLimit(rawFields, tree, maxCachedPickFieldTrees)
	return tree, nil
}

type fieldTree struct {
	modifier Modifier
	children map[string]*fieldTree
	wildcard *fieldTree
}

func newFieldTree(fields map[string]Modifier) *fieldTree {
	root := &fieldTree{children: make(map[string]*fieldTree, len(fields))}

	for path, modifier := range fields {
		node := root
		for _, part := range strings.Split(path, ".") {
			if part == "*" {
				if node.wildcard == nil {
					node.wildcard = &fieldTree{children: make(map[string]*fieldTree)}
				}
				node = node.wildcard
				continue
			}

			if node.children == nil {
				node.children = make(map[string]*fieldTree)
			}

			child, ok := node.children[part]
			if !ok {
				child = &fieldTree{children: make(map[string]*fieldTree)}
				node.children[part] = child
			}
			node = child
		}

		node.modifier = modifier
	}

	return root
}

func parseFields(rawFields string) (map[string]Modifier, error) {
	t := tokenizer.NewFromString(rawFields)

	fields, err := t.ScanAll()
	if err != nil {
		return nil, err
	}

	result := make(map[string]Modifier, len(fields))

	for _, f := range fields {
		parts := strings.SplitN(strings.TrimSpace(f), ":", 2)

		if len(parts) > 1 {
			m, err := initModifer(parts[1])
			if err != nil {
				return nil, err
			}
			result[parts[0]] = m
		} else {
			result[parts[0]] = nil
		}
	}

	return result, nil
}

func pickParsedFields(data any, fields *fieldTree) error {
	if fields == nil {
		return nil
	}

	switch v := data.(type) {
	case map[string]any:
		return pickMapFields(v, fields)
	case []map[string]any:
		for _, item := range v {
			if err := pickMapFields(item, fields); err != nil {
				return err
			}
		}
	case []any:
		if len(v) == 0 {
			return nil // nothing to pick
		}

		if _, ok := v[0].(map[string]any); !ok {
			return nil // for now ignore non-map values
		}

		for _, item := range v {
			if err := pickMapFields(item.(map[string]any), fields); err != nil {
				return err
			}
		}
	}

	return nil
}

func pickMapFields(data map[string]any, fields *fieldTree) error {
	if fields == nil || (fields.modifier == nil && len(fields.children) == 0 && fields.wildcard == nil) {
		return nil // nothing to pick
	}

	if fields.modifier != nil {
		return nil
	}

	for k := range data {
		next := fields.children[k]
		if next == nil {
			next = fields.wildcard
		}

		if next == nil {
			delete(data, k)
			continue
		}

		if next.modifier != nil {
			var err error
			data[k], err = next.modifier.Modify(data[k])
			if err != nil {
				return err
			}
			continue
		}

		if err := pickParsedFields(data[k], next); err != nil {
			return err
		}
	}

	return nil
}

func cloneMap(src map[string]any) map[string]any {
	dst := make(map[string]any, len(src))
	for k, v := range src {
		dst[k] = cloneJSONLike(v)
	}
	return dst
}

func cloneMapSlice(src []map[string]any) []map[string]any {
	dst := make([]map[string]any, len(src))
	for i := range src {
		dst[i] = cloneMap(src[i])
	}
	return dst
}

func cloneAnySlice(src []any) []any {
	dst := make([]any, len(src))
	for i, v := range src {
		dst[i] = cloneJSONLike(v)
	}
	return dst
}

func cloneJSONLike(value any) any {
	switch v := value.(type) {
	case map[string]any:
		return cloneMap(v)
	case []map[string]any:
		return cloneMapSlice(v)
	case []any:
		return cloneAnySlice(v)
	default:
		return v
	}
}

func normalizePickItems(items any) (any, error) {
	switch v := items.(type) {
	case map[string]any, []map[string]any, []any:
		return cloneJSONLike(v), nil
	default:
		encoded, err := json.Marshal(v)
		if err != nil {
			return nil, err
		}

		var decoded any
		if err := json.Unmarshal(encoded, &decoded); err != nil {
			return nil, err
		}

		return decoded, nil
	}
}
