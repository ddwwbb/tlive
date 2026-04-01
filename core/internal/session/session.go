// internal/session/session.go
package session

import (
	"crypto/rand"
	"encoding/hex"
	"sort"
	"sync"
	"time"
)

type Status string

const (
	StatusRunning Status = "running"
	StatusStopped Status = "stopped"
)

const outputBufferSize = 64 * 1024 // 64 KB — enough to replay ANSI styling on reconnect

type Session struct {
	ID        string
	Command   string
	Args      []string
	Pid       int
	Status    Status
	StartTime time.Time
	Rows      uint16
	Cols      uint16
	Cwd       string
	mu        sync.Mutex
	output    []byte
}

func New(command string, args []string) *Session {
	return &Session{
		ID:        generateID(),
		Command:   command,
		Args:      args,
		Status:    StatusRunning,
		StartTime: time.Now(),
		output:    make([]byte, 0, outputBufferSize),
	}
}

func (s *Session) AppendOutput(data []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.output = append(s.output, data...)
	if len(s.output) > outputBufferSize {
		s.output = s.output[len(s.output)-outputBufferSize:]
	}
}

func (s *Session) LastOutput(n int) []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	if n >= len(s.output) {
		result := make([]byte, len(s.output))
		copy(result, s.output)
		return result
	}
	result := make([]byte, n)
	copy(result, s.output[len(s.output)-n:])
	return result
}

func (s *Session) Size() (uint16, uint16) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.Rows, s.Cols
}

func (s *Session) SetSize(rows, cols uint16) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Rows = rows
	s.Cols = cols
}

func (s *Session) Duration() time.Duration {
	return time.Since(s.StartTime)
}

func generateID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}

type Store struct {
	mu       sync.RWMutex
	sessions map[string]*Session
}

func NewStore() *Store {
	return &Store{sessions: make(map[string]*Session)}
}

func (st *Store) Add(s *Session) {
	st.mu.Lock()
	defer st.mu.Unlock()
	st.sessions[s.ID] = s
}

func (st *Store) Get(id string) (*Session, bool) {
	st.mu.RLock()
	defer st.mu.RUnlock()
	s, ok := st.sessions[id]
	return s, ok
}

func (st *Store) Remove(id string) {
	st.mu.Lock()
	defer st.mu.Unlock()
	delete(st.sessions, id)
}

func (st *Store) List() []*Session {
	st.mu.RLock()
	defer st.mu.RUnlock()
	result := make([]*Session, 0, len(st.sessions))
	for _, s := range st.sessions {
		result = append(result, s)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].StartTime.Before(result[j].StartTime)
	})
	return result
}
