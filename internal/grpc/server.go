package grpcserver

import (
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func NewGRPCServer() *grpc.Server {
	server := grpc.NewServer()
	reflection.Register(server)
	return server
}
