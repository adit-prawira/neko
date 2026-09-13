package api

import (
	"net/http"
)

func NewHttpServer(s *Server) *http.Server {
	handler := buildRoutes(s)
	handler = CORS(RequestLogging(PanicRecovery(handler)))
	return &http.Server{
		Handler: handler,
	}
}
