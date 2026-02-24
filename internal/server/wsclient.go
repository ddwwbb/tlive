package server

import (
	"sync"

	"github.com/gorilla/websocket"
)

type WSClient struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

func NewWSClient(conn *websocket.Conn) *WSClient {
	return &WSClient{conn: conn}
}

func (c *WSClient) Send(data []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.conn.WriteMessage(websocket.BinaryMessage, data)
}

func (c *WSClient) Close() error {
	return c.conn.Close()
}
