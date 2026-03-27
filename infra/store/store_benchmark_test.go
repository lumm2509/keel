package store_test

import (
	"encoding/json"
	"strconv"
	"testing"

	"github.com/lumm2509/keel/infra/store"
)

func BenchmarkStoreMarshalJSON(b *testing.B) {
	s := store.New[string, map[string]any](nil)
	for i := 0; i < 1000; i++ {
		s.Set(strconv.Itoa(i), map[string]any{
			"id":    i,
			"title": "title",
			"flag":  i%2 == 0,
		})
	}

	b.Run("current", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			if _, err := s.MarshalJSON(); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("legacy", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			if _, err := legacyStoreMarshalJSON(s); err != nil {
				b.Fatal(err)
			}
		}
	})
}

func legacyStoreMarshalJSON[K comparable, T any](s *store.Store[K, T]) ([]byte, error) {
	return json.Marshal(s.GetAll())
}
