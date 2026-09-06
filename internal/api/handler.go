package api

import (
	"encoding/json"
	"net/http"
	"sync/atomic"

	"github.com/adit-prawira/neko/internal/ffi"
)

type Server struct {
	isEngineReady atomic.Bool
}

func NewServer() *Server {
	return &Server{}
}

func (s *Server) InitEngine(dataDirectory string) error {
	if err := ffi.Init(dataDirectory); err != nil {
		return err
	}
	s.isEngineReady.Store(true)
	return nil
}

func (s *Server) ShutDown() error {
	return ffi.ShutDown()
}

func (s *Server) HandleHealth(rw http.ResponseWriter, r *http.Request) {
	if !s.isEngineReady.Load() {
		WriteHairball(rw, http.StatusServiceUnavailable, HairballInternalError.String(), "engine not initialised")
		return
	}
	WriteJSON(rw, http.StatusOK, map[string]string{
		"status": "Neko Server is Healthy",
	})
}

type CreateCollectionRequestHttpDTO struct {
	Name   string `json:"name"`
	Dim    uint32 `json:"dim"`
	Metric string `json:"metric"`
}

type CreateCollectionResponseHttpDTO struct {
	Name   string `json:"name"`
	Dim    uint32 `json:"dim"`
	Metric string `json:"metric"`
}

func (s *Server) HandleCreateCollection(rw http.ResponseWriter, r *http.Request) {
	var body CreateCollectionRequestHttpDTO
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		WriteHairball(rw, http.StatusBadRequest, HairballInvalidName.String(), "invalid request body")
		return
	}
	if body.Name == "" {
		WriteHairball(rw, http.StatusBadRequest, HairballInvalidName.String(), "name is required")
		return
	}

	if body.Metric == "" {
		body.Metric = "cosine"
	}

	metricCode, err := ffi.ParseMetric(body.Metric)
	if err != nil {
		WriteHairball(rw, http.StatusBadRequest, HairballInvalidMetric.String(), err.Error())
		return
	}

	if err := ffi.Create(body.Name, body.Dim, metricCode, ""); err != nil {
		WriteFFIError(rw, err)
		return
	}

	WriteJSON(rw, http.StatusCreated, CreateCollectionResponseHttpDTO{
		Name:   body.Name,
		Dim:    body.Dim,
		Metric: ffi.MetricNames[metricCode],
	})
}

type CollectionResponseHttpDTO struct {
	Name         string `json:"name"`
	Dim          uint32 `json:"dim"`
	Metric       string `json:"metric"`
	VectorCount  uint64 `json:"vector_count"`
	StorageBytes uint64 `json:"storage_bytes,omitempty"`
}

type CollectionsResponseHttpDTO struct {
	Collections []CollectionResponseHttpDTO `json:"collections"`
}

func (s *Server) HandleGetCollections(rw http.ResponseWriter, r *http.Request) {
	names, err := ffi.List()
	if err != nil {
		WriteFFIError(rw, err)
		return
	}

	collections := make([]CollectionResponseHttpDTO, 0, len(names))
	for _, name := range names {
		stats, err := ffi.Stats(name)
		if err != nil {
			continue
		}

		collections = append(collections, CollectionResponseHttpDTO{
			Name:         name,
			Dim:          stats.Dim,
			Metric:       ffi.MetricNames[stats.Metric],
			VectorCount:  stats.VectorCount,
			StorageBytes: stats.StorageBytes,
		})
	}

	WriteJSON(rw, http.StatusOK, CollectionsResponseHttpDTO{
		Collections: collections,
	})
}

func (s *Server) HandleGetCollection(rw http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	stats, err := ffi.Stats(name)
	if err != nil {
		WriteFFIError(rw, err)
		return
	}

	WriteJSON(rw, http.StatusOK, CollectionResponseHttpDTO{
		Name:         name,
		Dim:          stats.Dim,
		Metric:       ffi.MetricNames[stats.Metric],
		VectorCount:  stats.VectorCount,
		StorageBytes: stats.StorageBytes,
	})
}

func (s *Server) HandleDropCollection(rw http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if err := ffi.Drop(name); err != nil {
		WriteFFIError(rw, err)
		return
	}

	rw.WriteHeader(http.StatusNoContent)
}
