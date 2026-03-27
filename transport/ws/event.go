package ws

import (
	"encoding/json"
	"net/http"

	"github.com/lumm2509/keel/infra/store"
	"github.com/lumm2509/keel/runtime/hook"
	"golang.org/x/net/websocket"
)

type Event struct {
	Conn    *websocket.Conn
	Request *http.Request

	hook.Event

	data store.Store[string, any]
}

func (e *Event) ReadJSON(dst any) error {
	return websocket.JSON.Receive(e.Conn, dst)
}

func (e *Event) WriteJSON(v any) error {
	return websocket.JSON.Send(e.Conn, v)
}

func (e *Event) ReadMessage() ([]byte, error) {
	var msg []byte
	err := websocket.Message.Receive(e.Conn, &msg)
	return msg, err
}

func (e *Event) WriteMessage(msg []byte) error {
	return websocket.Message.Send(e.Conn, msg)
}

func (e *Event) MarshalJSON(v any) ([]byte, error) {
	return json.Marshal(v)
}

func (e *Event) Close() error {
	return e.Conn.Close()
}

func (e *Event) Get(key string) any {
	return e.data.Get(key)
}

func (e *Event) GetAll() map[string]any {
	return e.data.GetAll()
}

func (e *Event) Set(key string, value any) {
	e.data.Set(key, value)
}

func (e *Event) SetAll(m map[string]any) {
	for k, v := range m {
		e.Set(k, v)
	}
}
