package picker

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/lumm2509/keel/pkg/search"
)

func BenchmarkPickSearchResult(b *testing.B) {
	items := make([]any, 0, 50)
	for i := 0; i < 50; i++ {
		items = append(items, map[string]any{
			"id":    i,
			"title": "title-" + strings.Repeat("x", 32),
			"body":  strings.Repeat("content-", 12),
			"meta": map[string]any{
				"a": i,
				"b": i * 2,
				"nested": map[string]any{
					"keep": "yes",
					"drop": "no",
				},
			},
		})
	}

	data := search.Result{
		Page:       1,
		PerPage:    50,
		TotalItems: 500,
		TotalPages: 10,
		Items:      items,
	}

	fields := "id,title,meta.nested.keep"

	b.Run("current", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			if _, err := Pick(data, fields); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("legacy", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			if _, err := legacyPick(data, fields); err != nil {
				b.Fatal(err)
			}
		}
	})
}

func legacyPick(data any, rawFields string) (any, error) {
	parsedFields, err := parseFields(rawFields)
	if err != nil {
		return nil, err
	}

	encoded, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	var decoded any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		return nil, err
	}

	var isSearchResult bool
	switch data.(type) {
	case search.Result, *search.Result:
		isSearchResult = true
	}

	if isSearchResult {
		if decodedMap, ok := decoded.(map[string]any); ok {
			legacyPickParsedFields(decodedMap["items"], parsedFields)
		}
	} else {
		legacyPickParsedFields(decoded, parsedFields)
	}

	return decoded, nil
}

func legacyPickParsedFields(data any, fields map[string]Modifier) error {
	switch v := data.(type) {
	case map[string]any:
		legacyPickMapFields(v, fields)
	case []map[string]any:
		for _, item := range v {
			if err := legacyPickMapFields(item, fields); err != nil {
				return err
			}
		}
	case []any:
		if len(v) == 0 {
			return nil
		}

		if _, ok := v[0].(map[string]any); !ok {
			return nil
		}

		for _, item := range v {
			if err := legacyPickMapFields(item.(map[string]any), fields); err != nil {
				return nil
			}
		}
	}

	return nil
}

func legacyPickMapFields(data map[string]any, fields map[string]Modifier) error {
	if len(fields) == 0 {
		return nil
	}

	if m, ok := fields["*"]; ok {
		for k := range data {
			var exists bool
			for f := range fields {
				if strings.HasPrefix(f+".", k+".") {
					exists = true
					break
				}
			}

			if !exists {
				fields[k] = m
			}
		}
	}

DataLoop:
	for k := range data {
		matchingFields := make(map[string]Modifier, len(fields))
		for f, m := range fields {
			if strings.HasPrefix(f+".", k+".") {
				matchingFields[f] = m
				continue
			}
		}

		if len(matchingFields) == 0 {
			delete(data, k)
			continue DataLoop
		}

		for f, m := range matchingFields {
			remains := strings.TrimSuffix(strings.TrimPrefix(f+".", k+"."), ".")
			if remains == "" {
				if m != nil {
					var err error
					data[k], err = m.Modify(data[k])
					if err != nil {
						return err
					}
				}
				continue DataLoop
			}

			delete(matchingFields, f)
			matchingFields[remains] = m
		}

		if err := legacyPickParsedFields(data[k], matchingFields); err != nil {
			return err
		}
	}

	return nil
}
