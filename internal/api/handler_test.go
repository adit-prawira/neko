package api

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/adit-prawira/neko/internal/ffi"
)

func apiTestSetup(t *testing.T) *Server {
	t.Helper()
	dataDirectory := filepath.Join(os.TempDir(), "neko_test_api")
	if err := os.MkdirAll(dataDirectory, 0o755); err != nil {
		t.Fatalf("failed to create data directory: %v", err)
	}
	server := NewServer()
	if err := server.InitEngine(dataDirectory); err != nil {
		t.Fatalf("engine init failed: %v", err)
	}
	return server
}

func decodeResponse(t *testing.T, recorder *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}
	return body
}

func errorCode(t *testing.T, recorder *httptest.ResponseRecorder) string {
	t.Helper()
	body := decodeResponse(t, recorder)
	detail, ok := body["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected error object in response, got %v", body)
	}
	code, ok := detail["code"].(string)
	if !ok {
		t.Fatalf("expected error code string, got %v", detail["code"])
	}
	return code
}

func TestHandleHealth(t *testing.T) {
	t.Run("given engine ready, then returns 200", func(t *testing.T) {
		server := apiTestSetup(t)
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/health", nil)

		server.HandleHealth(recorder, request)

		if recorder.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", recorder.Code)
		}
		body := decodeResponse(t, recorder)
		if _, ok := body["status"]; !ok {
			t.Errorf("expected 'status' field in response, got %v", body)
		}
	})

	t.Run("given engine not ready, then returns 503", func(t *testing.T) {
		server := NewServer()
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/health", nil)

		server.HandleHealth(recorder, request)

		if recorder.Code != http.StatusServiceUnavailable {
			t.Fatalf("expected status 503, got %d", recorder.Code)
		}
	})
}

func TestHandleCreateCollection(t *testing.T) {
	t.Run("given valid body with explicit metric, then returns 201", func(t *testing.T) {
		server := apiTestSetup(t)
		name := "api_test_create_explicit"
		defer func() { _ = ffi.Drop(name) }()

		body := strings.NewReader(`{"name":"` + name + `","dim":384,"metric":"cosine"}`)
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/v1/collections", body)

		server.HandleCreateCollection(recorder, request)

		if recorder.Code != http.StatusCreated {
			t.Fatalf("expected status 201, got %d: %s", recorder.Code, recorder.Body.String())
		}
		response := decodeResponse(t, recorder)
		if response["name"] != name {
			t.Errorf("expected name %q, got %v", name, response["name"])
		}
		if response["metric"] != "cosine" {
			t.Errorf("expected metric %q, got %v", "cosine", response["metric"])
		}
	})

	t.Run("given body without metric, then defaults to cosine", func(t *testing.T) {
		server := apiTestSetup(t)
		name := "api_test_create_default_metric"
		defer func() { _ = ffi.Drop(name) }()

		body := strings.NewReader(`{"name":"` + name + `","dim":384}`)
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/v1/collections", body)

		server.HandleCreateCollection(recorder, request)

		if recorder.Code != http.StatusCreated {
			t.Fatalf("expected status 201, got %d: %s", recorder.Code, recorder.Body.String())
		}
		response := decodeResponse(t, recorder)
		if response["metric"] != "cosine" {
			t.Errorf("expected default metric cosine, got %v", response["metric"])
		}
	})

	t.Run("given duplicate name, then returns 409 with HAIRBALL_ALREADY_EXISTS", func(t *testing.T) {
		server := apiTestSetup(t)
		name := "api_test_create_duplicate"
		defer func() { _ = ffi.Drop(name) }()

		payload := `{"name":"` + name + `","dim":384,"metric":"l2"}`

		firstRecorder := httptest.NewRecorder()
		server.HandleCreateCollection(firstRecorder, httptest.NewRequest(http.MethodPost, "/v1/collections", strings.NewReader(payload)))
		if firstRecorder.Code != http.StatusCreated {
			t.Fatalf("first create expected 201, got %d", firstRecorder.Code)
		}

		secondRecorder := httptest.NewRecorder()
		server.HandleCreateCollection(secondRecorder, httptest.NewRequest(http.MethodPost, "/v1/collections", strings.NewReader(payload)))

		if secondRecorder.Code != http.StatusConflict {
			t.Fatalf("expected status 409, got %d: %s", secondRecorder.Code, secondRecorder.Body.String())
		}
		if code := errorCode(t, secondRecorder); code != "HAIRBALL_ALREADY_EXISTS" {
			t.Errorf("expected HAIRBALL_ALREADY_EXISTS, got %s", code)
		}
	})

	t.Run("given invalid metric, then returns 400 with HAIRBALL_INVALID_METRIC", func(t *testing.T) {
		server := apiTestSetup(t)

		body := strings.NewReader(`{"name":"api_test_bad_metric","dim":384,"metric":"euclidean"}`)
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/v1/collections", body)

		server.HandleCreateCollection(recorder, request)

		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d", recorder.Code)
		}
		if code := errorCode(t, recorder); code != "HAIRBALL_INVALID_METRIC" {
			t.Errorf("expected HAIRBALL_INVALID_METRIC, got %s", code)
		}
	})

	t.Run("given empty name, then returns 400 with HAIRBALL_INVALID_NAME", func(t *testing.T) {
		server := apiTestSetup(t)

		body := strings.NewReader(`{"name":"","dim":384}`)
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/v1/collections", body)

		server.HandleCreateCollection(recorder, request)

		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d", recorder.Code)
		}
		if code := errorCode(t, recorder); code != "HAIRBALL_INVALID_NAME" {
			t.Errorf("expected HAIRBALL_INVALID_NAME, got %s", code)
		}
	})

	t.Run("given malformed JSON body, then returns 400", func(t *testing.T) {
		server := apiTestSetup(t)

		body := strings.NewReader(`{"name": "unclosed"`)
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/v1/collections", body)

		server.HandleCreateCollection(recorder, request)

		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d", recorder.Code)
		}
	})
}

func TestHandleGetCollections(t *testing.T) {
	t.Run("given collections exist, then returns 200 with the list", func(t *testing.T) {
		server := apiTestSetup(t)
		name := "api_test_list_collections"
		defer func() { _ = ffi.Drop(name) }()
		if err := ffi.Create(name, 256, ffi.MetricL2, ""); err != nil {
			t.Fatalf("failed to create collection: %v", err)
		}

		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/v1/collections", nil)

		server.HandleGetCollections(recorder, request)

		if recorder.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", recorder.Code)
		}
		body := decodeResponse(t, recorder)
		collections, ok := body["collections"].([]any)
		if !ok {
			t.Fatalf("expected 'collections' array, got %v", body)
		}
		found := false
		for _, item := range collections {
			collection, ok := item.(map[string]any)
			if ok && collection["name"] == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected collection %q in list, got %v", name, collections)
		}
	})
}

func TestHandleGetCollection(t *testing.T) {
	t.Run("given existing collection, then returns 200 with info", func(t *testing.T) {
		server := apiTestSetup(t)
		name := "api_test_get_collection"
		defer func() { _ = ffi.Drop(name) }()
		if err := ffi.Create(name, 128, ffi.MetricDot, ""); err != nil {
			t.Fatalf("failed to create collection: %v", err)
		}

		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/v1/collections/"+name, nil)
		request.SetPathValue("name", name)

		server.HandleGetCollection(recorder, request)

		if recorder.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", recorder.Code)
		}
		body := decodeResponse(t, recorder)
		if body["name"] != name {
			t.Errorf("expected name %q, got %v", name, body["name"])
		}
		if body["dim"] != float64(128) {
			t.Errorf("expected dim 128, got %v", body["dim"])
		}
	})

	t.Run("given nonexistent collection, then returns 404 with HAIRBALL_NOT_FOUND", func(t *testing.T) {
		server := apiTestSetup(t)
		name := "api_test_get_missing"

		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/v1/collections/"+name, nil)
		request.SetPathValue("name", name)

		server.HandleGetCollection(recorder, request)

		if recorder.Code != http.StatusNotFound {
			t.Fatalf("expected status 404, got %d", recorder.Code)
		}
		if code := errorCode(t, recorder); code != "HAIRBALL_NOT_FOUND" {
			t.Errorf("expected HAIRBALL_NOT_FOUND, got %s", code)
		}
	})
}

func TestHandleDropCollection(t *testing.T) {
	t.Run("given existing collection, then returns 204", func(t *testing.T) {
		server := apiTestSetup(t)
		name := "api_test_drop_collection"
		defer func() { _ = ffi.Drop(name) }()
		if err := ffi.Create(name, 64, ffi.MetricL2, ""); err != nil {
			t.Fatalf("failed to create collection: %v", err)
		}

		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodDelete, "/v1/collections/"+name, nil)
		request.SetPathValue("name", name)

		server.HandleDropCollection(recorder, request)

		if recorder.Code != http.StatusNoContent {
			t.Fatalf("expected status 204, got %d", recorder.Code)
		}
	})

	t.Run("given nonexistent collection, then returns 404 with HAIRBALL_NOT_FOUND", func(t *testing.T) {
		server := apiTestSetup(t)
		name := "api_test_drop_missing"

		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodDelete, "/v1/collections/"+name, nil)
		request.SetPathValue("name", name)

		server.HandleDropCollection(recorder, request)

		if recorder.Code != http.StatusNotFound {
			t.Fatalf("expected status 404, got %d", recorder.Code)
		}
		if code := errorCode(t, recorder); code != "HAIRBALL_NOT_FOUND" {
			t.Errorf("expected HAIRBALL_NOT_FOUND, got %s", code)
		}
	})
}

func TestHandleSearchCollection(t *testing.T) {
	t.Run("given valid query, then returns ranked top-K with scores", func(t *testing.T) {
		server := apiTestSetup(t)
		name := "api_test_search_l2"
		defer func() { _ = ffi.Drop(name) }()
		if err := ffi.Create(name, 3, ffi.MetricL2, ""); err != nil {
			t.Fatalf("create failed: %v", err)
		}
		if err := ffi.Insert(name, "far", []float32{10.0, 0.0, 0.0}, ""); err != nil {
			t.Fatalf("insert failed: %v", err)
		}
		if err := ffi.Insert(name, "near", []float32{2.0, 0.0, 0.0}, ""); err != nil {
			t.Fatalf("insert failed: %v", err)
		}
		if err := ffi.Insert(name, "mid", []float32{5.0, 0.0, 0.0}, ""); err != nil {
			t.Fatalf("insert failed: %v", err)
		}

		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/v1/collections/"+name+"/search", strings.NewReader(`{"vector":[1.0,0.0,0.0],"top_k":2}`))
		request.SetPathValue("name", name)

		server.HandleSearchCollection(recorder, request)

		if recorder.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
		}
		body := decodeResponse(t, recorder)
		results, ok := body["results"].([]any)
		if !ok {
			t.Fatalf("expected results array, got %v", body)
		}
		if len(results) != 2 {
			t.Fatalf("expected 2 results, got %d", len(results))
		}
		first, ok := results[0].(map[string]any)
		if !ok {
			t.Fatalf("expected first result to be an object, got %T", results[0])
		}
		if first["id"] != "near" {
			t.Errorf("expected first id 'near' (L2 distance 1 from query), got %v", first["id"])
		}
		second, ok := results[1].(map[string]any)
		if !ok {
			t.Fatalf("expected second result to be an object, got %T", results[1])
		}
		if second["id"] != "mid" {
			t.Errorf("expected second id 'mid' (L2 distance 4), got %v", second["id"])
		}
	})

	t.Run("given empty collection, then returns 200 with empty results", func(t *testing.T) {
		server := apiTestSetup(t)
		name := "api_test_search_empty"
		defer func() { _ = ffi.Drop(name) }()
		if err := ffi.Create(name, 3, ffi.MetricL2, ""); err != nil {
			t.Fatalf("create failed: %v", err)
		}

		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/v1/collections/"+name+"/search", strings.NewReader(`{"vector":[1.0,0.0,0.0],"top_k":10}`))
		request.SetPathValue("name", name)

		server.HandleSearchCollection(recorder, request)

		if recorder.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
		}
		body := decodeResponse(t, recorder)
		results, ok := body["results"].([]any)
		if !ok {
			t.Fatalf("expected results array, got %v", body)
		}
		if len(results) != 0 {
			t.Errorf("expected 0 results for empty collection, got %d", len(results))
		}
	})

	t.Run("given top_k larger than collection size, then returns all available", func(t *testing.T) {
		server := apiTestSetup(t)
		name := "api_test_search_topk_large"
		defer func() { _ = ffi.Drop(name) }()
		if err := ffi.Create(name, 3, ffi.MetricL2, ""); err != nil {
			t.Fatalf("create failed: %v", err)
		}
		if err := ffi.Insert(name, "near", []float32{2.0, 0.0, 0.0}, ""); err != nil {
			t.Fatalf("insert failed: %v", err)
		}
		if err := ffi.Insert(name, "far", []float32{10.0, 0.0, 0.0}, ""); err != nil {
			t.Fatalf("insert failed: %v", err)
		}

		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/v1/collections/"+name+"/search", strings.NewReader(`{"vector":[1.0,0.0,0.0],"top_k":10}`))
		request.SetPathValue("name", name)

		server.HandleSearchCollection(recorder, request)

		if recorder.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
		}
		body := decodeResponse(t, recorder)
		results, ok := body["results"].([]any)
		if !ok {
			t.Fatalf("expected results array, got %v", body)
		}
		if len(results) != 2 {
			t.Errorf("expected 2 results (all available), got %d", len(results))
		}
	})

	t.Run("given missing collection, then returns 404 with HAIRBALL_NOT_FOUND", func(t *testing.T) {
		server := apiTestSetup(t)
		name := "api_test_search_missing_clowder"

		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/v1/collections/"+name+"/search", strings.NewReader(`{"vector":[1.0,0.0,0.0],"top_k":10}`))
		request.SetPathValue("name", name)

		server.HandleSearchCollection(recorder, request)

		if recorder.Code != http.StatusNotFound {
			t.Fatalf("expected status 404, got %d: %s", recorder.Code, recorder.Body.String())
		}
		if code := errorCode(t, recorder); code != "HAIRBALL_NOT_FOUND" {
			t.Errorf("expected HAIRBALL_NOT_FOUND, got %s", code)
		}
	})

	t.Run("given vector dim mismatch, then returns 400 with HAIRBALL_DIM_MISMATCH", func(t *testing.T) {
		server := apiTestSetup(t)
		name := "api_test_search_dim_mismatch"
		defer func() { _ = ffi.Drop(name) }()
		if err := ffi.Create(name, 3, ffi.MetricL2, ""); err != nil {
			t.Fatalf("create failed: %v", err)
		}

		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/v1/collections/"+name+"/search", strings.NewReader(`{"vector":[1.0,0.0],"top_k":10}`))
		request.SetPathValue("name", name)

		server.HandleSearchCollection(recorder, request)

		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d: %s", recorder.Code, recorder.Body.String())
		}
		if code := errorCode(t, recorder); code != "HAIRBALL_DIM_MISMATCH" {
			t.Errorf("expected HAIRBALL_DIM_MISMATCH, got %s", code)
		}
	})

	t.Run("given empty vector, then returns 400 with HAIRBALL_DIM_TOO_SMALL", func(t *testing.T) {
		server := apiTestSetup(t)
		name := "api_test_search_empty_vector"
		defer func() { _ = ffi.Drop(name) }()
		if err := ffi.Create(name, 3, ffi.MetricL2, ""); err != nil {
			t.Fatalf("create failed: %v", err)
		}

		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/v1/collections/"+name+"/search", strings.NewReader(`{"vector":[],"top_k":10}`))
		request.SetPathValue("name", name)

		server.HandleSearchCollection(recorder, request)

		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d: %s", recorder.Code, recorder.Body.String())
		}
		if code := errorCode(t, recorder); code != "HAIRBALL_DIM_TOO_SMALL" {
			t.Errorf("expected HAIRBALL_DIM_TOO_SMALL, got %s", code)
		}
	})

	t.Run("given missing top_k field, then defaults to 10", func(t *testing.T) {
		server := apiTestSetup(t)
		name := "api_test_search_default_topk"
		defer func() { _ = ffi.Drop(name) }()
		if err := ffi.Create(name, 3, ffi.MetricL2, ""); err != nil {
			t.Fatalf("create failed: %v", err)
		}
		for index := range 11 {
			id := "vec_" + strconv.Itoa(index)
			if err := ffi.Insert(name, id, []float32{float32(index), 0.0, 0.0}, ""); err != nil {
				t.Fatalf("insert failed: %v", err)
			}
		}

		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/v1/collections/"+name+"/search", strings.NewReader(`{"vector":[1.0,0.0,0.0]}`))
		request.SetPathValue("name", name)

		server.HandleSearchCollection(recorder, request)

		if recorder.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
		}
		body := decodeResponse(t, recorder)
		results, ok := body["results"].([]any)
		if !ok {
			t.Fatalf("expected results array, got %v", body)
		}
		if len(results) != 10 {
			t.Errorf("expected 10 results (default top_k), got %d", len(results))
		}
	})

	t.Run("given explicit top_k zero, then returns empty results", func(t *testing.T) {
		server := apiTestSetup(t)
		name := "api_test_search_explicit_zero"
		defer func() { _ = ffi.Drop(name) }()
		if err := ffi.Create(name, 3, ffi.MetricL2, ""); err != nil {
			t.Fatalf("create failed: %v", err)
		}
		if err := ffi.Insert(name, "near", []float32{2.0, 0.0, 0.0}, ""); err != nil {
			t.Fatalf("insert failed: %v", err)
		}

		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/v1/collections/"+name+"/search", strings.NewReader(`{"vector":[1.0,0.0,0.0],"top_k":0}`))
		request.SetPathValue("name", name)

		server.HandleSearchCollection(recorder, request)

		if recorder.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
		}
		body := decodeResponse(t, recorder)
		results, ok := body["results"].([]any)
		if !ok {
			t.Fatalf("expected results array, got %v", body)
		}
		if len(results) != 0 {
			t.Errorf("expected 0 results for explicit top_k=0, got %d", len(results))
		}
	})

	t.Run("given malformed JSON body, then returns 400", func(t *testing.T) {
		server := apiTestSetup(t)
		name := "api_test_search_malformed"
		defer func() { _ = ffi.Drop(name) }()
		if err := ffi.Create(name, 3, ffi.MetricL2, ""); err != nil {
			t.Fatalf("create failed: %v", err)
		}

		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/v1/collections/"+name+"/search", strings.NewReader(`{"vector": [unclosed`))
		request.SetPathValue("name", name)

		server.HandleSearchCollection(recorder, request)

		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d: %s", recorder.Code, recorder.Body.String())
		}
	})
}

func TestHandleUpsertVector(t *testing.T) {
	t.Run("given new vector, then returns 201 with id and dim", func(t *testing.T) {
		server := apiTestSetup(t)
		name := "api_test_upsert_create"
		defer func() { _ = ffi.Drop(name) }()
		if err := ffi.Create(name, 3, ffi.MetricL2, ""); err != nil {
			t.Fatalf("create failed: %v", err)
		}

		body := strings.NewReader(`{"vector":[1.0,2.0,3.0]}`)
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPut, "/v1/collections/"+name+"/vectors/doc1", body)
		request.SetPathValue("name", name)
		request.SetPathValue("id", "doc1")

		server.HandleUpsertVector(recorder, request)

		if recorder.Code != http.StatusCreated {
			t.Fatalf("expected status 201 on first upsert, got %d: %s", recorder.Code, recorder.Body.String())
		}
		response := decodeResponse(t, recorder)
		if response["id"] != "doc1" {
			t.Errorf("expected id 'doc1', got %v", response["id"])
		}
		if response["dim"] != float64(3) {
			t.Errorf("expected dim 3, got %v", response["dim"])
		}
	})

	t.Run("given existing vector, then returns 200 with id and dim", func(t *testing.T) {
		server := apiTestSetup(t)
		name := "api_test_upsert_update"
		defer func() { _ = ffi.Drop(name) }()
		if err := ffi.Create(name, 3, ffi.MetricL2, ""); err != nil {
			t.Fatalf("create failed: %v", err)
		}
		if err := ffi.Insert(name, "doc1", []float32{1.0, 0.0, 0.0}, ""); err != nil {
			t.Fatalf("seed insert failed: %v", err)
		}

		body := strings.NewReader(`{"vector":[0.0,1.0,0.0]}`)
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPut, "/v1/collections/"+name+"/vectors/doc1", body)
		request.SetPathValue("name", name)
		request.SetPathValue("id", "doc1")

		server.HandleUpsertVector(recorder, request)

		if recorder.Code != http.StatusOK {
			t.Fatalf("expected status 200 on update, got %d: %s", recorder.Code, recorder.Body.String())
		}
		response := decodeResponse(t, recorder)
		if response["id"] != "doc1" {
			t.Errorf("expected id 'doc1', got %v", response["id"])
		}
		if response["dim"] != float64(3) {
			t.Errorf("expected dim 3, got %v", response["dim"])
		}
	})

	t.Run("given invalid JSON body, then returns 400 with HAIRBALL_INVALID_NAME", func(t *testing.T) {
		server := apiTestSetup(t)
		name := "api_test_upsert_bad_json"
		defer func() { _ = ffi.Drop(name) }()
		if err := ffi.Create(name, 3, ffi.MetricL2, ""); err != nil {
			t.Fatalf("create failed: %v", err)
		}

		body := strings.NewReader(`{not json`)
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPut, "/v1/collections/"+name+"/vectors/doc1", body)
		request.SetPathValue("name", name)
		request.SetPathValue("id", "doc1")

		server.HandleUpsertVector(recorder, request)

		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d: %s", recorder.Code, recorder.Body.String())
		}
		if code := errorCode(t, recorder); code != "HAIRBALL_INVALID_NAME" {
			t.Errorf("expected HAIRBALL_INVALID_NAME, got %s", code)
		}
	})

	t.Run("given empty vector, then returns 400 with HAIRBALL_DIM_TOO_SMALL", func(t *testing.T) {
		server := apiTestSetup(t)
		name := "api_test_upsert_empty"
		defer func() { _ = ffi.Drop(name) }()
		if err := ffi.Create(name, 3, ffi.MetricL2, ""); err != nil {
			t.Fatalf("create failed: %v", err)
		}

		body := strings.NewReader(`{"vector":[]}`)
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPut, "/v1/collections/"+name+"/vectors/doc1", body)
		request.SetPathValue("name", name)
		request.SetPathValue("id", "doc1")

		server.HandleUpsertVector(recorder, request)

		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d: %s", recorder.Code, recorder.Body.String())
		}
		if code := errorCode(t, recorder); code != "HAIRBALL_DIM_TOO_SMALL" {
			t.Errorf("expected HAIRBALL_DIM_TOO_SMALL, got %s", code)
		}
	})

	t.Run("given dim mismatch, then returns 400 with HAIRBALL_DIM_MISMATCH", func(t *testing.T) {
		server := apiTestSetup(t)
		name := "api_test_upsert_dim"
		defer func() { _ = ffi.Drop(name) }()
		if err := ffi.Create(name, 3, ffi.MetricL2, ""); err != nil {
			t.Fatalf("create failed: %v", err)
		}

		body := strings.NewReader(`{"vector":[1.0,2.0]}`)
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPut, "/v1/collections/"+name+"/vectors/doc1", body)
		request.SetPathValue("name", name)
		request.SetPathValue("id", "doc1")

		server.HandleUpsertVector(recorder, request)

		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d: %s", recorder.Code, recorder.Body.String())
		}
		if code := errorCode(t, recorder); code != "HAIRBALL_DIM_MISMATCH" {
			t.Errorf("expected HAIRBALL_DIM_MISMATCH, got %s", code)
		}
	})

	t.Run("given nonexistent collection, then returns 404 with HAIRBALL_NOT_FOUND", func(t *testing.T) {
		server := apiTestSetup(t)

		body := strings.NewReader(`{"vector":[1.0,2.0,3.0]}`)
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPut, "/v1/collections/no_such_api_upsert_clowder/vectors/doc1", body)
		request.SetPathValue("name", "no_such_api_upsert_clowder")
		request.SetPathValue("id", "doc1")

		server.HandleUpsertVector(recorder, request)

		if recorder.Code != http.StatusNotFound {
			t.Fatalf("expected status 404, got %d: %s", recorder.Code, recorder.Body.String())
		}
		if code := errorCode(t, recorder); code != "HAIRBALL_NOT_FOUND" {
			t.Errorf("expected HAIRBALL_NOT_FOUND, got %s", code)
		}
	})
}

func TestHandleInsertVector(t *testing.T) {
	t.Run("given valid body, then returns 201 with id and dim echo", func(t *testing.T) {
		server := apiTestSetup(t)
		name := "api_test_insert_happy"
		defer func() { _ = ffi.Drop(name) }()
		if err := ffi.Create(name, 3, ffi.MetricL2, ""); err != nil {
			t.Fatalf("create failed: %v", err)
		}

		body := strings.NewReader(`{"id":"doc1","vector":[1.0,2.0,3.0]}`)
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/v1/collections/"+name+"/vectors", body)
		request.SetPathValue("name", name)

		server.HandleInsertVector(recorder, request)

		if recorder.Code != http.StatusCreated {
			t.Fatalf("expected status 201, got %d: %s", recorder.Code, recorder.Body.String())
		}
		response := decodeResponse(t, recorder)
		if response["id"] != "doc1" {
			t.Errorf("expected id 'doc1', got %v", response["id"])
		}
		if response["dim"] != float64(3) {
			t.Errorf("expected dim 3, got %v", response["dim"])
		}
	})

	t.Run("given valid body with metadata, then returns 201 and accepts metadata field", func(t *testing.T) {
		server := apiTestSetup(t)
		name := "api_test_insert_with_metadata"
		defer func() { _ = ffi.Drop(name) }()
		if err := ffi.Create(name, 2, ffi.MetricCosine, ""); err != nil {
			t.Fatalf("create failed: %v", err)
		}

		body := strings.NewReader(`{"id":"doc1","vector":[1.0,0.0],"metadata":"{\"author\":\"alice\"}"}`)
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/v1/collections/"+name+"/vectors", body)
		request.SetPathValue("name", name)

		server.HandleInsertVector(recorder, request)

		if recorder.Code != http.StatusCreated {
			t.Fatalf("expected status 201, got %d: %s", recorder.Code, recorder.Body.String())
		}
	})

	t.Run("given missing id, then returns 400 with HAIRBALL_INVALID_NAME", func(t *testing.T) {
		server := apiTestSetup(t)
		name := "api_test_insert_no_id"
		defer func() { _ = ffi.Drop(name) }()
		if err := ffi.Create(name, 3, ffi.MetricL2, ""); err != nil {
			t.Fatalf("create failed: %v", err)
		}

		body := strings.NewReader(`{"vector":[1.0,2.0,3.0]}`)
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/v1/collections/"+name+"/vectors", body)
		request.SetPathValue("name", name)

		server.HandleInsertVector(recorder, request)

		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d: %s", recorder.Code, recorder.Body.String())
		}
		if code := errorCode(t, recorder); code != "HAIRBALL_INVALID_NAME" {
			t.Errorf("expected HAIRBALL_INVALID_NAME, got %s", code)
		}
	})

	t.Run("given empty id string, then returns 400 with HAIRBALL_INVALID_NAME", func(t *testing.T) {
		server := apiTestSetup(t)
		name := "api_test_insert_empty_id"
		defer func() { _ = ffi.Drop(name) }()
		if err := ffi.Create(name, 3, ffi.MetricL2, ""); err != nil {
			t.Fatalf("create failed: %v", err)
		}

		body := strings.NewReader(`{"id":"","vector":[1.0,2.0,3.0]}`)
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/v1/collections/"+name+"/vectors", body)
		request.SetPathValue("name", name)

		server.HandleInsertVector(recorder, request)

		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d: %s", recorder.Code, recorder.Body.String())
		}
		if code := errorCode(t, recorder); code != "HAIRBALL_INVALID_NAME" {
			t.Errorf("expected HAIRBALL_INVALID_NAME, got %s", code)
		}
	})

	t.Run("given vector dim mismatch with collection, then returns 400 with HAIRBALL_DIM_MISMATCH", func(t *testing.T) {
		server := apiTestSetup(t)
		name := "api_test_insert_dim_mismatch"
		defer func() { _ = ffi.Drop(name) }()
		if err := ffi.Create(name, 3, ffi.MetricL2, ""); err != nil {
			t.Fatalf("create failed: %v", err)
		}

		body := strings.NewReader(`{"id":"doc1","vector":[1.0,2.0]}`)
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/v1/collections/"+name+"/vectors", body)
		request.SetPathValue("name", name)

		server.HandleInsertVector(recorder, request)

		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d: %s", recorder.Code, recorder.Body.String())
		}
		if code := errorCode(t, recorder); code != "HAIRBALL_DIM_MISMATCH" {
			t.Errorf("expected HAIRBALL_DIM_MISMATCH, got %s", code)
		}
	})

	t.Run("given missing collection, then returns 404 with HAIRBALL_NOT_FOUND", func(t *testing.T) {
		server := apiTestSetup(t)
		name := "api_test_insert_no_clowder"

		body := strings.NewReader(`{"id":"doc1","vector":[1.0,2.0,3.0]}`)
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/v1/collections/"+name+"/vectors", body)
		request.SetPathValue("name", name)

		server.HandleInsertVector(recorder, request)

		if recorder.Code != http.StatusNotFound {
			t.Fatalf("expected status 404, got %d: %s", recorder.Code, recorder.Body.String())
		}
		if code := errorCode(t, recorder); code != "HAIRBALL_NOT_FOUND" {
			t.Errorf("expected HAIRBALL_NOT_FOUND, got %s", code)
		}
	})

	t.Run("given empty vector, then returns 400 with HAIRBALL_DIM_TOO_SMALL", func(t *testing.T) {
		server := apiTestSetup(t)
		name := "api_test_insert_empty_vec"
		defer func() { _ = ffi.Drop(name) }()
		if err := ffi.Create(name, 3, ffi.MetricL2, ""); err != nil {
			t.Fatalf("create failed: %v", err)
		}

		body := strings.NewReader(`{"id":"doc1","vector":[]}`)
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/v1/collections/"+name+"/vectors", body)
		request.SetPathValue("name", name)

		server.HandleInsertVector(recorder, request)

		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d: %s", recorder.Code, recorder.Body.String())
		}
		if code := errorCode(t, recorder); code != "HAIRBALL_DIM_TOO_SMALL" {
			t.Errorf("expected HAIRBALL_DIM_TOO_SMALL, got %s", code)
		}
	})

	t.Run("given malformed JSON body, then returns 400", func(t *testing.T) {
		server := apiTestSetup(t)
		name := "api_test_insert_malformed"
		defer func() { _ = ffi.Drop(name) }()
		if err := ffi.Create(name, 3, ffi.MetricL2, ""); err != nil {
			t.Fatalf("create failed: %v", err)
		}

		body := strings.NewReader(`{"id": "unclosed`)
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/v1/collections/"+name+"/vectors", body)
		request.SetPathValue("name", name)

		server.HandleInsertVector(recorder, request)

		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d: %s", recorder.Code, recorder.Body.String())
		}
	})
}

func TestHandleGetVector(t *testing.T) {
	t.Run("given existing vector with metadata, then returns 200 with id, vector, and metadata", func(t *testing.T) {
		server := apiTestSetup(t)
		name := "api_test_get_with_meta"
		defer func() { _ = ffi.Drop(name) }()
		if err := ffi.Create(name, 3, ffi.MetricL2, ""); err != nil {
			t.Fatalf("create failed: %v", err)
		}
		if err := ffi.Insert(name, "doc1", []float32{1.0, 2.0, 3.0}, `{"author":"alice"}`); err != nil {
			t.Fatalf("insert failed: %v", err)
		}

		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/v1/collections/"+name+"/vectors/doc1", nil)
		request.SetPathValue("name", name)
		request.SetPathValue("id", "doc1")

		server.HandleGetVector(recorder, request)

		if recorder.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
		}
		response := decodeResponse(t, recorder)
		if response["id"] != "doc1" {
			t.Errorf("expected id 'doc1', got %v", response["id"])
		}
		if metadata, ok := response["metadata"].(string); !ok || metadata != `{"author":"alice"}` {
			t.Errorf("expected metadata %q, got %v (present=%v)", `{"author":"alice"}`, response["metadata"], ok)
		}
	})

	t.Run("given existing vector without metadata, then returns 200 with id and vector only (no metadata field)", func(t *testing.T) {
		server := apiTestSetup(t)
		name := "api_test_get_no_meta"
		defer func() { _ = ffi.Drop(name) }()
		if err := ffi.Create(name, 3, ffi.MetricL2, ""); err != nil {
			t.Fatalf("create failed: %v", err)
		}
		if err := ffi.Insert(name, "doc1", []float32{1.0, 2.0, 3.0}, ""); err != nil {
			t.Fatalf("insert failed: %v", err)
		}

		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/v1/collections/"+name+"/vectors/doc1", nil)
		request.SetPathValue("name", name)
		request.SetPathValue("id", "doc1")

		server.HandleGetVector(recorder, request)

		if recorder.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
		}
		response := decodeResponse(t, recorder)
		if _, exists := response["metadata"]; exists {
			t.Errorf("expected metadata field to be absent (omitempty), got present: %v", response["metadata"])
		}
	})

	t.Run("given missing collection, then returns 404 with HAIRBALL_NOT_FOUND", func(t *testing.T) {
		server := apiTestSetup(t)
		name := "api_test_get_no_clowder"

		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/v1/collections/"+name+"/vectors/doc1", nil)
		request.SetPathValue("name", name)
		request.SetPathValue("id", "doc1")

		server.HandleGetVector(recorder, request)

		if recorder.Code != http.StatusNotFound {
			t.Fatalf("expected status 404, got %d: %s", recorder.Code, recorder.Body.String())
		}
		if code := errorCode(t, recorder); code != "HAIRBALL_NOT_FOUND" {
			t.Errorf("expected HAIRBALL_NOT_FOUND, got %s", code)
		}
	})

	t.Run("given missing vector id, then returns 404 with HAIRBALL_NOT_FOUND", func(t *testing.T) {
		server := apiTestSetup(t)
		name := "api_test_get_no_id"
		defer func() { _ = ffi.Drop(name) }()
		if err := ffi.Create(name, 3, ffi.MetricL2, ""); err != nil {
			t.Fatalf("create failed: %v", err)
		}

		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/v1/collections/"+name+"/vectors/ghost", nil)
		request.SetPathValue("name", name)
		request.SetPathValue("id", "ghost")

		server.HandleGetVector(recorder, request)

		if recorder.Code != http.StatusNotFound {
			t.Fatalf("expected status 404, got %d: %s", recorder.Code, recorder.Body.String())
		}
		if code := errorCode(t, recorder); code != "HAIRBALL_NOT_FOUND" {
			t.Errorf("expected HAIRBALL_NOT_FOUND, got %s", code)
		}
	})

	t.Run("given cosine metric collection, then returns the cosine-normalized vector (unit length)", func(t *testing.T) {
		server := apiTestSetup(t)
		name := "api_test_get_cosine"
		defer func() { _ = ffi.Drop(name) }()
		if err := ffi.Create(name, 3, ffi.MetricCosine, ""); err != nil {
			t.Fatalf("create failed: %v", err)
		}
		if err := ffi.Insert(name, "doc1", []float32{1.0, 2.0, 3.0}, ""); err != nil {
			t.Fatalf("insert failed: %v", err)
		}

		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/v1/collections/"+name+"/vectors/doc1", nil)
		request.SetPathValue("name", name)
		request.SetPathValue("id", "doc1")

		server.HandleGetVector(recorder, request)

		if recorder.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
		}
		response := decodeResponse(t, recorder)
		vectorRaw, ok := response["vector"].([]interface{})
		if !ok {
			t.Fatalf("expected vector to be []interface{}, got %T: %v", response["vector"], response["vector"])
		}
		lengthSquared := 0.0
		for _, component := range vectorRaw {
			componentFloat, ok := component.(float64)
			if !ok {
				t.Fatalf("expected vector component to be float64, got %T: %v", component, component)
			}
			lengthSquared += componentFloat * componentFloat
		}
		if math.Abs(lengthSquared-1.0) > 0.0001 {
			t.Errorf("expected cosine-normalized vector (unit length, length^2 ~= 1.0), got length^2 = %v", lengthSquared)
		}
	})
}

func TestHandleDeleteVector(t *testing.T) {
	t.Run("given existing vector, then returns 204 with empty body", func(t *testing.T) {
		server := apiTestSetup(t)
		name := "api_test_delete_existing"
		defer func() { _ = ffi.Drop(name) }()
		if err := ffi.Create(name, 3, ffi.MetricL2, ""); err != nil {
			t.Fatalf("create failed: %v", err)
		}
		if err := ffi.Insert(name, "doc1", []float32{1.0, 2.0, 3.0}, ""); err != nil {
			t.Fatalf("insert failed: %v", err)
		}

		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodDelete, "/v1/collections/"+name+"/vectors/doc1", nil)
		request.SetPathValue("name", name)
		request.SetPathValue("id", "doc1")

		server.HandleDeleteVector(recorder, request)

		if recorder.Code != http.StatusNoContent {
			t.Fatalf("expected status 204, got %d: %s", recorder.Code, recorder.Body.String())
		}
		if recorder.Body.Len() != 0 {
			t.Errorf("expected empty body for 204, got %d bytes: %s", recorder.Body.Len(), recorder.Body.String())
		}
	})

	t.Run("given nonexistent collection, then returns 404 with HAIRBALL_NOT_FOUND", func(t *testing.T) {
		server := apiTestSetup(t)
		name := "api_test_delete_no_clowder"

		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodDelete, "/v1/collections/"+name+"/vectors/doc1", nil)
		request.SetPathValue("name", name)
		request.SetPathValue("id", "doc1")

		server.HandleDeleteVector(recorder, request)

		if recorder.Code != http.StatusNotFound {
			t.Fatalf("expected status 404, got %d: %s", recorder.Code, recorder.Body.String())
		}
		if code := errorCode(t, recorder); code != "HAIRBALL_NOT_FOUND" {
			t.Errorf("expected HAIRBALL_NOT_FOUND, got %s", code)
		}
	})

	t.Run("given existing collection but unknown id, then returns 404 with HAIRBALL_NOT_FOUND", func(t *testing.T) {
		server := apiTestSetup(t)
		name := "api_test_delete_no_id"
		defer func() { _ = ffi.Drop(name) }()
		if err := ffi.Create(name, 3, ffi.MetricL2, ""); err != nil {
			t.Fatalf("create failed: %v", err)
		}

		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodDelete, "/v1/collections/"+name+"/vectors/ghost", nil)
		request.SetPathValue("name", name)
		request.SetPathValue("id", "ghost")

		server.HandleDeleteVector(recorder, request)

		if recorder.Code != http.StatusNotFound {
			t.Fatalf("expected status 404, got %d: %s", recorder.Code, recorder.Body.String())
		}
		if code := errorCode(t, recorder); code != "HAIRBALL_NOT_FOUND" {
			t.Errorf("expected HAIRBALL_NOT_FOUND, got %s", code)
		}
	})

	t.Run("given deleted vector, then subsequent get returns 404 with HAIRBALL_NOT_FOUND", func(t *testing.T) {
		server := apiTestSetup(t)
		name := "api_test_delete_then_get"
		defer func() { _ = ffi.Drop(name) }()
		if err := ffi.Create(name, 3, ffi.MetricL2, ""); err != nil {
			t.Fatalf("create failed: %v", err)
		}
		if err := ffi.Insert(name, "doc1", []float32{1.0, 2.0, 3.0}, ""); err != nil {
			t.Fatalf("insert failed: %v", err)
		}

		deleteRecorder := httptest.NewRecorder()
		deleteRequest := httptest.NewRequest(http.MethodDelete, "/v1/collections/"+name+"/vectors/doc1", nil)
		deleteRequest.SetPathValue("name", name)
		deleteRequest.SetPathValue("id", "doc1")
		server.HandleDeleteVector(deleteRecorder, deleteRequest)
		if deleteRecorder.Code != http.StatusNoContent {
			t.Fatalf("setup: expected delete to return 204, got %d: %s", deleteRecorder.Code, deleteRecorder.Body.String())
		}

		getRecorder := httptest.NewRecorder()
		getRequest := httptest.NewRequest(http.MethodGet, "/v1/collections/"+name+"/vectors/doc1", nil)
		getRequest.SetPathValue("name", name)
		getRequest.SetPathValue("id", "doc1")
		server.HandleGetVector(getRecorder, getRequest)

		if getRecorder.Code != http.StatusNotFound {
			t.Fatalf("expected get-after-delete to return 404, got %d: %s", getRecorder.Code, getRecorder.Body.String())
		}
		if code := errorCode(t, getRecorder); code != "HAIRBALL_NOT_FOUND" {
			t.Errorf("expected HAIRBALL_NOT_FOUND, got %s", code)
		}
	})
}

func TestHandleInsertManyVector(t *testing.T) {
	t.Run("given valid body of three vectors, then returns 201 with ids, dim, and inserted count", func(t *testing.T) {
		server := apiTestSetup(t)
		name := "api_test_batch_happy"
		defer func() { _ = ffi.Drop(name) }()
		if err := ffi.Create(name, 3, ffi.MetricL2, ""); err != nil {
			t.Fatalf("create failed: %v", err)
		}

		body := strings.NewReader(`[{"id":"doc1","vector":[1.0,2.0,3.0]},{"id":"doc2","vector":[4.0,5.0,6.0]},{"id":"doc3","vector":[7.0,8.0,9.0]}]`)
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/v1/collections/"+name+"/vectors/batch", body)
		request.SetPathValue("name", name)

		server.HandleInsertManyVector(recorder, request)

		if recorder.Code != http.StatusCreated {
			t.Fatalf("expected status 201, got %d: %s", recorder.Code, recorder.Body.String())
		}
		response := decodeResponse(t, recorder)
		if response["inserted"] != float64(3) {
			t.Errorf("expected inserted=3, got %v", response["inserted"])
		}
		if response["dim"] != float64(3) {
			t.Errorf("expected dim=3, got %v", response["dim"])
		}
		ids, ok := response["ids"].([]any)
		if !ok || len(ids) != 3 {
			t.Fatalf("expected ids to be a 3-element array, got %v", response["ids"])
		}
		if ids[0] != "doc1" || ids[1] != "doc2" || ids[2] != "doc3" {
			t.Errorf("expected ids [doc1 doc2 doc3], got %v", ids)
		}
	})

	t.Run("given empty body, then returns 400 with HAIRBALL_INVALID_NAME", func(t *testing.T) {
		server := apiTestSetup(t)
		name := "api_test_batch_empty"

		body := strings.NewReader(`[]`)
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/v1/collections/"+name+"/vectors/batch", body)
		request.SetPathValue("name", name)

		server.HandleInsertManyVector(recorder, request)

		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d: %s", recorder.Code, recorder.Body.String())
		}
		if code := errorCode(t, recorder); code != "HAIRBALL_INVALID_NAME" {
			t.Errorf("expected HAIRBALL_INVALID_NAME, got %s", code)
		}
	})

	t.Run("given batch size above cap, then returns 400 with HAIRBALL_INVALID_NAME", func(t *testing.T) {
		server := apiTestSetup(t)
		name := "api_test_batch_oversize"

		var items []string
		for index := 0; index < 10001; index++ {
			items = append(items, fmt.Sprintf(`{"id":"doc%d","vector":[1.0,2.0,3.0]}`, index))
		}
		body := strings.NewReader("[" + strings.Join(items, ",") + "]")
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/v1/collections/"+name+"/vectors/batch", body)
		request.SetPathValue("name", name)

		server.HandleInsertManyVector(recorder, request)

		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d: %s", recorder.Code, recorder.Body.String())
		}
		if code := errorCode(t, recorder); code != "HAIRBALL_INVALID_NAME" {
			t.Errorf("expected HAIRBALL_INVALID_NAME, got %s", code)
		}
	})

	t.Run("given nonexistent collection, then returns 404 with HAIRBALL_NOT_FOUND", func(t *testing.T) {
		server := apiTestSetup(t)
		name := "no_such_api_batch_clowder"

		body := strings.NewReader(`[{"id":"doc1","vector":[1.0,2.0,3.0]}]`)
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/v1/collections/"+name+"/vectors/batch", body)
		request.SetPathValue("name", name)

		server.HandleInsertManyVector(recorder, request)

		if recorder.Code != http.StatusNotFound {
			t.Fatalf("expected status 404, got %d: %s", recorder.Code, recorder.Body.String())
		}
		if code := errorCode(t, recorder); code != "HAIRBALL_NOT_FOUND" {
			t.Errorf("expected HAIRBALL_NOT_FOUND, got %s", code)
		}
	})

	t.Run("given item with empty id, then returns 400 with index in message", func(t *testing.T) {
		server := apiTestSetup(t)
		name := "api_test_batch_empty_id"
		defer func() { _ = ffi.Drop(name) }()
		if err := ffi.Create(name, 3, ffi.MetricL2, ""); err != nil {
			t.Fatalf("create failed: %v", err)
		}

		body := strings.NewReader(`[{"id":"doc1","vector":[1.0,2.0,3.0]},{"id":"","vector":[4.0,5.0,6.0]}]`)
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/v1/collections/"+name+"/vectors/batch", body)
		request.SetPathValue("name", name)

		server.HandleInsertManyVector(recorder, request)

		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d: %s", recorder.Code, recorder.Body.String())
		}
		if code := errorCode(t, recorder); code != "HAIRBALL_INVALID_NAME" {
			t.Errorf("expected HAIRBALL_INVALID_NAME, got %s", code)
		}
	})

	t.Run("given item with wrong dim, then returns 400 with HAIRBALL_DIM_MISMATCH", func(t *testing.T) {
		server := apiTestSetup(t)
		name := "api_test_batch_dim"
		defer func() { _ = ffi.Drop(name) }()
		if err := ffi.Create(name, 3, ffi.MetricL2, ""); err != nil {
			t.Fatalf("create failed: %v", err)
		}

		body := strings.NewReader(`[{"id":"doc1","vector":[1.0,2.0,3.0]},{"id":"doc2","vector":[4.0,5.0]}]`)
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/v1/collections/"+name+"/vectors/batch", body)
		request.SetPathValue("name", name)

		server.HandleInsertManyVector(recorder, request)

		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d: %s", recorder.Code, recorder.Body.String())
		}
		if code := errorCode(t, recorder); code != "HAIRBALL_DIM_MISMATCH" {
			t.Errorf("expected HAIRBALL_DIM_MISMATCH, got %s", code)
		}
	})
}
