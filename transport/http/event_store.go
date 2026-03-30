package http

import "github.com/lumm2509/keel/infra/store"

// EventData is an embed that adds a KV store per-event.
type EventData struct {
	data store.Store[string, any]
}

func (e *EventData) Get(key string) any {
	return e.data.Get(key)
}

func (e *EventData) GetAll() map[string]any {
	return e.data.GetAll()
}

func (e *EventData) Set(key string, value any) {
	e.data.Set(key, value)
}

func (e *EventData) SetAll(m map[string]any) {
	for k, v := range m {
		e.data.Set(k, v)
	}
}
