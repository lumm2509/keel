package search

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"github.com/spf13/cast"
)

func BenchmarkReplaceFilterPlaceholders(b *testing.B) {
	raw := FilterData(strings.Repeat("(title ~ {:q} && status = {:status} && total >= {:min}) || ", 8) + "name != {:name}")
	replacements := Params{
		"q":      "abc",
		"status": true,
		"min":    42,
		"name":   "zzz",
	}

	b.Run("current", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			rs := make(map[string]string, len(replacements))
			for k, v := range replacements {
				rs[k] = stringifyFilterReplacement(v)
			}
			_ = replaceFilterPlaceholders(string(raw), rs)
		}
	})

	b.Run("legacy", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_ = legacyReplaceFilterPlaceholders(string(raw), replacements)
		}
	})
}

func legacyReplaceFilterPlaceholders(raw string, replacements Params) string {
	for key, value := range replacements {
		var replacement string
		switch v := value.(type) {
		case nil:
			replacement = "null"
		case bool, float64, float32, int, int64, int32, int16, int8, uint, uint64, uint32, uint16, uint8:
			replacement = cast.ToString(v)
		default:
			replacement = cast.ToString(v)
			if replacement == "" {
				encoded, _ := json.Marshal(v)
				replacement = string(encoded)
			}
			replacement = strconv.Quote(replacement)
		}
		raw = strings.ReplaceAll(raw, "{:"+key+"}", replacement)
	}

	return raw
}
