package server

import (
	"io/fs"
	"net/http"

	"github.com/termlive/termlive/internal/hub"
	"github.com/termlive/termlive/internal/session"
)

// ResizeFunc is a callback invoked when a WebSocket client sends a resize
// control message.
type ResizeFunc func(rows, cols uint16)

type Server struct {
	store       *session.Store
	hubs        map[string]*hub.Hub
	resizeFuncs map[string]ResizeFunc
	webFS       fs.FS
	token       string
}

func New(store *session.Store, hubs map[string]*hub.Hub, token string) *Server {
	return &Server{store: store, hubs: hubs, token: token}
}

// SetResizeFunc registers a resize callback for the given session ID.
func (s *Server) SetResizeFunc(sessionID string, fn ResizeFunc) {
	if s.resizeFuncs == nil {
		s.resizeFuncs = make(map[string]ResizeFunc)
	}
	s.resizeFuncs[sessionID] = fn
}

func (s *Server) SetWebFS(webFS fs.FS) {
	s.webFS = webFS
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/sessions", handleSessionList(s.store))
	mux.HandleFunc("/ws/", handleWebSocket(s.hubs, s.resizeFuncs))
	if s.webFS != nil {
		mux.Handle("/", http.FileServer(http.FS(s.webFS)))
	}
	if s.token != "" {
		return s.authMiddleware(mux)
	}
	return mux
}

func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.URL.Query().Get("token")
		if token == "" {
			if cookie, err := r.Cookie("tl_token"); err == nil {
				token = cookie.Value
			}
		}
		if token != s.token {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		http.SetCookie(w, &http.Cookie{
			Name:  "tl_token",
			Value: s.token,
			Path:  "/",
		})
		next.ServeHTTP(w, r)
	})
}
