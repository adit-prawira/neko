package grpcserver

import (
	"context"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/adit-prawira/neko/internal/ffi"
	nekov1 "github.com/adit-prawira/neko/internal/gen/neko/v1"
)

func TestVectorServiceInsert(t *testing.T) {
	t.Run("given valid name id and vector, then vector is inserted with correct dim", func(t *testing.T) {
		grpcTestSetup(t)
		name := "grpc_test_insert_basic"
		defer func() { _ = ffi.Drop(name) }()

		if err := ffi.Create(name, 3, ffi.MetricL2, ""); err != nil {
			t.Fatalf("setup create failed: %v", err)
		}

		service := &VectorService{}
		req := &nekov1.InsertVectorRequest{
			Name: name,
			Body: &nekov1.UpsertVector{
				Id:     "doc1",
				Vector: []float32{1.0, 2.0, 3.0},
			},
		}

		response, err := service.Insert(context.Background(), req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if response.Id != "doc1" {
			t.Errorf("Id: got %q, want doc1", response.Id)
		}
		if response.Dim != 3 {
			t.Errorf("Dim: got %d, want 3", response.Dim)
		}
	})

	t.Run("given empty id, then returns InvalidArgument", func(t *testing.T) {
		grpcTestSetup(t)
		name := "grpc_test_insert_empty_id"
		defer func() { _ = ffi.Drop(name) }()

		if err := ffi.Create(name, 3, ffi.MetricL2, ""); err != nil {
			t.Fatalf("setup create failed: %v", err)
		}

		service := &VectorService{}
		req := &nekov1.InsertVectorRequest{
			Name: name,
			Body: &nekov1.UpsertVector{
				Id:     "",
				Vector: []float32{1.0, 2.0, 3.0},
			},
		}

		_, err := service.Insert(context.Background(), req)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if status.Code(err) != codes.InvalidArgument {
			t.Errorf("got code %v, want %v", status.Code(err), codes.InvalidArgument)
		}
	})

	t.Run("given missing collection, then returns NotFound", func(t *testing.T) {
		grpcTestSetup(t)
		service := &VectorService{}
		req := &nekov1.InsertVectorRequest{
			Name: "grpc_test_insert_missing_clowder",
			Body: &nekov1.UpsertVector{
				Id:     "doc1",
				Vector: []float32{1.0, 2.0, 3.0},
			},
		}

		_, err := service.Insert(context.Background(), req)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if status.Code(err) != codes.NotFound {
			t.Errorf("got code %v, want %v", status.Code(err), codes.NotFound)
		}
	})
}

func TestVectorServiceInsertMany(t *testing.T) {
	t.Run("given valid batch, then all vectors are inserted", func(t *testing.T) {
		grpcTestSetup(t)
		name := "grpc_test_insert_many_basic"
		defer func() { _ = ffi.Drop(name) }()

		if err := ffi.Create(name, 3, ffi.MetricL2, ""); err != nil {
			t.Fatalf("setup create failed: %v", err)
		}

		service := &VectorService{}
		req := &nekov1.InsertManyVectorRequest{
			Name: name,
			Vectors: []*nekov1.UpsertVector{
				{Id: "doc1", Vector: []float32{1.0, 2.0, 3.0}},
				{Id: "doc2", Vector: []float32{4.0, 5.0, 6.0}},
			},
		}

		response, err := service.InsertMany(context.Background(), req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if response.Inserted != 2 {
			t.Errorf("Inserted: got %d, want 2", response.Inserted)
		}
		if len(response.Ids) != 2 {
			t.Errorf("Ids: got %d entries, want 2", len(response.Ids))
		}
	})

	t.Run("given empty vectors array, then returns InvalidArgument", func(t *testing.T) {
		grpcTestSetup(t)
		service := &VectorService{}
		req := &nekov1.InsertManyVectorRequest{
			Name:    "grpc_test_insert_many_empty",
			Vectors: []*nekov1.UpsertVector{},
		}

		_, err := service.InsertMany(context.Background(), req)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if status.Code(err) != codes.InvalidArgument {
			t.Errorf("got code %v, want %v", status.Code(err), codes.InvalidArgument)
		}
	})

	t.Run("given item with wrong dim, then returns InvalidArgument", func(t *testing.T) {
		grpcTestSetup(t)
		name := "grpc_test_insert_many_dim"
		defer func() { _ = ffi.Drop(name) }()

		if err := ffi.Create(name, 3, ffi.MetricL2, ""); err != nil {
			t.Fatalf("setup create failed: %v", err)
		}

		service := &VectorService{}
		req := &nekov1.InsertManyVectorRequest{
			Name: name,
			Vectors: []*nekov1.UpsertVector{
				{Id: "doc1", Vector: []float32{1.0, 2.0}},
			},
		}

		_, err := service.InsertMany(context.Background(), req)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if status.Code(err) != codes.InvalidArgument {
			t.Errorf("got code %v, want %v", status.Code(err), codes.InvalidArgument)
		}
	})
}

func TestVectorServiceGet(t *testing.T) {
	t.Run("given existing vector, then returns its value", func(t *testing.T) {
		grpcTestSetup(t)
		name := "grpc_test_get_vector_basic"
		defer func() { _ = ffi.Drop(name) }()

		if err := ffi.Create(name, 3, ffi.MetricL2, ""); err != nil {
			t.Fatalf("setup create failed: %v", err)
		}
		if err := ffi.Insert(name, "doc1", []float32{1.0, 2.0, 3.0}, ""); err != nil {
			t.Fatalf("setup insert failed: %v", err)
		}

		service := &VectorService{}
		req := &nekov1.GetVectorRequest{Name: name, Id: "doc1"}

		response, err := service.Get(context.Background(), req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if response.Id != "doc1" {
			t.Errorf("Id: got %q, want doc1", response.Id)
		}
		if len(response.Vector) != 3 {
			t.Errorf("Vector length: got %d, want 3", len(response.Vector))
		}
	})

	t.Run("given missing id, then returns NotFound", func(t *testing.T) {
		grpcTestSetup(t)
		name := "grpc_test_get_vector_missing_id"
		defer func() { _ = ffi.Drop(name) }()

		if err := ffi.Create(name, 3, ffi.MetricL2, ""); err != nil {
			t.Fatalf("setup create failed: %v", err)
		}

		service := &VectorService{}
		req := &nekov1.GetVectorRequest{Name: name, Id: "ghost"}

		_, err := service.Get(context.Background(), req)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if status.Code(err) != codes.NotFound {
			t.Errorf("got code %v, want %v", status.Code(err), codes.NotFound)
		}
	})
}

func TestVectorServiceUpsert(t *testing.T) {
	t.Run("given new id, then vector is created", func(t *testing.T) {
		grpcTestSetup(t)
		name := "grpc_test_upsert_new"
		defer func() { _ = ffi.Drop(name) }()

		if err := ffi.Create(name, 3, ffi.MetricL2, ""); err != nil {
			t.Fatalf("setup create failed: %v", err)
		}

		service := &VectorService{}
		req := &nekov1.UpsertVectorRequest{
			Name: name,
			Id:   "doc1",
			Body: &nekov1.UpsertVector{
				Vector: []float32{1.0, 2.0, 3.0},
			},
		}

		response, err := service.Upsert(context.Background(), req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if response.Id != "doc1" {
			t.Errorf("Id: got %q, want doc1", response.Id)
		}
		if response.Dim != 3 {
			t.Errorf("Dim: got %d, want 3", response.Dim)
		}
	})

	t.Run("given existing id, then vector is replaced", func(t *testing.T) {
		grpcTestSetup(t)
		name := "grpc_test_upsert_replace"
		defer func() { _ = ffi.Drop(name) }()

		if err := ffi.Create(name, 3, ffi.MetricL2, ""); err != nil {
			t.Fatalf("setup create failed: %v", err)
		}
		if err := ffi.Insert(name, "doc1", []float32{1.0, 0.0, 0.0}, ""); err != nil {
			t.Fatalf("setup insert failed: %v", err)
		}

		service := &VectorService{}
		req := &nekov1.UpsertVectorRequest{
			Name: name,
			Id:   "doc1",
			Body: &nekov1.UpsertVector{
				Vector: []float32{0.0, 1.0, 0.0},
			},
		}

		_, err := service.Upsert(context.Background(), req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		retrieved, _, getErr := ffi.GetVector(name, "doc1", 3)
		if getErr != nil {
			t.Fatalf("get after upsert failed: %v", getErr)
		}
		if retrieved[0] != 0.0 || retrieved[1] != 1.0 || retrieved[2] != 0.0 {
			t.Errorf("retrieved vector: got %v, want [0.0 1.0 0.0]", retrieved)
		}
	})
}

func TestVectorServiceDelete(t *testing.T) {
	t.Run("given existing vector, then it is removed", func(t *testing.T) {
		grpcTestSetup(t)
		name := "grpc_test_delete_basic"
		defer func() { _ = ffi.Drop(name) }()

		if err := ffi.Create(name, 3, ffi.MetricL2, ""); err != nil {
			t.Fatalf("setup create failed: %v", err)
		}
		if err := ffi.Insert(name, "doc1", []float32{1.0, 2.0, 3.0}, ""); err != nil {
			t.Fatalf("setup insert failed: %v", err)
		}

		service := &VectorService{}
		req := &nekov1.DeleteVectorRequest{Name: name, Id: "doc1"}

		_, err := service.Delete(context.Background(), req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		_, _, getErr := ffi.GetVector(name, "doc1", 3)
		if getErr == nil {
			t.Error("expected vector to be removed, but GetVector still succeeds")
		}
	})

	t.Run("given missing id, then returns NotFound", func(t *testing.T) {
		grpcTestSetup(t)
		name := "grpc_test_delete_missing_id"
		defer func() { _ = ffi.Drop(name) }()

		if err := ffi.Create(name, 3, ffi.MetricL2, ""); err != nil {
			t.Fatalf("setup create failed: %v", err)
		}

		service := &VectorService{}
		req := &nekov1.DeleteVectorRequest{Name: name, Id: "ghost"}

		_, err := service.Delete(context.Background(), req)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if status.Code(err) != codes.NotFound {
			t.Errorf("got code %v, want %v", status.Code(err), codes.NotFound)
		}
	})
}
