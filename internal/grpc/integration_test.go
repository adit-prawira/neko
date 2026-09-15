package grpcserver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/soheilhy/cmux"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/adit-prawira/neko/internal/api"
	"github.com/adit-prawira/neko/internal/ffi"
	nekov1 "github.com/adit-prawira/neko/internal/gen/neko/v1"
)

const parityCollection = "grpc_parity_col"

// startTestServer wires the cmux-multiplexed REST + gRPC server on an
// ephemeral port (127.0.0.1:0). It returns the listen address and a cleanup
// function the caller MUST defer.
//
// Engine init is idempotent (Engine::init returns early when the singleton is
// already initialised), so this helper does not call ShutDown on cleanup —
// other tests in this package may share the engine.
func startTestServer(t *testing.T) (string, func()) {
	t.Helper()

	dataDirectory := filepath.Join(os.TempDir(), "neko_test_grpc_parity")
	if err := os.MkdirAll(dataDirectory, 0o755); err != nil {
		t.Fatalf("create data dir: %v", err)
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	address := listener.Addr().String()

	mux := cmux.New(listener)
	grpcListener := mux.Match(cmux.HTTP2())
	httpListener := mux.Match(cmux.HTTP1Fast())
	anyListener := mux.Match(cmux.Any())

	apiServer := api.NewServer()
	if err := apiServer.InitEngine(dataDirectory); err != nil {
		t.Fatalf("init engine: %v", err)
	}
	httpServer := api.NewHttpServer(apiServer)
	grpcServer := NewGRPCServer()
	RegisterServices(grpcServer)

	errChannel := make(chan error, 4)
	go func() { errChannel <- httpServer.Serve(httpListener) }()
	go func() { errChannel <- grpcServer.Serve(grpcListener) }()
	go func() { errChannel <- drainAnyListener(anyListener) }()
	go func() { errChannel <- mux.Serve() }()

	// Wait for /health to return 200 — at most 5s.
	deadline := time.Now().Add(5 * time.Second)
	for {
		response, err := http.Get(fmt.Sprintf("http://%s/health", address))
		if err == nil {
			body, _ := io.ReadAll(response.Body)
			response.Body.Close()
			if response.StatusCode == http.StatusOK {
				break
			}
			_ = body
		}
		if time.Now().After(deadline) {
			grpcServer.Stop()
			listener.Close()
			t.Fatalf("server did not become healthy on %s within 5s", address)
		}
		time.Sleep(20 * time.Millisecond)
	}

	cleanup := func() {
		grpcServer.Stop()
		_ = listener.Close()
	}
	return address, cleanup
}

func drainAnyListener(ln net.Listener) error {
	for {
		connection, err := ln.Accept()
		if err != nil {
			return nil
		}
		_ = connection.Close()
	}
}

// restCreateCollection POSTs to /v1/collections to create parityCollection.
func restCreateCollection(t *testing.T, baseURL string) {
	t.Helper()
	body := []byte(fmt.Sprintf(`{"name":%q,"dim":4,"metric":"l2"}`, parityCollection))
	response, err := http.Post(baseURL+"/v1/collections", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("REST create collection: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusCreated && response.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(response.Body)
		t.Fatalf("REST create collection: status %d body %s", response.StatusCode, raw)
	}
}

// restInsert POSTs a single vector to /v1/collections/{name}/vectors.
func restInsert(t *testing.T, baseURL, id string, vector []float32) {
	t.Helper()
	body, _ := json.Marshal(map[string]any{
		"id":     id,
		"vector": vector,
	})
	response, err := http.Post(
		fmt.Sprintf("%s/v1/collections/%s/vectors", baseURL, parityCollection),
		"application/json",
		bytes.NewReader(body),
	)
	if err != nil {
		t.Fatalf("REST insert %s: %v", id, err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusCreated && response.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(response.Body)
		t.Fatalf("REST insert %s: status %d body %s", id, response.StatusCode, raw)
	}
}

// restSearch POSTs to /v1/collections/{name}/search and decodes the response.
func restSearch(t *testing.T, baseURL string, query []float32, topK int) []scoredResult {
	t.Helper()
	body, _ := json.Marshal(map[string]any{
		"vector": query,
		"top_k":  topK,
	})
	response, err := http.Post(
		fmt.Sprintf("%s/v1/collections/%s/search", baseURL, parityCollection),
		"application/json",
		bytes.NewReader(body),
	)
	if err != nil {
		t.Fatalf("REST search: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(response.Body)
		t.Fatalf("REST search: status %d body %s", response.StatusCode, raw)
	}
	var payload struct {
		Results []scoredResult `json:"results"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("REST search decode: %v", err)
	}
	return payload.Results
}

// restGetVector GETs /v1/collections/{name}/vectors/{id} and returns the vector.
func restGetVector(t *testing.T, baseURL, id string, dim int) []float32 {
	t.Helper()
	response, err := http.Get(
		fmt.Sprintf("%s/v1/collections/%s/vectors/%s", baseURL, parityCollection, id),
	)
	if err != nil {
		t.Fatalf("REST get %s: %v", id, err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(response.Body)
		t.Fatalf("REST get %s: status %d body %s", id, response.StatusCode, raw)
	}
	var payload struct {
		Vector []float32 `json:"vector"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("REST get decode: %v", err)
	}
	if len(payload.Vector) != dim {
		t.Fatalf("REST get %s: got dim %d, want %d", id, len(payload.Vector), dim)
	}
	return payload.Vector
}

func grpcInsert(t *testing.T, conn *grpc.ClientConn, id string, vector []float32) {
	t.Helper()
	client := nekov1.NewVectorServiceClient(conn)
	_, err := client.Insert(context.Background(), &nekov1.InsertVectorRequest{
		Name: parityCollection,
		Body: &nekov1.UpsertVector{Id: id, Vector: vector},
	})
	if err != nil {
		t.Fatalf("gRPC insert %s: %v", id, err)
	}
}

func grpcSearch(t *testing.T, conn *grpc.ClientConn, query []float32, topK int) []scoredResult {
	t.Helper()
	client := nekov1.NewCollectionServiceClient(conn)
	k := uint32(topK)
	response, err := client.Search(context.Background(), &nekov1.SearchRequest{
		Name: parityCollection,
		Query: &nekov1.SearchQueryParams{
			Vector: query,
			TopK:   &k,
		},
	})
	if err != nil {
		t.Fatalf("gRPC search: %v", err)
	}
	out := make([]scoredResult, len(response.Results))
	for index, result := range response.Results {
		out[index] = scoredResult{ID: result.Id, Score: result.Score}
	}
	return out
}

type scoredResult struct {
	ID    string  `json:"id"`
	Score float32 `json:"score"`
}

// assertResultsEqual asserts that two slices carry the same ids in the same
// order and the same scores within float32 precision. REST and gRPC both
// round through the same FFI path, so the scores are bit-identical in
// practice — the tolerance is for safety.
func assertResultsEqual(t *testing.T, queryLabel string, rest, grpcResults []scoredResult) {
	t.Helper()
	if len(rest) != len(grpcResults) {
		t.Fatalf("%s: result count mismatch — REST=%d gRPC=%d\nREST: %+v\ngRPC: %+v",
			queryLabel, len(rest), len(grpcResults), rest, grpcResults)
	}
	for index := range rest {
		if rest[index].ID != grpcResults[index].ID {
			t.Errorf("%s: id mismatch at index %d — REST=%q gRPC=%q",
				queryLabel, index, rest[index].ID, grpcResults[index].ID)
		}
		if diff := math.Abs(float64(rest[index].Score - grpcResults[index].Score)); diff > 1e-6 {
			t.Errorf("%s: score mismatch at index %d (id=%q) — REST=%v gRPC=%v diff=%v",
				queryLabel, index, rest[index].ID, rest[index].Score, grpcResults[index].Score, diff)
		}
	}
}

func TestRESTGRPCSearchParity(t *testing.T) {
	address, cleanup := startTestServer(t)
	defer cleanup()
	baseURL := fmt.Sprintf("http://%s", address)

	conn, err := grpc.NewClient(
		address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("grpc.NewClient: %v", err)
	}
	defer conn.Close()

	restCreateCollection(t, baseURL)
	defer func() { _ = ffi.Drop(parityCollection) }()

	t.Run("given same data inserted via REST, then REST search and gRPC search return identical top-k for 3 queries", func(t *testing.T) {
		// Five deterministic 4-dim vectors chosen so L2 distances to the
		// three queries below are unambiguous (no ties).
		vectors := map[string][]float32{
			"a": {1, 0, 0, 0},
			"b": {0, 1, 0, 0},
			"c": {0, 0, 1, 0},
			"d": {0, 0, 0, 1},
			"e": {2, 0, 0, 0},
		}
		for id, vector := range vectors {
			restInsert(t, baseURL, id, vector)
		}

		queries := []struct {
			label  string
			vector []float32
		}{
			{"q1=[1,1,0,0]", []float32{1, 1, 0, 0}},
			{"q2=[0,0,1,1]", []float32{0, 0, 1, 1}},
			{"q3=[2,0,1,0]", []float32{2, 0, 1, 0}},
		}

		for _, query := range queries {
			t.Run(query.label, func(t *testing.T) {
				restResults := restSearch(t, baseURL, query.vector, 5)
				grpcResults := grpcSearch(t, conn, query.vector, 5)
				assertResultsEqual(t, query.label, restResults, grpcResults)
			})
		}
	})

	t.Run("given a vector inserted via gRPC, then REST GET returns the same bytes", func(t *testing.T) {
		vector := []float32{3, 0, 0, 0}
		grpcInsert(t, conn, "f", vector)

		got := restGetVector(t, baseURL, "f", 4)
		for index, value := range vector {
			if got[index] != value {
				t.Errorf("index %d: got %v, want %v", index, got[index], value)
			}
		}
	})
}
