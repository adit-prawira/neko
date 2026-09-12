package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adit-prawira/neko/internal/ffi"
)

func routerTestSetup(t *testing.T) *Server {
	t.Helper()
	dataDirectory := filepath.Join(os.TempDir(), "neko_test_router")
	if err := os.MkdirAll(dataDirectory, 0o755); err != nil {
		t.Fatalf("failed to create data directory: %v", err)
	}
	server := NewServer()
	if err := server.InitEngine(dataDirectory); err != nil {
		t.Fatalf("engine init failed: %v", err)
	}
	return server
}

func TestSearchRoute(t *testing.T) {
	t.Run("given POST through mux, then path extracts collection name", func(t *testing.T) {
		server := routerTestSetup(t)
		name := "router_test_search_route"
		if err := server.InitEngine(filepath.Join(os.TempDir(), "neko_test_router")); err != nil {
			t.Fatalf("re-init engine: %v", err)
		}
		handler := buildRoutes(server)

		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/v1/collections/"+name+"/search", strings.NewReader(`{"vector":[1.0,0.0,0.0],"top_k":2}`))

		handler.ServeHTTP(recorder, request)

		if recorder.Code != http.StatusNotFound {
			t.Fatalf("expected 404 (collection does not exist), got %d: %s", recorder.Code, recorder.Body.String())
		}
		if code := errorCode(t, recorder); code != "HAIRBALL_NOT_FOUND" {
			t.Errorf("expected HAIRBALL_NOT_FOUND, got %s", code)
		}
	})

	t.Run("given GET on search endpoint, then returns 405 from mux default", func(t *testing.T) {
		server := routerTestSetup(t)
		handler := buildRoutes(server)

		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/v1/collections/anything/search", nil)

		handler.ServeHTTP(recorder, request)

		if recorder.Code == http.StatusOK {
			t.Errorf("expected non-200 for wrong method, got %d", recorder.Code)
		}
	})
}

func TestInsertVectorRoute(t *testing.T) {
	t.Run("given POST through mux, then dispatches to handler and returns 201 for valid insert", func(t *testing.T) {
		// Creating the collection first ensures the handler (not the catchall)
		// is invoked. If the route were unregistered, the catchall would return
		// 404 HAIRBALL_NOT_FOUND with a 'is not found' message.
		server := routerTestSetup(t)
		name := "router_test_insert_route"
		defer func() { _ = ffi.Drop(name) }()
		if err := ffi.Create(name, 3, ffi.MetricL2, ""); err != nil {
			t.Fatalf("create failed: %v", err)
		}
		handler := buildRoutes(server)

		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/v1/collections/"+name+"/vectors", strings.NewReader(`{"id":"doc1","vector":[1.0,0.0,0.0]}`))

		handler.ServeHTTP(recorder, request)

		if recorder.Code != http.StatusCreated {
			t.Fatalf("expected 201 (route dispatched to handler), got %d: %s", recorder.Code, recorder.Body.String())
		}
	})

	t.Run("given GET on insert endpoint, then falls through to catchall and returns 404", func(t *testing.T) {
		// Pattern 'POST /v1/collections/{name}/vectors' only matches POST.
		// A GET request to the same path falls through to the catchall.
		server := routerTestSetup(t)
		handler := buildRoutes(server)

		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/v1/collections/some_name/vectors", nil)

		handler.ServeHTTP(recorder, request)

		if recorder.Code != http.StatusNotFound {
			t.Errorf("expected 404 (GET on POST-only route), got %d", recorder.Code)
		}
	})
}

func TestGetVectorRoute(t *testing.T) {
	t.Run("given GET through mux, then dispatches to handler and returns 200 for valid get", func(t *testing.T) {
		// Creating the collection and inserting a vector first ensures the
		// handler (not the catchall) is invoked. If the route were unregistered,
		// the catchall would return 404 HAIRBALL_NOT_FOUND with a 'is not found'
		// message.
		server := routerTestSetup(t)
		name := "router_test_get_route"
		defer func() { _ = ffi.Drop(name) }()
		if err := ffi.Create(name, 3, ffi.MetricL2, ""); err != nil {
			t.Fatalf("create failed: %v", err)
		}
		if err := ffi.Insert(name, "doc1", []float32{1.0, 0.0, 0.0}, ""); err != nil {
			t.Fatalf("insert failed: %v", err)
		}
		handler := buildRoutes(server)

		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/v1/collections/"+name+"/vectors/doc1", nil)

		handler.ServeHTTP(recorder, request)

		if recorder.Code != http.StatusOK {
			t.Fatalf("expected 200 (route dispatched to handler), got %d: %s", recorder.Code, recorder.Body.String())
		}
	})

	t.Run("given POST on get endpoint, then falls through to catchall and returns 404", func(t *testing.T) {
		// Pattern 'GET /v1/collections/{name}/vectors/{id}' only matches GET.
		// A POST request to the same path falls through to the catchall.
		server := routerTestSetup(t)
		handler := buildRoutes(server)

		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/v1/collections/some_name/vectors/doc1", nil)

		handler.ServeHTTP(recorder, request)

		if recorder.Code != http.StatusNotFound {
			t.Errorf("expected 404 (POST on GET-only route), got %d", recorder.Code)
		}
	})
}

func TestUpsertVectorRoute(t *testing.T) {
	t.Run("given PUT through mux, then dispatches to handler and returns 201 for new vector", func(t *testing.T) {
		// Creating the collection first ensures the handler (not the catchall)
		// is invoked. If the route were unregistered, the catchall would return
		// 404 HAIRBALL_NOT_FOUND with a 'is not found' message.
		server := routerTestSetup(t)
		name := "router_test_upsert_route"
		defer func() { _ = ffi.Drop(name) }()
		if err := ffi.Create(name, 3, ffi.MetricL2, ""); err != nil {
			t.Fatalf("create failed: %v", err)
		}
		handler := buildRoutes(server)

		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPut, "/v1/collections/"+name+"/vectors/doc1", strings.NewReader(`{"vector":[1.0,0.0,0.0]}`))

		handler.ServeHTTP(recorder, request)

		if recorder.Code != http.StatusCreated {
			t.Fatalf("expected 201 (route dispatched to handler), got %d: %s", recorder.Code, recorder.Body.String())
		}
	})

	t.Run("given GET on upsert endpoint, then falls through to catchall and returns 404", func(t *testing.T) {
		// Pattern 'PUT /v1/collections/{name}/vectors/{id}' only matches PUT.
		// A GET request to the same path falls through to the catchall.
		server := routerTestSetup(t)
		handler := buildRoutes(server)

		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/v1/collections/some_name/vectors/doc1", nil)

		handler.ServeHTTP(recorder, request)

		if recorder.Code != http.StatusNotFound {
			t.Errorf("expected 404 (GET on PUT-only route), got %d", recorder.Code)
		}
	})
}

func TestDeleteVectorRoute(t *testing.T) {
	t.Run("given DELETE through mux, then dispatches to handler and returns 204 for existing vector", func(t *testing.T) {
		// Creating the collection and inserting a vector first ensures the
		// handler (not the catchall) is invoked. If the route were unregistered,
		// the catchall would return 404 HAIRBALL_NOT_FOUND with a 'is not found'
		// message.
		server := routerTestSetup(t)
		name := "router_test_delete_route"
		defer func() { _ = ffi.Drop(name) }()
		if err := ffi.Create(name, 3, ffi.MetricL2, ""); err != nil {
			t.Fatalf("create failed: %v", err)
		}
		if err := ffi.Insert(name, "doc1", []float32{1.0, 0.0, 0.0}, ""); err != nil {
			t.Fatalf("insert failed: %v", err)
		}
		handler := buildRoutes(server)

		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodDelete, "/v1/collections/"+name+"/vectors/doc1", nil)

		handler.ServeHTTP(recorder, request)

		if recorder.Code != http.StatusNoContent {
			t.Fatalf("expected 204 (route dispatched to handler), got %d: %s", recorder.Code, recorder.Body.String())
		}
	})

	t.Run("given POST on delete endpoint, then falls through to catchall and returns 404", func(t *testing.T) {
		// Pattern 'DELETE /v1/collections/{name}/vectors/{id}' only matches DELETE.
		// A POST request to the same path falls through to the catchall.
		server := routerTestSetup(t)
		handler := buildRoutes(server)

		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/v1/collections/some_name/vectors/doc1", nil)

		handler.ServeHTTP(recorder, request)

		if recorder.Code != http.StatusNotFound {
			t.Errorf("expected 404 (POST on DELETE-only route), got %d", recorder.Code)
		}
	})
}

func TestInsertManyVectorRoute(t *testing.T) {
	t.Run("given POST through mux to batch endpoint, then dispatches to handler and returns 201 for valid body", func(t *testing.T) {
		// Creating the collection first ensures the handler (not the catchall)
		// is invoked. If the route were unregistered, the catchall would return
		// 404 HAIRBALL_NOT_FOUND with a 'is not found' message.
		server := routerTestSetup(t)
		name := "router_test_batch_route"
		defer func() { _ = ffi.Drop(name) }()
		if err := ffi.Create(name, 3, ffi.MetricL2, ""); err != nil {
			t.Fatalf("create failed: %v", err)
		}
		handler := buildRoutes(server)

		body := strings.NewReader(`[{"id":"doc1","vector":[1.0,2.0,3.0]}]`)
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/v1/collections/"+name+"/vectors/batch", body)

		handler.ServeHTTP(recorder, request)

		if recorder.Code != http.StatusCreated {
			t.Fatalf("expected 201 (route dispatched to handler), got %d: %s", recorder.Code, recorder.Body.String())
		}
	})

	t.Run("given GET on batch endpoint, then falls through to catchall and returns 404", func(t *testing.T) {
		// Pattern 'POST /v1/collections/{name}/vectors/batch' only matches POST.
		// A GET request to the same path falls through to the catchall.
		server := routerTestSetup(t)
		handler := buildRoutes(server)

		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/v1/collections/some_name/vectors/batch", nil)

		handler.ServeHTTP(recorder, request)

		if recorder.Code != http.StatusNotFound {
			t.Errorf("expected 404 (GET on POST-only route), got %d", recorder.Code)
		}
	})
}
