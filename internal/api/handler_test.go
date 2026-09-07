package api

import (
	"encoding/json"
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
