package hub

import (
	"sync"
	"testing"
	"time"
)

type mockClient struct {
	mu       sync.Mutex
	received [][]byte
}

func (m *mockClient) Send(data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make([]byte, len(data))
	copy(cp, data)
	m.received = append(m.received, cp)
	return nil
}

func (m *mockClient) getReceived() [][]byte {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.received
}

func TestHubBroadcast(t *testing.T) {
	h := New()
	go h.Run()
	defer h.Stop()
	c1 := &mockClient{}
	c2 := &mockClient{}
	h.Register(c1)
	h.Register(c2)
	h.Broadcast([]byte("hello"))
	time.Sleep(50 * time.Millisecond)
	if len(c1.getReceived()) != 1 || string(c1.getReceived()[0]) != "hello" {
		t.Errorf("client1 expected 'hello', got %v", c1.getReceived())
	}
	if len(c2.getReceived()) != 1 || string(c2.getReceived()[0]) != "hello" {
		t.Errorf("client2 expected 'hello', got %v", c2.getReceived())
	}
}

func TestHubUnregister(t *testing.T) {
	h := New()
	go h.Run()
	defer h.Stop()
	c1 := &mockClient{}
	h.Register(c1)
	h.Unregister(c1)
	h.Broadcast([]byte("hello"))
	time.Sleep(50 * time.Millisecond)
	if len(c1.getReceived()) != 0 {
		t.Error("unregistered client should not receive messages")
	}
}

func TestHubOnInput(t *testing.T) {
	h := New()
	go h.Run()
	defer h.Stop()
	var received []byte
	var mu sync.Mutex
	h.SetInputHandler(func(data []byte) {
		mu.Lock()
		received = append(received, data...)
		mu.Unlock()
	})
	h.Input([]byte("test input"))
	time.Sleep(50 * time.Millisecond)
	mu.Lock()
	defer mu.Unlock()
	if string(received) != "test input" {
		t.Errorf("expected 'test input', got %q", string(received))
	}
}

func TestHubClientCount(t *testing.T) {
	h := New()
	go h.Run()
	defer h.Stop()
	c1 := &mockClient{}
	c2 := &mockClient{}
	h.Register(c1)
	h.Register(c2)
	time.Sleep(20 * time.Millisecond)
	if h.ClientCount() != 2 {
		t.Errorf("expected 2 clients, got %d", h.ClientCount())
	}
	h.Unregister(c1)
	time.Sleep(20 * time.Millisecond)
	if h.ClientCount() != 1 {
		t.Errorf("expected 1 client, got %d", h.ClientCount())
	}
}
