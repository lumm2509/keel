package http

import (
	"encoding"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"
)

type benchmarkBindStruct struct {
	Title   string    `form:"title"`
	Count   int       `form:"count"`
	Active  bool      `form:"active"`
	Tags    []string  `form:"tags"`
	Numbers []int     `form:"numbers"`
	When    time.Time `form:"when"`
	Nested  struct {
		Name  string `form:"name"`
		Total int    `form:"total"`
	} `form:"nested"`
}

func BenchmarkUnmarshalRequestData(b *testing.B) {
	data := map[string][]string{
		"title":        {"test title"},
		"count":        {"42"},
		"active":       {"true"},
		"tags":         {"a", "b", "c", "d"},
		"numbers":      {"1", "2", "3", "4"},
		"when":         {"2009-11-10T15:00:00Z"},
		"nested.name":  {"nested"},
		"nested.total": {"99"},
		"@jsonPayload": {`{"extra":"value"}`},
	}

	b.Run("current", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			dst := struct {
				benchmarkBindStruct
				Extra string `json:"extra"`
			}{}
			if err := UnmarshalRequestData(data, &dst, "form", ""); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("legacy", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			dst := struct {
				benchmarkBindStruct
				Extra string `json:"extra"`
			}{}
			if err := legacyUnmarshalRequestData(data, &dst, "form", ""); err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkUnmarshalRequestDataMixedJSONPayloadWins(b *testing.B) {
	data := map[string][]string{
		"title":        {"test title"},
		"count":        {"42"},
		"active":       {"true"},
		"tags":         {"a", "b", "c", "d"},
		"numbers":      {"1", "2", "3", "4"},
		"when":         {"2009-11-10T15:00:00Z"},
		"nested.name":  {"nested"},
		"nested.total": {"99"},
		"@jsonPayload": {`{"extra":"value"}`},
	}

	b.Run("current", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			dst := struct {
				benchmarkBindStruct
				Extra string `json:"extra"`
			}{}
			if err := UnmarshalRequestData(data, &dst, "form", ""); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("legacy", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			dst := struct {
				benchmarkBindStruct
				Extra string `json:"extra"`
			}{}
			if err := legacyUnmarshalRequestData(data, &dst, "form", ""); err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkUnmarshalRequestDataJSONPayloadOnly(b *testing.B) {
	data := map[string][]string{
		"@jsonPayload": {`{"title":"test title","count":42,"active":true,"extra":"value","nested":{"name":"nested","total":99}}`},
	}

	b.Run("current", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			dst := struct {
				Title  string `json:"title"`
				Count  int    `json:"count"`
				Active bool   `json:"active"`
				Extra  string `json:"extra"`
				Nested struct {
					Name  string `json:"name"`
					Total int    `json:"total"`
				} `json:"nested"`
			}{}
			if err := UnmarshalRequestData(data, &dst, "form", ""); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("legacy", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			dst := struct {
				Title  string `json:"title"`
				Count  int    `json:"count"`
				Active bool   `json:"active"`
				Extra  string `json:"extra"`
				Nested struct {
					Name  string `json:"name"`
					Total int    `json:"total"`
				} `json:"nested"`
			}{}
			if err := legacyUnmarshalRequestData(data, &dst, "form", ""); err != nil {
				b.Fatal(err)
			}
		}
	})
}

func legacyUnmarshalRequestData(data map[string][]string, dst any, structTagKey string, structPrefix string) error {
	if len(data) == 0 {
		return nil
	}

	dstValue := reflect.ValueOf(dst)
	if dstValue.Kind() != reflect.Pointer {
		return errors.New("dst must be a pointer")
	}

	dstValue = dereference(dstValue)
	dstType := dstValue.Type()

	switch dstType.Kind() {
	case reflect.Map:
		if dstType.Elem().Kind() != reflect.Interface {
			return errors.New("dst map value type must be any/interface{}")
		}

		for k, v := range data {
			if k == JSONPayloadKey {
				continue
			}

			if len(v) == 1 {
				dstValue.SetMapIndex(reflect.ValueOf(k), reflect.ValueOf(inferValue(v[0])))
			} else {
				normalized := make([]any, len(v))
				for i, vItem := range v {
					normalized[i] = inferValue(vItem)
				}
				dstValue.SetMapIndex(reflect.ValueOf(k), reflect.ValueOf(normalized))
			}
		}
	case reflect.Struct:
		if structTagKey == "" {
			structTagKey = "form"
		}

		if err := legacyUnmarshalInStructValue(data, dstValue, structTagKey, structPrefix); err != nil {
			return err
		}
	default:
		return errors.New("dst must be a map[string]any or struct")
	}

	for _, payload := range data[JSONPayloadKey] {
		if err := json.Unmarshal([]byte(payload), dst); err != nil {
			return err
		}
	}

	return nil
}

func legacyUnmarshalInStructValue(data map[string][]string, dstStructValue reflect.Value, structTagKey string, structPrefix string) error {
	dstStructType := dstStructValue.Type()

	for i := 0; i < dstStructValue.NumField(); i++ {
		fieldType := dstStructType.Field(i)
		tag := fieldType.Tag.Get(structTagKey)

		if tag == "-" || (!fieldType.Anonymous && !fieldType.IsExported()) {
			continue
		}

		fieldValue := dereference(dstStructValue.Field(i))

		ft := fieldType.Type
		if ft.Kind() == reflect.Ptr {
			ft = ft.Elem()
		}

		isSlice := ft.Kind() == reflect.Slice
		if isSlice {
			ft = ft.Elem()
		}

		name := tag
		if name == "" && !fieldType.Anonymous {
			name = fieldType.Name
		}
		if name != "" && structPrefix != "" {
			name = structPrefix + "." + name
		}

		if ft.Implements(textUnmarshalerType) || reflect.PointerTo(ft).Implements(textUnmarshalerType) {
			values, ok := data[name]
			if !ok || len(values) == 0 || !fieldValue.CanSet() {
				continue
			}

			if isSlice {
				n := len(values)
				slice := reflect.MakeSlice(fieldValue.Type(), n, n)
				for i, v := range values {
					unmarshaler, ok := dereference(slice.Index(i)).Addr().Interface().(encoding.TextUnmarshaler)
					if ok {
						if err := unmarshaler.UnmarshalText([]byte(v)); err != nil {
							return err
						}
					}
				}
				fieldValue.Set(slice)
			} else {
				unmarshaler, ok := fieldValue.Addr().Interface().(encoding.TextUnmarshaler)
				if ok {
					if err := unmarshaler.UnmarshalText([]byte(values[0])); err != nil {
						return err
					}
				}
			}
			continue
		}

		if ft.Kind() != reflect.Struct {
			values, ok := data[name]
			if !ok || len(values) == 0 || !fieldValue.CanSet() {
				continue
			}

			if isSlice {
				n := len(values)
				slice := reflect.MakeSlice(fieldValue.Type(), n, n)
				for i, v := range values {
					if err := setRegularReflectedValue(dereference(slice.Index(i)), v); err != nil {
						return err
					}
				}
				fieldValue.Set(slice)
			} else {
				if err := setRegularReflectedValue(fieldValue, values[0]); err != nil {
					return err
				}
			}
			continue
		}

		if isSlice {
			continue
		}

		if tag != "" {
			structPrefix = tag
		} else {
			structPrefix = name
		}

		if err := legacyUnmarshalInStructValue(data, fieldValue, structTagKey, structPrefix); err != nil {
			return err
		}
	}

	return nil
}
