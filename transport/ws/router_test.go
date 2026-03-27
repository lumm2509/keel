package ws

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/lumm2509/keel/runtime/hook"
	"golang.org/x/net/websocket"
)

type testEvent struct {
	Event
	trace *[]string
}

func newTestEvent(conn *websocket.Conn, req *http.Request) (*testEvent, EventCleanupFunc) {
	trace := []string{}
	return &testEvent{
		Event: Event{
			Conn:    conn,
			Request: req,
		},
		trace: &trace,
	}, nil
}

type hijackableResponseWriter struct {
	conn   net.Conn
	header http.Header
}

func (rw *hijackableResponseWriter) Header() http.Header {
	if rw.header == nil {
		rw.header = http.Header{}
	}

	return rw.header
}

func (rw *hijackableResponseWriter) Write(data []byte) (int, error) {
	return rw.conn.Write(data)
}

func (rw *hijackableResponseWriter) WriteHeader(_ int) {}

func (rw *hijackableResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return rw.conn, bufio.NewReadWriter(bufio.NewReader(rw.conn), bufio.NewWriter(rw.conn)), nil
}

type wsSession struct {
	client net.Conn
	reader *bufio.Reader
	done   chan struct{}
}

func newTestSession(t *testing.T, handler http.Handler, path string) *wsSession {
	t.Helper()

	serverConn, clientConn := net.Pipe()

	req := httptest.NewRequest(http.MethodGet, "http://example.com"+path, nil)
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Origin", "http://example.com")
	req.Header.Set("Sec-WebSocket-Version", "13")
	req.Header.Set("Sec-WebSocket-Key", base64.StdEncoding.EncodeToString([]byte("0123456789abcdef")))

	done := make(chan struct{})

	go func() {
		defer close(done)
		handler.ServeHTTP(&hijackableResponseWriter{conn: serverConn}, req)
	}()

	_ = clientConn.SetDeadline(time.Now().Add(2 * time.Second))

	reader := bufio.NewReader(clientConn)
	resp, err := http.ReadResponse(reader, req)
	if err != nil {
		t.Fatalf("read handshake response: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusSwitchingProtocols {
		t.Fatalf("expected websocket upgrade status %d, got %d", http.StatusSwitchingProtocols, resp.StatusCode)
	}

	return &wsSession{
		client: clientConn,
		reader: reader,
		done:   done,
	}
}

func (s *wsSession) Close() {
	_ = s.client.Close()
	<-s.done
}

func writeClientFrame(t *testing.T, conn net.Conn, opcode byte, payload []byte) {
	t.Helper()

	mask := [4]byte{1, 2, 3, 4}
	frame := make([]byte, 0, 2+len(mask)+len(payload))
	frame = append(frame, 0x80|opcode, 0x80|byte(len(payload)))
	frame = append(frame, mask[:]...)

	for i, b := range payload {
		frame = append(frame, b^mask[i%len(mask)])
	}

	if _, err := conn.Write(frame); err != nil {
		t.Fatalf("write client frame: %v", err)
	}
}

func readServerFrame(t *testing.T, reader *bufio.Reader) (byte, []byte) {
	t.Helper()

	header := make([]byte, 2)
	if _, err := io.ReadFull(reader, header); err != nil {
		t.Fatalf("read frame header: %v", err)
	}

	opcode := header[0] & 0x0f
	payloadLen := int(header[1] & 0x7f)
	if header[1]&0x80 != 0 {
		t.Fatalf("expected unmasked server frame")
	}

	payload := make([]byte, payloadLen)
	if _, err := io.ReadFull(reader, payload); err != nil {
		t.Fatalf("read frame payload: %v", err)
	}

	return opcode, payload
}

func TestRouterBuildHandler(t *testing.T) {
	router := NewRouter(newTestEvent)

	rootMiddleware := &hook.Handler[*testEvent]{
		Id: "root",
		Func: func(e *testEvent) error {
			*e.trace = append(*e.trace, "root")
			return e.Next()
		},
	}

	api := router.Group("/api")
	api.Bind(rootMiddleware)

	route := api.Route("/chat", func(e *testEvent) error {
		*e.trace = append(*e.trace, "action")
		return e.WriteJSON(map[string]any{
			"trace": *e.trace,
			"path":  e.Request.URL.Path,
		})
	})
	route.BindFunc(func(e *testEvent) error {
		*e.trace = append(*e.trace, "route")
		return e.Next()
	})

	handler, err := router.BuildHandler()
	if err != nil {
		t.Fatalf("BuildHandler() error = %v", err)
	}

	session := newTestSession(t, handler, "/api/chat")
	defer session.Close()

	opcode, payload := readServerFrame(t, session.reader)
	if opcode != websocket.TextFrame {
		t.Fatalf("expected text frame opcode %d, got %d", websocket.TextFrame, opcode)
	}

	var msg struct {
		Path  string   `json:"path"`
		Trace []string `json:"trace"`
	}
	if err := json.Unmarshal(payload, &msg); err != nil {
		t.Fatalf("unmarshal json payload: %v", err)
	}

	if msg.Path != "/api/chat" {
		t.Fatalf("expected path %q, got %q", "/api/chat", msg.Path)
	}

	expectedTrace := []string{"root", "route", "action"}
	if !reflect.DeepEqual(expectedTrace, msg.Trace) {
		t.Fatalf("expected trace %v, got %v", expectedTrace, msg.Trace)
	}
}

func TestRouterErrorHandler(t *testing.T) {
	router := NewRouter(newTestEvent)
	router.Route("/err", func(e *testEvent) error {
		return errors.New("boom")
	})

	handler, err := router.BuildHandler()
	if err != nil {
		t.Fatalf("BuildHandler() error = %v", err)
	}

	session := newTestSession(t, handler, "/err")
	defer session.Close()

	_, payload := readServerFrame(t, session.reader)

	var msg map[string]any
	if err := json.Unmarshal(payload, &msg); err != nil {
		t.Fatalf("unmarshal error payload: %v", err)
	}

	if msg["message"] != "boom" {
		t.Fatalf("expected error message %q, got %v", "boom", msg["message"])
	}
}

func TestRouterGroupUnbind(t *testing.T) {
	router := NewRouter(newTestEvent)

	group := router.Group("/api")
	group.Bind(
		&hook.Handler[*testEvent]{
			Id: "skip",
			Func: func(e *testEvent) error {
				*e.trace = append(*e.trace, "skip")
				return e.Next()
			},
		},
		&hook.Handler[*testEvent]{
			Id: "keep",
			Func: func(e *testEvent) error {
				*e.trace = append(*e.trace, "keep")
				return e.Next()
			},
		},
	)

	route := group.Route("/chat", func(e *testEvent) error {
		return e.WriteJSON(*e.trace)
	})
	route.Unbind("skip")

	handler, err := router.BuildHandler()
	if err != nil {
		t.Fatalf("BuildHandler() error = %v", err)
	}

	session := newTestSession(t, handler, "/api/chat")
	defer session.Close()

	_, payload := readServerFrame(t, session.reader)

	var trace []string
	if err := json.Unmarshal(payload, &trace); err != nil {
		t.Fatalf("unmarshal trace payload: %v", err)
	}

	expected := []string{"keep"}
	if !reflect.DeepEqual(expected, trace) {
		t.Fatalf("expected trace %v, got %v", expected, trace)
	}
}

func TestRouterHasRoute(t *testing.T) {
	router := NewRouter(newTestEvent)
	router.Group("/api").Route("/events/{topic...}", func(e *testEvent) error {
		return nil
	})

	if !router.HasRoute("/api/events/") {
		t.Fatalf("expected wildcard route to be found")
	}
}

func TestEventReadWriteMessage(t *testing.T) {
	router := NewRouter(func(conn *websocket.Conn, req *http.Request) (*Event, EventCleanupFunc) {
		return &Event{Conn: conn, Request: req}, nil
	})
	router.Route("/echo", func(e *Event) error {
		msg, err := e.ReadMessage()
		if err != nil {
			return err
		}

		return e.WriteMessage(append([]byte("echo:"), msg...))
	})

	handler, err := router.BuildHandler()
	if err != nil {
		t.Fatalf("BuildHandler() error = %v", err)
	}

	session := newTestSession(t, handler, "/echo")
	defer session.Close()

	writeClientFrame(t, session.client, websocket.BinaryFrame, []byte("hello"))
	opcode, msg := readServerFrame(t, session.reader)
	if opcode != websocket.BinaryFrame {
		t.Fatalf("expected binary frame opcode %d, got %d", websocket.BinaryFrame, opcode)
	}

	if string(msg) != "echo:hello" {
		t.Fatalf("expected message %q, got %q", "echo:hello", string(msg))
	}
}
