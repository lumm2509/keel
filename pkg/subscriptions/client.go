package subscriptions

import (
	"encoding/json"
	"net/url"
	"sort"
	"strings"
	"sync"

	"github.com/lumm2509/keel/infra/security"
	"github.com/lumm2509/keel/pkg/inflector"
	"github.com/spf13/cast"
)

const optionsParam = "options"

// SubscriptionOptions defines the request options (query params, headers, etc.)
// for a single subscription topic.
type SubscriptionOptions struct {
	Query   map[string]string `json:"query"`
	Headers map[string]string `json:"headers"`
}

// Client is an interface for a generic subscription client.
type Client interface {
	// Id Returns the unique id of the client.
	Id() string

	// Channel returns the client's communication channel.
	//
	// NB! The channel shouldn't be used after calling Discard().
	Channel() chan Message

	// Subscriptions returns a shallow copy of the client subscriptions matching the prefixes.
	// If no prefix is specified, returns all subscriptions.
	Subscriptions(prefixes ...string) map[string]SubscriptionOptions

	// Subscribe subscribes the client to the provided subscriptions list.
	//
	// Each subscription can also have "options" (json serialized SubscriptionOptions) as query parameter.
	//
	// Example:
	//
	// 	Subscribe(
	// 	    "subscriptionA",
	// 	    `subscriptionB?options={"query":{"a":1},"headers":{"x_token":"abc"}}`,
	// 	)
	Subscribe(subs ...string)

	// Unsubscribe unsubscribes the client from the provided subscriptions list.
	Unsubscribe(subs ...string)

	// HasSubscription checks if the client is subscribed to `sub`.
	HasSubscription(sub string) bool

	// Set stores any value to the client's context.
	Set(key string, value any)

	// Unset removes a single value from the client's context.
	Unset(key string)

	// Get retrieves the key value from the client's context.
	Get(key string) any

	// Discard marks the client as "discarded" (and closes its channel),
	// meaning that it shouldn't be used anymore for sending new messages.
	//
	// It is safe to call Discard() multiple times.
	Discard()

	// IsDiscarded indicates whether the client has been "discarded"
	// and should no longer be used.
	IsDiscarded() bool

	// Send sends the specified message to the client's channel (if not discarded).
	Send(m Message)
}

// ensures that DefaultClient satisfies the Client interface
var _ Client = (*DefaultClient)(nil)

// DefaultClient defines a generic subscription client.
type DefaultClient struct {
	store         map[string]any
	subscriptions map[string]SubscriptionOptions
	topics        map[string]string
	topicSubs     map[string]map[string]SubscriptionOptions
	sortedTopics  []string
	topicsDirty   bool
	channel       chan Message
	id            string
	mu            sync.RWMutex
	isDiscarded   bool
}

// NewDefaultClient creates and returns a new DefaultClient instance.
func NewDefaultClient() *DefaultClient {
	return &DefaultClient{
		id:            security.RandomString(40),
		store:         map[string]any{},
		channel:       make(chan Message),
		subscriptions: map[string]SubscriptionOptions{},
		topics:        map[string]string{},
		topicSubs:     map[string]map[string]SubscriptionOptions{},
	}
}

// Id implements the [Client.Id] interface method.
func (c *DefaultClient) Id() string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.id
}

// Channel implements the [Client.Channel] interface method.
func (c *DefaultClient) Channel() chan Message {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.channel
}

// Subscriptions implements the [Client.Subscriptions] interface method.
//
// It returns a shallow copy of the client subscriptions matching the prefixes.
// If no prefix is specified, returns all subscriptions.
func (c *DefaultClient) Subscriptions(prefixes ...string) map[string]SubscriptionOptions {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// no prefix -> return copy of all subscriptions
	if len(prefixes) == 0 {
		result := make(map[string]SubscriptionOptions, len(c.subscriptions))

		for s, options := range c.subscriptions {
			result[s] = options
		}

		return result
	}

	result := make(map[string]SubscriptionOptions)
	topics := c.sortedTopicsSnapshot()

	for _, prefix := range prefixes {
		for _, topic := range matchingTopics(topics, prefix) {
			for sub, options := range c.topicSubs[topic] {
				result[sub] = options
			}
		}
	}

	return result
}

// Subscribe implements the [Client.Subscribe] interface method.
//
// Empty subscriptions (aka. "") are ignored.
func (c *DefaultClient) Subscribe(subs ...string) {
	type parsedSubscription struct {
		raw     string
		topic   string
		options SubscriptionOptions
	}

	parsed := make([]parsedSubscription, 0, len(subs))

	for _, s := range subs {
		if s == "" {
			continue // skip empty
		}

		topic, options := parseSubscription(s)
		parsed = append(parsed, parsedSubscription{
			raw:     s,
			topic:   topic,
			options: options,
		})
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	for _, sub := range parsed {
		oldTopic, existed := c.topics[sub.raw]
		if existed && oldTopic != sub.topic {
			c.removeTopicSubscription(oldTopic, sub.raw)
		}

		c.subscriptions[sub.raw] = sub.options
		c.topics[sub.raw] = sub.topic
		c.addTopicSubscription(sub.topic, sub.raw, sub.options)
	}
}

// Unsubscribe implements the [Client.Unsubscribe] interface method.
//
// If subs is not set, this method removes all registered client's subscriptions.
func (c *DefaultClient) Unsubscribe(subs ...string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(subs) > 0 {
		for _, s := range subs {
			c.removeTopicSubscription(c.topics[s], s)
			delete(c.subscriptions, s)
			delete(c.topics, s)
		}
	} else {
		// unsubscribe all
		for s := range c.subscriptions {
			c.removeTopicSubscription(c.topics[s], s)
			delete(c.subscriptions, s)
			delete(c.topics, s)
		}
	}
}

// HasSubscription implements the [Client.HasSubscription] interface method.
func (c *DefaultClient) HasSubscription(sub string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	_, ok := c.subscriptions[sub]

	return ok
}

// Get implements the [Client.Get] interface method.
func (c *DefaultClient) Get(key string) any {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.store[key]
}

// Set implements the [Client.Set] interface method.
func (c *DefaultClient) Set(key string, value any) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.store[key] = value
}

// Unset implements the [Client.Unset] interface method.
func (c *DefaultClient) Unset(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.store, key)
}

// Discard implements the [Client.Discard] interface method.
func (c *DefaultClient) Discard() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.isDiscarded {
		return
	}

	close(c.channel)

	c.isDiscarded = true
}

// IsDiscarded implements the [Client.IsDiscarded] interface method.
func (c *DefaultClient) IsDiscarded() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.isDiscarded
}

// Send sends the specified message to the client's channel (if not discarded).
func (c *DefaultClient) Send(m Message) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.isDiscarded {
		return
	}

	c.channel <- m
}

func parseSubscription(rawSub string) (string, SubscriptionOptions) {
	rawOptions := struct {
		// note: any instead of string to minimize the breaking changes with earlier versions
		Query   map[string]any `json:"query"`
		Headers map[string]any `json:"headers"`
	}{}

	topic := rawSub
	if u, err := url.Parse(rawSub); err == nil {
		topic = u.Path
		if topic == "" {
			topic = rawSub
		}

		raw := u.Query().Get(optionsParam)
		if raw != "" {
			_ = json.Unmarshal([]byte(raw), &rawOptions)
		}
	}

	options := SubscriptionOptions{
		Query:   make(map[string]string, len(rawOptions.Query)),
		Headers: make(map[string]string, len(rawOptions.Headers)),
	}

	// normalize query
	// (currently only single string values are supported for consistency with the default routes handling)
	for k, v := range rawOptions.Query {
		options.Query[k] = cast.ToString(v)
	}

	// normalize headers name and values, eg. "X-Token" is converted to "x_token"
	// (currently only single string values are supported for consistency with the default routes handling)
	for k, v := range rawOptions.Headers {
		options.Headers[inflector.Snakecase(k)] = cast.ToString(v)
	}

	return topic, options
}

func (c *DefaultClient) addTopicSubscription(topic string, raw string, options SubscriptionOptions) {
	if _, ok := c.topicSubs[topic]; !ok {
		c.topicSubs[topic] = map[string]SubscriptionOptions{}
		c.topicsDirty = true
	}

	c.topicSubs[topic][raw] = options
}

func (c *DefaultClient) removeTopicSubscription(topic string, raw string) {
	if topic == "" {
		return
	}

	subs, ok := c.topicSubs[topic]
	if !ok {
		return
	}

	delete(subs, raw)
	if len(subs) == 0 {
		delete(c.topicSubs, topic)
		c.topicsDirty = true
	}
}

func (c *DefaultClient) sortedTopicsSnapshot() []string {
	if c.topicsDirty {
		c.sortedTopics = c.sortedTopics[:0]
		for topic := range c.topicSubs {
			c.sortedTopics = append(c.sortedTopics, topic)
		}
		sort.Strings(c.sortedTopics)
		c.topicsDirty = false
	}

	return c.sortedTopics
}

func matchingTopics(topics []string, prefix string) []string {
	if prefix == "" {
		return topics
	}

	start := sort.Search(len(topics), func(i int) bool {
		return topics[i] >= prefix
	})

	end := start
	for end < len(topics) && strings.HasPrefix(topics[end], prefix) {
		end++
	}

	return topics[start:end]
}
