package http

import (
	"io"
	"strings"
	"testing"
)

func BenchmarkRereadableReadCloserReadOnce(b *testing.B) {
	payload := strings.Repeat("abcdef0123456789", 256)

	b.Run("lazy_no_replay", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			r := &RereadableReadCloser{
				ReadCloser: io.NopCloser(strings.NewReader(payload)),
				MaxMemory:  1024,
				Lazy:       true,
			}
			if _, err := io.ReadAll(r); err != nil {
				b.Fatal(err)
			}
			if err := r.Close(); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("legacy_eager", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			r := &RereadableReadCloser{
				ReadCloser: io.NopCloser(strings.NewReader(payload)),
				MaxMemory:  1024,
			}
			if _, err := io.ReadAll(r); err != nil {
				b.Fatal(err)
			}
			if err := r.Close(); err != nil {
				b.Fatal(err)
			}
		}
	})
}
