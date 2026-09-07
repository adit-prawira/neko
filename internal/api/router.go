package api

import (
	"fmt"
	"net/http"

	"github.com/adit-prawira/neko/internal/shared"
)

func buildRoutes(s *Server) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.HandleHealth)
	mux.HandleFunc("POST /v1/collections", s.HandleCreateCollection)
	mux.HandleFunc("GET /v1/collections", s.HandleGetCollections)
	mux.HandleFunc("GET /v1/collections/{name}", s.HandleGetCollection)
	mux.HandleFunc("DELETE /v1/collections/{name}", s.HandleDropCollection)
	mux.HandleFunc("POST /v1/collections/{name}/search", s.HandleSearchCollection)
	mux.HandleFunc("/", func(rw http.ResponseWriter, r *http.Request) {
		message := fmt.Sprintf("route %s %s is not found", r.Method, r.URL.Path)
		WriteHairball(rw, http.StatusNotFound, shared.HairballNotFound.String(), message)
	})
	return mux
}
