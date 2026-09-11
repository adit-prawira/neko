package api

import (
	"encoding/json"
	"net/http"
	"sync/atomic"

	"github.com/adit-prawira/neko/internal/ffi"
	"github.com/adit-prawira/neko/internal/shared"
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
		WriteHairball(rw, http.StatusServiceUnavailable, shared.HairballInternalError.String(), "engine not initialised")
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
		WriteHairball(rw, http.StatusBadRequest, shared.HairballInvalidName.String(), "invalid request body")
		return
	}
	if body.Name == "" {
		WriteHairball(rw, http.StatusBadRequest, shared.HairballInvalidName.String(), "name is required")
		return
	}

	if body.Metric == "" {
		body.Metric = "cosine"
	}

	metricCode, err := ffi.ParseMetric(body.Metric)
	if err != nil {
		WriteHairball(rw, http.StatusBadRequest, shared.HairballInvalidMetric.String(), err.Error())
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

type SearchQueryParamsHttpDTO struct {
	Vector []float32       `json:"vector"`
	TopK   *uint32         `json:"top_k"`
	Filter json.RawMessage `json:"filter,omitempty"`
}

type ScoredResultHttpDTO struct {
	ID    string  `json:"id"`
	Score float32 `json:"score"`
}

type SearchResponseHttpDTO struct {
	Results []ScoredResultHttpDTO `json:"results"`
}

const defaultTopK uint32 = 10

func (s *Server) HandleSearchCollection(rw http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	var body SearchQueryParamsHttpDTO
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		WriteHairball(rw, http.StatusBadRequest, shared.HairballInvalidName.String(), "invalid request body")
		return
	}

	topK := defaultTopK
	if body.TopK != nil {
		topK = *body.TopK
	}

	results, err := ffi.Search(name, body.Vector, topK)
	if err != nil {
		WriteFFIError(rw, err)
		return
	}

	scored_results := make([]ScoredResultHttpDTO, 0, len(results))
	for _, result := range results {
		scored_results = append(scored_results, ScoredResultHttpDTO{
			ID:    result.ID,
			Score: result.Score,
		})
	}
	WriteJSON(rw, http.StatusOK, SearchResponseHttpDTO{
		Results: scored_results,
	})
}

type InsertVectorHttpDTO struct {
	ID       string    `json:"id"`
	Vector   []float32 `json:"vector"`
	Metadata string    `json:"metadata"`
}

type InsertVectorResponseHttpDTO struct {
	ID  string `json:"id"`
	Dim int    `json:"dim"`
}

func (s *Server) HandleInsertVector(rw http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	var body InsertVectorHttpDTO
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		WriteHairball(rw, http.StatusBadRequest, shared.HairballInvalidName.String(), "invalid request body")
		return
	}

	if body.ID == "" {
		WriteHairball(rw, http.StatusBadRequest, shared.HairballInvalidName.String(), "id is required")
		return
	}

	if err := ffi.Insert(name, body.ID, body.Vector, body.Metadata); err != nil {
		WriteFFIError(rw, err)
		return
	}

	WriteJSON(rw, http.StatusCreated, InsertVectorResponseHttpDTO{
		ID:  body.ID,
		Dim: len(body.Vector),
	})
}

type GetVectorResponseHttpDTO struct {
	ID       string    `json:"id"`
	Vector   []float32 `json:"vector"`
	Metadata string    `json:"metadata,omitempty"`
}

func (s *Server) HandleGetVector(rw http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	id := r.PathValue("id")

	stats, err := ffi.Stats(name)
	if err != nil {
		WriteFFIError(rw, err)
		return
	}

	vector, metadata, err := ffi.GetVector(name, id, stats.Dim)
	if err != nil {
		WriteFFIError(rw, err)
		return
	}

	WriteJSON(rw, http.StatusOK, GetVectorResponseHttpDTO{
		ID:       id,
		Vector:   vector,
		Metadata: metadata,
	})
}
