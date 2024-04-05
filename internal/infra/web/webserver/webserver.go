package webserver

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type WebServer struct {
	Router        chi.Router
	Handlers      map[string]http.HandlerFunc
	Methods       map[string]string
	WebServerPort string
}

func NewWebServer(serverPort string) *WebServer {
	return &WebServer{
		Router:        chi.NewRouter(),
		Handlers:      make(map[string]http.HandlerFunc),
		Methods:       make(map[string]string),
		WebServerPort: serverPort,
	}
}

func (s *WebServer) AddHandler(method string, path string, handler http.HandlerFunc) {
	s.Handlers[path] = handler
	s.Methods[path] = method
}

// loop through the handlers and add them to the router
// register middeleware logger
// start the server
func (s *WebServer) Start() {
	s.Router.Use(middleware.Logger)
	for path, handler := range s.Handlers {
		if s.Methods[path] == "POST" {
			s.Router.Post(path, handler)
		} else if s.Methods[path] == "GET" {
			s.Router.Get(path, handler)
		} else {
			s.Router.Handle(path, handler)
		}
	}

	http.ListenAndServe(":"+s.WebServerPort, s.Router)
}
