package subscriptions_test

import (
	"strconv"
	"strings"
	"testing"

	"github.com/lumm2509/keel/pkg/subscriptions"
)

func BenchmarkClientSubscriptionsByPrefix(b *testing.B) {
	c := subscriptions.NewDefaultClient()
	legacySubs := make(map[string]subscriptions.SubscriptionOptions)
	legacyTopics := make(map[string]string)

	for i := 0; i < 2000; i++ {
		topic := "topic/" + strconv.Itoa(i%200)
		raw := topic + "/sub/" + strconv.Itoa(i)
		c.Subscribe(raw)
		legacySubs[raw] = subscriptions.SubscriptionOptions{
			Query:   map[string]string{},
			Headers: map[string]string{},
		}
		legacyTopics[raw] = topic + "/sub/" + strconv.Itoa(i)
	}

	prefixes := []string{"topic/1", "topic/12", "topic/123"}

	b.Run("current", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			subs := c.Subscriptions(prefixes...)
			if len(subs) == 0 {
				b.Fatal("expected matching subscriptions")
			}
		}
	})

	b.Run("legacy", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			subs := legacySubscriptionsByPrefix(legacySubs, legacyTopics, prefixes...)
			if len(subs) == 0 {
				b.Fatal("expected matching subscriptions")
			}
		}
	})
}

func legacySubscriptionsByPrefix(
	subs map[string]subscriptions.SubscriptionOptions,
	topics map[string]string,
	prefixes ...string,
) map[string]subscriptions.SubscriptionOptions {
	result := make(map[string]subscriptions.SubscriptionOptions)

	for _, prefix := range prefixes {
		for sub, options := range subs {
			if strings.HasPrefix(topics[sub], prefix) {
				result[sub] = options
			}
		}
	}

	return result
}
