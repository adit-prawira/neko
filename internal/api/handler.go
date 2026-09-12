// Package api exposes the neko REST surface (HTTP/1.1, JSON).
//
//	@title			Neko API
//	@version		0.1.0
//	@description	Local-first vector database REST API. Brute-force KNN search over raw f32 vectors, with collection-scoped inserts and metadata.
//	@host			localhost:3434
//	@schemes		http
package api

import (
	"encoding/json"
	"fmt"
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

// HandleHealth godoc
//
//	@Summary		Liveness check
//	@Description	Returns ok if the engine has been initialised.
//	@Tags			system
//	@Produce		json
//	@Success		200	{object}	map[string]string	"Neko Server is Healthy"
//	@Failure		503	{object}	APIError			"engine not initialised"
//	@Router			/health [get]
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

// HandleCreateCollection godoc
//
//	@Summary		Create a collection
//	@Description	Creates a new named collection with a fixed dimension and distance metric.
//	@Tags			collections
//	@Accept			json
//	@Produce		json
//	@Param			body	body		CreateCollectionRequestHttpDTO	true	"Collection config"
//	@Success		201		{object}	CreateCollectionResponseHttpDTO
//	@Failure		400		{object}	APIError	"HAIRBALL_INVALID_NAME / HAIRBALL_INVALID_METRIC"
//	@Failure		409		{object}	APIError	"HAIRBALL_ALREADY_EXISTS"
//	@Router			/v1/collections [post]
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

// HandleGetCollections godoc
//
//	@Summary		List collections
//	@Description	Returns all collections with their dimension, metric, and vector count.
//	@Tags			collections
//	@Produce		json
//	@Success		200	{object}	CollectionsResponseHttpDTO
//	@Router			/v1/collections [get]
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

// HandleGetCollection godoc
//
//	@Summary		Get collection info
//	@Description	Returns stats for a single collection.
//	@Tags			collections
//	@Produce		json
//	@Param			name	path		string	true	"Collection name"
//	@Success		200		{object}	CollectionResponseHttpDTO
//	@Failure		404		{object}	APIError	"HAIRBALL_NOT_FOUND"
//	@Router			/v1/collections/{name} [get]
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

// HandleDropCollection godoc
//
//	@Summary		Drop a collection
//	@Description	Removes a collection and its data directory.
//	@Tags			collections
//	@Produce		json
//	@Param			name	path	string	true	"Collection name"
//	@Success		204		"No Content"
//	@Failure		404		{object}	APIError	"HAIRBALL_NOT_FOUND"
//	@Router			/v1/collections/{name} [delete]
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

// HandleSearchCollection godoc
//
//	@Summary		Search top-K nearest neighbors
//	@Description	Brute-force KNN over the collection's vectors. Returns the top-k results sorted by score.
//	@Tags			search
//	@Accept			json
//	@Produce		json
//	@Param			name	path		string						true	"Collection name"
//	@Param			body	body		SearchQueryParamsHttpDTO	true	"Search query"
//	@Success		200		{object}	SearchResponseHttpDTO
//	@Failure		400		{object}	APIError	"HAIRBALL_DIM_MISMATCH / HAIRBALL_DIM_TOO_SMALL"
//	@Failure		404		{object}	APIError	"HAIRBALL_NOT_FOUND"
//	@Router			/v1/collections/{name}/search [post]
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

type UpsertVectorHttpDTO struct {
	ID       string    `json:"id"`
	Vector   []float32 `json:"vector"`
	Metadata string    `json:"metadata"`
}

type UpsertVectorResponseHttpDTO struct {
	ID  string `json:"id"`
	Dim int    `json:"dim"`
}

// HandleInsertVector godoc
//
//	@Summary		Insert a single vector
//	@Description	Inserts a vector with a given ID. Fails if the ID already exists.
//	@Tags			vectors
//	@Accept			json
//	@Produce		json
//	@Param			name	path		string				true	"Collection name"
//	@Param			body	body		UpsertVectorHttpDTO	true	"Vector to insert (id is in the path; do not include in body)"
//	@Success		201		{object}	UpsertVectorResponseHttpDTO
//	@Failure		400		{object}	APIError	"HAIRBALL_DIM_MISMATCH / HAIRBALL_DIM_TOO_SMALL"
//	@Failure		404		{object}	APIError	"HAIRBALL_NOT_FOUND"
//	@Failure		409		{object}	APIError	"HAIRBALL_ALREADY_EXISTS"
//	@Router			/v1/collections/{name}/vectors [post]
func (s *Server) HandleInsertVector(rw http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	var body UpsertVectorHttpDTO
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

	WriteJSON(rw, http.StatusCreated, UpsertVectorResponseHttpDTO{
		ID:  body.ID,
		Dim: len(body.Vector),
	})
}

const maxBatchInsertSize = 10000

type InsertManyVectorResponseHttpDTO struct {
	IDs      []string `json:"ids"`
	Inserted int      `json:"inserted"`
	Dim      uint32   `json:"dim"`
}

// HandleInsertManyVector godoc
//
//	@Summary		Batch insert vectors
//	@Description	Inserts up to 10,000 vectors in one call. The request body is a bare JSON array (no wrapping object). Each element must include `id` and `vector`. All-or-nothing on dim and id validation.
//	@Tags			vectors
//	@Accept			json
//	@Produce		json
//	@Param			name	path		string					true	"Collection name"
//	@Param			body	body		[]UpsertVectorHttpDTO	true	"Array of vectors to insert (max 10,000)"
//	@Success		201		{object}	InsertManyVectorResponseHttpDTO
//	@Failure		400		{object}	APIError	"HAIRBALL_INVALID_NAME / HAIRBALL_DIM_MISMATCH"
//	@Failure		404		{object}	APIError	"HAIRBALL_NOT_FOUND"
//	@Router			/v1/collections/{name}/vectors/batch [post]
func (s *Server) HandleInsertManyVector(rw http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	var body []UpsertVectorHttpDTO

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		WriteHairball(rw, http.StatusBadRequest, shared.HairballInvalidName.String(), "invalid request body")
		return
	}

	if len(body) == 0 {
		WriteHairball(rw, http.StatusBadRequest, shared.HairballInvalidName.String(), "vector array is required and must not be empty")
		return
	}

	if len(body) > maxBatchInsertSize {
		message := fmt.Sprintf("batch size %d exceeds maximum of %d vectors", len(body), maxBatchInsertSize)
		WriteHairball(rw, http.StatusBadRequest, shared.HairballInvalidName.String(), message)
		return
	}

	stats, err := ffi.Stats(name)
	if err != nil {
		WriteFFIError(rw, err)
		return
	}

	inputVectors := make([]ffi.InputVector, 0, len(body))
	for index, input := range body {
		if input.ID == "" {
			message := fmt.Sprintf("vector[%d].id is required", index)
			WriteHairball(rw, http.StatusBadRequest, shared.HairballInvalidName.String(), message)
			return
		}

		dim := len(input.Vector)
		if dim != int(stats.Dim) {
			message := fmt.Sprintf("vectors[%d] has dim %d, expected %d", index, dim, stats.Dim)
			WriteHairball(rw, http.StatusBadRequest, shared.HairballDimMismatch.String(), message)
			return
		}

		inputVectors = append(inputVectors, ffi.InputVector{
			ID:       input.ID,
			Vector:   input.Vector,
			Metadata: input.Metadata,
		})
	}

	if err := ffi.InsertMany(name, inputVectors); err != nil {
		WriteFFIError(rw, err)
		return
	}

	insertedIDs := make([]string, len(body))

	for index, input := range body {
		insertedIDs[index] = input.ID
	}

	WriteJSON(rw, http.StatusCreated, InsertManyVectorResponseHttpDTO{
		IDs:      insertedIDs,
		Inserted: len(insertedIDs),
		Dim:      stats.Dim,
	})
}

type GetVectorResponseHttpDTO struct {
	ID       string    `json:"id"`
	Vector   []float32 `json:"vector"`
	Metadata string    `json:"metadata,omitempty"`
}

// HandleGetVector godoc
//
//	@Summary		Get a vector by ID
//	@Description	Returns the vector and its raw metadata JSON string.
//	@Tags			vectors
//	@Produce		json
//	@Param			name	path		string	true	"Collection name"
//	@Param			id		path		string	true	"Vector ID"
//	@Success		200		{object}	GetVectorResponseHttpDTO
//	@Failure		404		{object}	APIError	"HAIRBALL_NOT_FOUND"
//	@Router			/v1/collections/{name}/vectors/{id} [get]
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

// HandleUpsertVector godoc
//
//	@Summary		Insert or update a vector
//	@Description	Insert-or-update by ID. Returns 201 if the vector was created, 200 if an existing vector was replaced.
//	@Tags			vectors
//	@Accept			json
//	@Produce		json
//	@Param			name	path		string						true	"Collection name"
//	@Param			id		path		string						true	"Vector ID"
//	@Param			body	body		UpsertVectorHttpDTO			true	"Vector data (id is in the path; do not include in body)"
//	@Success		200		{object}	UpsertVectorResponseHttpDTO	"Vector updated"
//	@Success		201		{object}	UpsertVectorResponseHttpDTO	"Vector created"
//	@Failure		400		{object}	APIError					"HAIRBALL_DIM_MISMATCH / HAIRBALL_DIM_TOO_SMALL"
//	@Failure		404		{object}	APIError					"HAIRBALL_NOT_FOUND"
//	@Router			/v1/collections/{name}/vectors/{id} [put]
func (s *Server) HandleUpsertVector(rw http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	id := r.PathValue("id")

	var body UpsertVectorHttpDTO
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		WriteHairball(rw, http.StatusBadRequest, shared.HairballInvalidName.String(), "invalid request body")
		return
	}

	created, err := ffi.Upsert(name, id, body.Vector, body.Metadata)
	if err != nil {
		WriteFFIError(rw, err)
		return
	}

	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}

	WriteJSON(rw, status, UpsertVectorResponseHttpDTO{
		ID:  id,
		Dim: len(body.Vector),
	})
}

// HandleDeleteVector godoc
//
//	@Summary		Delete a vector
//	@Description	Removes a vector by ID.
//	@Tags			vectors
//	@Produce		json
//	@Param			name	path	string	true	"Collection name"
//	@Param			id		path	string	true	"Vector ID"
//	@Success		204		"No Content"
//	@Failure		404		{object}	APIError	"HAIRBALL_NOT_FOUND"
//	@Router			/v1/collections/{name}/vectors/{id} [delete]
func (s *Server) HandleDeleteVector(rw http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	id := r.PathValue("id")

	if err := ffi.Delete(name, id); err != nil {
		WriteFFIError(rw, err)
		return
	}
	rw.WriteHeader(http.StatusNoContent)
}
