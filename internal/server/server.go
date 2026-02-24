package server

import (
	"io/fs"
	"net/http"

	"github.com/termlive/termlive/internal/hub"
	"github.com/termlive/termlive/internal/session"
)

type Server struct {
	store *session.Store
	hubs  map[string]*hub.Hub
	webFS fs.FS
	token string
}

func New(store *session.Store, hubs map[string]*hub.Hub, token string) *Server {
	return &Server{store: store, hubs: hubs, token: token}
}

func (s *Server) SetWebFS(webFS fs.FS) {
	s.webFS = webFS
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/sessions", handleSessionList(s.store))
	mux.HandleFunc("/ws/", handleWebSocket(s.hubs))
	if s.webFS != nil {
		mux.Handle("/", http.FileServer(http.FS(s.webFS)))
	}
	return mux
}
