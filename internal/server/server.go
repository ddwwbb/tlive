package server

import (
	"io/fs"
	"net/http"

	"github.com/termlive/termlive/internal/daemon"
)

type Server struct {
	mgr   *daemon.SessionManager
	webFS fs.FS
}

func New(mgr *daemon.SessionManager) *Server {
	return &Server{mgr: mgr}
}

func (s *Server) SetWebFS(webFS fs.FS) {
	s.webFS = webFS
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/sessions", handleSessionList(s.mgr))
	mux.HandleFunc("/ws/", handleWebSocket(s.mgr))
	if s.webFS != nil {
		mux.Handle("/", http.FileServer(http.FS(s.webFS)))
	}
	return mux
}
