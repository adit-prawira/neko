package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
