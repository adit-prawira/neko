package grpcserver

import (
	nekov1 "github.com/adit-prawira/neko/internal/gen/neko/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func NewGRPCServer() *grpc.Server {
	server := grpc.NewServer()
	reflection.Register(server)
	return server
}

func RegisterServices(server *grpc.Server) {
	nekov1.RegisterCollectionServiceServer(server, &CollectionService{})
	nekov1.RegisterVectorServiceServer(server, &VectorService{})
}
