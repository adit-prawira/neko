package api

import (
	_ "embed"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

//go:embed docs/swagger.json
var embeddedSwaggerSpec []byte

// expectedRoutes is the source of truth for the API surface.
// Every entry must exist as a route in router.go AND as a path+method
// in the generated OpenAPI spec. If either side drifts, this test fails.
//
// To add a new endpoint:
//  1. Register the route in internal/api/router.go
//  2. Add a swag annotation block above the handler in internal/api/handler.go
//  3. Add the (method, path) pair here
//  4. Run `make swag` to regenerate docs/swagger.json
//  5. Run `go test ./internal/api/...` to verify
var expectedRoutes = []struct {
	method string
	path   string
}{
	{"GET", "/health"},
	{"POST", "/v1/collections"},
	{"GET", "/v1/collections"},
	{"GET", "/v1/collections/{name}"},
	{"DELETE", "/v1/collections/{name}"},
	{"POST", "/v1/collections/{name}/search"},
	{"POST", "/v1/collections/{name}/vectors"},
	{"POST", "/v1/collections/{name}/vectors/batch"},
	{"GET", "/v1/collections/{name}/vectors/{id}"},
	{"PUT", "/v1/collections/{name}/vectors/{id}"},
	{"DELETE", "/v1/collections/{name}/vectors/{id}"},
}

type swaggerSpec struct {
	Paths map[string]map[string]json.RawMessage `json:"paths"`
}

func TestSwaggerSpecIsParseable(t *testing.T) {
	t.Run("given generated swagger.json, then it parses as valid JSON", func(t *testing.T) {
		var spec swaggerSpec
		if err := json.Unmarshal(embeddedSwaggerSpec, &spec); err != nil {
			t.Fatalf("swagger.json is not valid JSON: %v", err)
		}
		if len(spec.Paths) == 0 {
			t.Fatal("swagger.json has no paths — `make swag` may not have been run")
		}
	})
}

func TestSwaggerSpecMatchesRoutes(t *testing.T) {
	// The generated spec stores paths exactly as written in @Router
	// annotations, which match the route registrations 1:1.
	t.Run("given every expected route, then swagger.json documents it", func(t *testing.T) {
		var spec swaggerSpec
		if err := json.Unmarshal(embeddedSwaggerSpec, &spec); err != nil {
			t.Fatalf("swagger.json is not valid JSON: %v", err)
		}

		for _, route := range expectedRoutes {
			t.Run(route.method+" "+route.path, func(t *testing.T) {
				methods, ok := spec.Paths[route.path]
				if !ok {
					t.Fatalf("path %q is missing from swagger.json — add a swag annotation to the handler and run `make swag`", route.path)
				}
				if _, ok := methods[strings.ToLower(route.method)]; !ok {
					t.Fatalf("method %s on path %q is missing from swagger.json — add a swag annotation to the handler and run `make swag`", route.method, route.path)
				}
			})
		}
	})

	t.Run("given every swagger.json path, then a route is registered for it", func(t *testing.T) {
		var spec swaggerSpec
		if err := json.Unmarshal(embeddedSwaggerSpec, &spec); err != nil {
			t.Fatalf("swagger.json is not valid JSON: %v", err)
		}

		expected := make(map[string]bool, len(expectedRoutes))
		for _, route := range expectedRoutes {
			expected[route.method+"|"+route.path] = true
		}

		for path, methods := range spec.Paths {
			for method := range methods {
				key := strings.ToUpper(method) + "|" + path
				if !expected[key] {
					t.Errorf("swagger.json documents %s %s but no matching route in router.go (or the expectedRoutes list)", strings.ToUpper(method), path)
				}
			}
		}
	})

	t.Run("given swag annotations, then every documented route responds through the router", func(t *testing.T) {
		// We can't easily introspect http.ServeMux, so this subtest verifies
		// that the router at least returns a non-default response for each
		// path. The catch-all 404 handler is expected only for genuinely
		// unrecognised paths.
		server := NewServer()
		handler := buildRoutes(server)

		for _, route := range expectedRoutes {
			t.Run(route.method+" "+route.path, func(t *testing.T) {
				recorder := httptest.NewRecorder()
				request := httptest.NewRequest(route.method, route.path, nil)
				handler.ServeHTTP(recorder, request)

				// /health returns 503 (engine not initialised) or 200.
				// Everything else is expected to reach its handler. We
				// only assert that we didn't get the catch-all 404 with
				// the "route ... is not found" message.
				body := recorder.Body.String()
				if recorder.Code == http.StatusNotFound && strings.Contains(body, "is not found") {
					t.Fatalf("route %s %s fell through to the catch-all 404 — register it in router.go", route.method, route.path)
				}
			})
		}
	})
}
