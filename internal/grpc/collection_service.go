package grpcserver

import (
	"context"

	"github.com/adit-prawira/neko/internal/ffi"
	nekov1 "github.com/adit-prawira/neko/internal/gen/neko/v1"
	"google.golang.org/protobuf/types/known/emptypb"
)

type CollectionService struct {
	nekov1.UnimplementedCollectionServiceServer
}

const defatulGRPCMetric = nekov1.Metric_METRIC_COSINE

func (cs *CollectionService) Create(_ context.Context, req *nekov1.CreateCollectionRequest) (*nekov1.CreateCollectionResponse, error) {
	if req.Name == "" {
		return nil, StatusError(CodeInvalidArgument, "name is required")
	}

	protoMetric := req.Metric
	if protoMetric == nekov1.Metric_METRIC_UNSPECIFIED {
		protoMetric = defatulGRPCMetric
	}

	metricCode, err := protoMetricToFFI(protoMetric)
	if err != nil {
		return nil, StatusError(CodeInvalidArgument, err.Error())
	}

	if err := ffi.Create(req.Name, req.Dim, metricCode, ""); err != nil {
		return nil, HairballToGRPCStatus(err)
	}

	return &nekov1.CreateCollectionResponse{
		Name:   req.Name,
		Dim:    req.Dim,
		Metric: ffiMetricToProto(metricCode),
	}, nil
}

func (cs *CollectionService) List(_ context.Context, _ *emptypb.Empty) (*nekov1.CollectionsResponse, error) {
	names, err := ffi.List()
	if err != nil {
		return nil, HairballToGRPCStatus(err)
	}

	collections := make([]*nekov1.CollectionResponse, 0, len(names))
	for _, name := range names {
		stats, err := ffi.Stats(name)
		if err != nil {
			continue
		}

		collections = append(collections, &nekov1.CollectionResponse{
			Name:         name,
			Dim:          stats.Dim,
			Metric:       ffiMetricToProto(stats.Metric),
			VectorCount:  stats.VectorCount,
			StorageBytes: stats.StorageBytes,
		})
	}

	return &nekov1.CollectionsResponse{
		Collections: collections,
	}, nil
}

func (cs *CollectionService) Get(_ context.Context, req *nekov1.GetCollectionRequest) (*nekov1.CollectionResponse, error) {
	stats, err := ffi.Stats(req.Name)
	if err != nil {
		return nil, HairballToGRPCStatus(err)
	}

	return &nekov1.CollectionResponse{
		Name:         req.Name,
		Dim:          stats.Dim,
		Metric:       ffiMetricToProto(stats.Metric),
		VectorCount:  stats.VectorCount,
		StorageBytes: stats.StorageBytes,
	}, nil
}

func (cs *CollectionService) Drop(_ context.Context, req *nekov1.DropCollectionRequest) (*emptypb.Empty, error) {
	if err := ffi.Drop(req.Name); err != nil {
		return nil, HairballToGRPCStatus(err)
	}

	return &emptypb.Empty{}, nil
}

const defaultTopK uint32 = 10

func (cs *CollectionService) Search(_ context.Context, req *nekov1.SearchRequest) (*nekov1.SearchResponse, error) {
	topK := defaultTopK
	if req.Query.TopK != nil {
		topK = *req.Query.TopK
	}

	results, err := ffi.Search(req.Name, req.Query.Vector, topK)
	if err != nil {
		return nil, HairballToGRPCStatus(err)
	}

	scoredResults := make([]*nekov1.ScoredResult, 0, len(results))
	for _, result := range results {
		scoredResults = append(scoredResults, &nekov1.ScoredResult{
			Id:    result.ID,
			Score: result.Score,
		})
	}

	return &nekov1.SearchResponse{
		Results: scoredResults,
	}, nil
}
