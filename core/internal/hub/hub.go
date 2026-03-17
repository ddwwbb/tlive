package hub

import (
	"sync"
	"sync/atomic"
)

// Client is the interface that hub consumers must implement.
type Client interface {
	Send(data []byte) error
}

// Hub is a concurrent broadcast center that distributes messages
// to all registered clients and routes input to a handler.
type Hub struct {
	broadcast    chan []byte
	inputCh      chan []byte
	stop         chan struct{}
	inputHandler func([]byte)
	handlerMu    sync.RWMutex
	clients      map[Client]struct{}
	clientsMu    sync.RWMutex
	clientCount  atomic.Int32
}

// New creates a new Hub instance.
func New() *Hub {
	return &Hub{
		broadcast: make(chan []byte, 256),
		inputCh:   make(chan []byte, 64),
		stop:      make(chan struct{}),
		clients:   make(map[Client]struct{}),
	}
}

// Run starts the hub event loop. It should be called in a goroutine.
func (h *Hub) Run() {
	for {
		select {
		case data := <-h.broadcast:
			h.clientsMu.RLock()
			for c := range h.clients {
				c.Send(data)
			}
			h.clientsMu.RUnlock()
		case data := <-h.inputCh:
			h.handlerMu.RLock()
			handler := h.inputHandler
			h.handlerMu.RUnlock()
			if handler != nil {
				handler(data)
			}
		case <-h.stop:
			return
		}
	}
}

// Register adds a client to the hub.
func (h *Hub) Register(c Client) {
	h.clientsMu.Lock()
	h.clients[c] = struct{}{}
	h.clientCount.Store(int32(len(h.clients)))
	h.clientsMu.Unlock()
}

// Unregister removes a client from the hub.
func (h *Hub) Unregister(c Client) {
	h.clientsMu.Lock()
	delete(h.clients, c)
	h.clientCount.Store(int32(len(h.clients)))
	h.clientsMu.Unlock()
}

// Broadcast sends data to all registered clients.
func (h *Hub) Broadcast(data []byte) { h.broadcast <- data }

// Input sends input data to the input handler.
func (h *Hub) Input(data []byte) { h.inputCh <- data }

// SetInputHandler sets the function that handles input data.
func (h *Hub) SetInputHandler(fn func([]byte)) {
	h.handlerMu.Lock()
	defer h.handlerMu.Unlock()
	h.inputHandler = fn
}

// Stop signals the hub to stop its event loop.
func (h *Hub) Stop() { close(h.stop) }

// ClientCount returns the current number of registered clients.
func (h *Hub) ClientCount() int { return int(h.clientCount.Load()) }
