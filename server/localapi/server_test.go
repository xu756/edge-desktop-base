package localapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/wailsapp/wails/v3/pkg/application"
)

func testSocket(t *testing.T) *websocket.Conn {
	t.Helper()
	router := gin.New()
	server := New("127.0.0.1:0", "1.2.3")
	router.GET("/ws", server.handleWebSocket)
	host := httptest.NewServer(router)
	t.Cleanup(host.Close)
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(host.URL, "http")+"/ws", nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	var ready struct {
		Type    string
		Payload struct{ Version string }
	}
	if err := conn.ReadJSON(&ready); err != nil {
		t.Fatal(err)
	}
	if ready.Type != "runtime.ready" || ready.Payload.Version != "1.2.3" {
		t.Fatal(ready)
	}
	return conn
}

func TestWebSocketEcho(t *testing.T) {
	conn := testSocket(t)
	message := map[string]any{"type": "ping", "payload": map[string]any{"message": "hello"}}
	if err := conn.WriteJSON(message); err != nil {
		t.Fatal(err)
	}
	var echo struct {
		Type    string
		Payload map[string]any
	}
	if err := conn.ReadJSON(&echo); err != nil {
		t.Fatal(err)
	}
	got, _ := json.Marshal(echo.Payload)
	want, _ := json.Marshal(message)
	if echo.Type != "echo" || string(got) != string(want) {
		t.Fatal(echo)
	}
}

func TestWebSocketMessageLimit(t *testing.T) {
	conn := testSocket(t)
	// Use valid JSON so decoding reaches the message limit rather than rejecting syntax.
	// The peer can close as soon as it reads the frame header, before WriteMessage finishes.
	_ = conn.WriteJSON(strings.Repeat("x", maxWebSocketMessageSize+1))
	_, _, err := conn.ReadMessage()
	if !websocket.IsCloseError(err, websocket.CloseMessageTooBig) {
		t.Fatalf("expected message-too-big close, got %v", err)
	}
}

func TestLocalServerLifecycle(t *testing.T) {
	s := New("127.0.0.1:0", "1.2.3")
	if err := s.ServiceStartup(context.Background(), application.ServiceOptions{}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.ServiceShutdown() })
	status := s.Status()
	if !status.Running || strings.HasSuffix(status.Address, ":0") {
		t.Fatal(status)
	}
	response, err := http.Get("http://" + status.Address + "/health")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var health struct {
		OK      bool
		Version string
	}
	if err := json.NewDecoder(response.Body).Decode(&health); err != nil {
		t.Fatal(err)
	}
	if !health.OK || health.Version != "1.2.3" {
		t.Fatal(health)
	}
	if s.WebSocketURL() != "ws://"+status.Address+"/ws" {
		t.Fatal(s.WebSocketURL())
	}
	if err := s.ServiceShutdown(); err != nil {
		t.Fatal(err)
	}
}
