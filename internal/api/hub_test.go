package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func TestHubBroadcast(t *testing.T) {
	hub := NewHub()
	srv := httptest.NewServer(http.HandlerFunc(hub.ServeWS))
	defer srv.Close()
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	// Wait for the server to register the client.
	deadline := time.Now().Add(2 * time.Second)
	for hub.Clients() == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if hub.Clients() != 1 {
		t.Fatalf("Clients = %d, want 1", hub.Clients())
	}

	hub.Broadcast([]byte(`{"type":"event","event":{"direction":"up"}}`))

	_, data, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	var msg WSMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if msg.Type != "event" || msg.Event == nil || msg.Event.Direction != "up" {
		t.Errorf("broadcast message = %+v", msg)
	}
}

func TestHubBroadcastNoClientsIsNoop(t *testing.T) {
	hub := NewHub()
	hub.Broadcast([]byte(`{}`)) // must not panic or block
	if hub.Clients() != 0 {
		t.Errorf("Clients = %d, want 0", hub.Clients())
	}
}
