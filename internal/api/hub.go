package api

import (
	"context"
	"net/http"
	"sync"

	"github.com/coder/websocket"
)

// Hub is the WebSocket broadcast hub for the live event feed. Domain events are
// published to it and fanned out to every connected browser client. Slow
// clients drop messages rather than blocking the publisher.
type Hub struct {
	mu      sync.RWMutex
	clients map[*wsClient]struct{}
}

type wsClient struct {
	send chan []byte
}

// NewHub creates an empty hub.
func NewHub() *Hub {
	return &Hub{clients: make(map[*wsClient]struct{})}
}

// ServeWS upgrades an HTTP request to a WebSocket and serves the client until it
// disconnects. It is mounted at /ws.
func (h *Hub) ServeWS(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	cl := &wsClient{send: make(chan []byte, 64)}
	h.add(cl)
	defer h.remove(cl)

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case msg := <-cl.send:
				if err := conn.Write(ctx, websocket.MessageText, msg); err != nil {
					cancel()
					return
				}
			}
		}
	}()

	// Block reading until the client disconnects (we ignore inbound messages).
	for {
		if _, _, err := conn.Read(ctx); err != nil {
			return
		}
	}
}

// Broadcast sends msg to every connected client. It never blocks: a client whose
// buffer is full drops this message.
func (h *Hub) Broadcast(msg []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for cl := range h.clients {
		select {
		case cl.send <- msg:
		default:
		}
	}
}

// Clients returns the number of connected clients.
func (h *Hub) Clients() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

func (h *Hub) add(cl *wsClient) {
	h.mu.Lock()
	h.clients[cl] = struct{}{}
	h.mu.Unlock()
}

func (h *Hub) remove(cl *wsClient) {
	h.mu.Lock()
	delete(h.clients, cl)
	h.mu.Unlock()
}
