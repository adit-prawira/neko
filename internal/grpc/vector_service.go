package grpcserver

import (
	"context"
	"fmt"

	"github.com/adit-prawira/neko/internal/ffi"
	nekov1 "github.com/adit-prawira/neko/internal/gen/neko/v1"
	"google.golang.org/protobuf/types/known/emptypb"
)

type VectorService struct {
	nekov1.UnimplementedVectorServiceServer
}

func (vs *VectorService) Insert(_ context.Context, req *nekov1.InsertVectorRequest) (*nekov1.UpsertVectorResponse, error) {
	if req.Body.Id == "" {
		return nil, StatusError(CodeInvalidArgument, "id is required")
	}

	if err := ffi.Insert(req.Name, req.Body.Id, req.Body.Vector, req.Body.Metadata); err != nil {
		return nil, HairballToGRPCStatus(err)
	}

	return &nekov1.UpsertVectorResponse{
		Id:  req.Body.Id,
		Dim: uint32(len(req.Body.Vector)),
	}, nil
}

const maxBatchInsertSize = 10000

func (vs *VectorService) InsertMany(_ context.Context, req *nekov1.InsertManyVectorRequest) (*nekov1.InsertManyVectorResponse, error) {
	if len(req.Vectors) == 0 {
		return nil, StatusError(CodeInvalidArgument, "vectors are required")
	}

	if len(req.Vectors) > maxBatchInsertSize {
		message := fmt.Sprintf("batch size %d exceeds maximum of %d vectors", len(req.Vectors), maxBatchInsertSize)
		return nil, StatusError(CodeInvalidArgument, message)
	}

	stats, err := ffi.Stats(req.Name)
	if err != nil {
		return nil, HairballToGRPCStatus(err)
	}

	inputVectors := make([]ffi.InputVector, 0, len(req.Vectors))
	for index, input := range req.Vectors {
		if input.Id == "" {
			message := fmt.Sprintf("vector[%d].id is required", index)
			return nil, StatusError(CodeInvalidArgument, message)
		}

		dim := len(input.Vector)
		if dim != int(stats.Dim) {
			message := fmt.Sprintf("vectors[%d] has dim %d, expected %d", index, dim, stats.Dim)
			return nil, StatusError(CodeInvalidArgument, message)
		}

		inputVectors = append(inputVectors, ffi.InputVector{
			ID:       input.Id,
			Vector:   input.Vector,
			Metadata: input.Metadata,
		})
	}

	if err := ffi.InsertMany(req.Name, inputVectors); err != nil {
		return nil, HairballToGRPCStatus(err)
	}

	insertedIDs := make([]string, len(req.Vectors))
	for index, input := range req.Vectors {
		insertedIDs[index] = input.Id
	}

	return &nekov1.InsertManyVectorResponse{
		Ids:      insertedIDs,
		Inserted: int32(len(insertedIDs)),
		Dim:      stats.Dim,
	}, nil
}

func (vs *VectorService) Get(_ context.Context, req *nekov1.GetVectorRequest) (*nekov1.GetVectorResponse, error) {
	stats, err := ffi.Stats(req.Name)
	if err != nil {
		return nil, HairballToGRPCStatus(err)
	}

	vector, metadata, err := ffi.GetVector(req.Name, req.Id, stats.Dim)
	if err != nil {
		return nil, HairballToGRPCStatus(err)
	}

	return &nekov1.GetVectorResponse{
		Id:       req.Id,
		Vector:   vector,
		Metadata: metadata,
	}, nil
}

func (vs *VectorService) Upsert(_ context.Context, req *nekov1.UpsertVectorRequest) (*nekov1.UpsertVectorResponse, error) {
	_, err := ffi.Upsert(req.Name, req.Id, req.Body.Vector, req.Body.Metadata)
	if err != nil {
		return nil, HairballToGRPCStatus(err)
	}

	return &nekov1.UpsertVectorResponse{
		Id:  req.Id,
		Dim: uint32(len(req.Body.Vector)),
	}, nil
}

func (vs *VectorService) Delete(_ context.Context, req *nekov1.DeleteVectorRequest) (*emptypb.Empty, error) {
	if err := ffi.Delete(req.Name, req.Id); err != nil {
		return nil, HairballToGRPCStatus(err)
	}

	return &emptypb.Empty{}, nil
}
