package grpcserver

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/adit-prawira/neko/internal/ffi"
	nekov1 "github.com/adit-prawira/neko/internal/gen/neko/v1"
)

func grpcTestSetup(t *testing.T) {
	t.Helper()
	dataDirectory := filepath.Join(os.TempDir(), "neko_test_grpc")
	if err := os.MkdirAll(dataDirectory, 0o755); err != nil {
		t.Fatalf("failed to create data directory: %v", err)
	}
	if err := ffi.Init(dataDirectory); err != nil {
		t.Fatalf("engine init failed: %v", err)
	}
}

func TestCollectionServiceCreate(t *testing.T) {
	t.Run("given valid name dim and cosine metric, then collection is created with that shape", func(t *testing.T) {
		grpcTestSetup(t)
		name := "grpc_test_create_basic"
		defer func() { _ = ffi.Drop(name) }()

		service := &CollectionService{}
		req := &nekov1.CreateCollectionRequest{
			Name:   name,
			Dim:    3,
			Metric: nekov1.Metric_METRIC_COSINE,
		}

		response, err := service.Create(context.Background(), req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if response.Name != name {
			t.Errorf("response.Name: got %q, want %q", response.Name, name)
		}
		if response.Dim != 3 {
			t.Errorf("response.Dim: got %d, want 3", response.Dim)
		}
		if response.Metric != nekov1.Metric_METRIC_COSINE {
			t.Errorf("response.Metric: got %v, want METRIC_COSINE", response.Metric)
		}
	})

	t.Run("given metric unspecified, then defaults to cosine", func(t *testing.T) {
		grpcTestSetup(t)
		name := "grpc_test_create_default"
		defer func() { _ = ffi.Drop(name) }()

		service := &CollectionService{}
		req := &nekov1.CreateCollectionRequest{
			Name:   name,
			Dim:    3,
			Metric: nekov1.Metric_METRIC_UNSPECIFIED,
		}

		response, err := service.Create(context.Background(), req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if response.Metric != nekov1.Metric_METRIC_COSINE {
			t.Errorf("response.Metric: got %v, want METRIC_COSINE (default)", response.Metric)
		}
	})

	t.Run("given empty name, then returns InvalidArgument", func(t *testing.T) {
		grpcTestSetup(t)
		service := &CollectionService{}
		req := &nekov1.CreateCollectionRequest{
			Name:   "",
			Dim:    3,
			Metric: nekov1.Metric_METRIC_COSINE,
		}

		_, err := service.Create(context.Background(), req)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if status.Code(err) != codes.InvalidArgument {
			t.Errorf("got code %v, want %v", status.Code(err), codes.InvalidArgument)
		}
	})

	t.Run("given duplicate name, then returns AlreadyExists", func(t *testing.T) {
		grpcTestSetup(t)
		name := "grpc_test_create_dup"
		defer func() { _ = ffi.Drop(name) }()

		if err := ffi.Create(name, 3, ffi.MetricCosine, ""); err != nil {
			t.Fatalf("setup create failed: %v", err)
		}

		service := &CollectionService{}
		req := &nekov1.CreateCollectionRequest{
			Name:   name,
			Dim:    3,
			Metric: nekov1.Metric_METRIC_COSINE,
		}

		_, err := service.Create(context.Background(), req)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if status.Code(err) != codes.AlreadyExists {
			t.Errorf("got code %v, want %v", status.Code(err), codes.AlreadyExists)
		}
	})
}

func TestCollectionServiceList(t *testing.T) {
	t.Run("given created collection, then list returns it", func(t *testing.T) {
		grpcTestSetup(t)
		name := "grpc_test_list_basic"
		defer func() { _ = ffi.Drop(name) }()

		if err := ffi.Create(name, 3, ffi.MetricL2, ""); err != nil {
			t.Fatalf("setup create failed: %v", err)
		}

		service := &CollectionService{}
		response, err := service.List(context.Background(), nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		found := false
		for _, collection := range response.Collections {
			if collection.Name == name {
				found = true
				if collection.Dim != 3 {
					t.Errorf("Dim: got %d, want 3", collection.Dim)
				}
				if collection.Metric != nekov1.Metric_METRIC_L2 {
					t.Errorf("Metric: got %v, want METRIC_L2", collection.Metric)
				}
				break
			}
		}
		if !found {
			t.Errorf("expected collection %q to be listed, got %d collections", name, len(response.Collections))
		}
	})
}

func TestCollectionServiceGet(t *testing.T) {
	t.Run("given existing collection, then returns its stats", func(t *testing.T) {
		grpcTestSetup(t)
		name := "grpc_test_get_basic"
		defer func() { _ = ffi.Drop(name) }()

		if err := ffi.Create(name, 4, ffi.MetricDot, ""); err != nil {
			t.Fatalf("setup create failed: %v", err)
		}

		service := &CollectionService{}
		req := &nekov1.GetCollectionRequest{Name: name}

		response, err := service.Get(context.Background(), req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if response.Name != name {
			t.Errorf("Name: got %q, want %q", response.Name, name)
		}
		if response.Dim != 4 {
			t.Errorf("Dim: got %d, want 4", response.Dim)
		}
		if response.Metric != nekov1.Metric_METRIC_DOT {
			t.Errorf("Metric: got %v, want METRIC_DOT", response.Metric)
		}
	})

	t.Run("given missing collection, then returns NotFound", func(t *testing.T) {
		grpcTestSetup(t)
		service := &CollectionService{}
		req := &nekov1.GetCollectionRequest{Name: "grpc_test_get_missing"}

		_, err := service.Get(context.Background(), req)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if status.Code(err) != codes.NotFound {
			t.Errorf("got code %v, want %v", status.Code(err), codes.NotFound)
		}
	})
}

func TestCollectionServiceDrop(t *testing.T) {
	t.Run("given existing collection, then it is removed", func(t *testing.T) {
		grpcTestSetup(t)
		name := "grpc_test_drop_basic"

		if err := ffi.Create(name, 3, ffi.MetricCosine, ""); err != nil {
			t.Fatalf("setup create failed: %v", err)
		}

		service := &CollectionService{}
		req := &nekov1.DropCollectionRequest{Name: name}

		_, err := service.Drop(context.Background(), req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		_, getErr := ffi.Stats(name)
		if getErr == nil {
			t.Error("expected collection to be removed, but Stats still succeeds")
		}
	})

	t.Run("given missing collection, then returns NotFound", func(t *testing.T) {
		grpcTestSetup(t)
		service := &CollectionService{}
		req := &nekov1.DropCollectionRequest{Name: "grpc_test_drop_missing"}

		_, err := service.Drop(context.Background(), req)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if status.Code(err) != codes.NotFound {
			t.Errorf("got code %v, want %v", status.Code(err), codes.NotFound)
		}
	})
}

func TestCollectionServiceSearch(t *testing.T) {
	t.Run("given query against empty collection, then returns empty results", func(t *testing.T) {
		grpcTestSetup(t)
		name := "grpc_test_search_empty"
		defer func() { _ = ffi.Drop(name) }()

		if err := ffi.Create(name, 3, ffi.MetricL2, ""); err != nil {
			t.Fatalf("setup create failed: %v", err)
		}

		service := &CollectionService{}
		req := &nekov1.SearchRequest{
			Name: name,
			Query: &nekov1.SearchQueryParams{
				Vector: []float32{1.0, 0.0, 0.0},
				TopK:   ptrUint32(5),
			},
		}

		response, err := service.Search(context.Background(), req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(response.Results) != 0 {
			t.Errorf("expected empty results, got %d", len(response.Results))
		}
	})

	t.Run("given missing collection, then returns NotFound", func(t *testing.T) {
		grpcTestSetup(t)
		service := &CollectionService{}
		req := &nekov1.SearchRequest{
			Name: "grpc_test_search_missing",
			Query: &nekov1.SearchQueryParams{
				Vector: []float32{1.0, 0.0, 0.0},
				TopK:   ptrUint32(5),
			},
		}

		_, err := service.Search(context.Background(), req)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if status.Code(err) != codes.NotFound {
			t.Errorf("got code %v, want %v", status.Code(err), codes.NotFound)
		}
	})
}

func ptrUint32(value uint32) *uint32 {
	return &value
}
