package http

// EventData is an embed that adds a per-event KV store.
// Events are single-goroutine per request; no mutex needed.
type EventData struct {
	data map[string]any
}

func (e *EventData) Get(key string) any {
	return e.data[key]
}

func (e *EventData) GetAll() map[string]any {
	clone := make(map[string]any, len(e.data))
	for k, v := range e.data {
		clone[k] = v
	}
	return clone
}

func (e *EventData) Set(key string, value any) {
	if e.data == nil {
		e.data = make(map[string]any)
	}
	e.data[key] = value
}

func (e *EventData) SetAll(m map[string]any) {
	if e.data == nil {
		e.data = make(map[string]any, len(m))
	}
	for k, v := range m {
		e.data[k] = v
	}
}
