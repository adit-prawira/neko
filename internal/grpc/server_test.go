package grpcserver

import (
	"context"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	reflectionpb "google.golang.org/grpc/reflection/grpc_reflection_v1"
)

func TestNewGRPCServer(t *testing.T) {
	t.Run("given a reflection request, then it lists at least one service", func(t *testing.T) {
		server := NewGRPCServer()

		listener, listenError := net.Listen("tcp", "127.0.0.1:0")
		if listenError != nil {
			t.Fatalf("listen: %v", listenError)
		}
		defer listener.Close()

		serveError := make(chan error, 1)
		go func() {
			serveError <- server.Serve(listener)
		}()
		defer server.Stop()

		connection, dialError := grpc.NewClient(
			listener.Addr().String(),
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		)
		if dialError != nil {
			t.Fatalf("dial: %v", dialError)
		}
		defer connection.Close()

		client := reflectionpb.NewServerReflectionClient(connection)

		requestContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		stream, streamError := client.ServerReflectionInfo(requestContext)
		if streamError != nil {
			t.Fatalf("open reflection stream: %v", streamError)
		}

		if sendError := stream.Send(&reflectionpb.ServerReflectionRequest{
			MessageRequest: &reflectionpb.ServerReflectionRequest_ListServices{
				ListServices: "",
			},
		}); sendError != nil {
			t.Fatalf("send list services request: %v", sendError)
		}

		response, receiveError := stream.Recv()
		if receiveError != nil {
			t.Fatalf("recv list services response: %v", receiveError)
		}

		services := response.GetListServicesResponse().GetService()
		if len(services) == 0 {
			t.Fatal("expected reflection to list at least one service")
		}
	})
}

func TestRegisterServices(t *testing.T) {
	t.Run("given RegisterServices, then reflection lists neko.v1.CollectionService and neko.v1.VectorService", func(t *testing.T) {
		server := NewGRPCServer()
		RegisterServices(server)

		listener, listenError := net.Listen("tcp", "127.0.0.1:0")
		if listenError != nil {
			t.Fatalf("listen: %v", listenError)
		}
		defer listener.Close()

		serveError := make(chan error, 1)
		go func() {
			serveError <- server.Serve(listener)
		}()
		defer server.Stop()

		connection, dialError := grpc.NewClient(
			listener.Addr().String(),
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		)
		if dialError != nil {
			t.Fatalf("dial: %v", dialError)
		}
		defer connection.Close()

		client := reflectionpb.NewServerReflectionClient(connection)

		requestContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		stream, streamError := client.ServerReflectionInfo(requestContext)
		if streamError != nil {
			t.Fatalf("open reflection stream: %v", streamError)
		}

		if sendError := stream.Send(&reflectionpb.ServerReflectionRequest{
			MessageRequest: &reflectionpb.ServerReflectionRequest_ListServices{
				ListServices: "",
			},
		}); sendError != nil {
			t.Fatalf("send list services request: %v", sendError)
		}

		response, receiveError := stream.Recv()
		if receiveError != nil {
			t.Fatalf("recv list services response: %v", receiveError)
		}

		listed := make(map[string]bool)
		for _, svc := range response.GetListServicesResponse().GetService() {
			listed[svc.GetName()] = true
		}

		if !listed["neko.v1.CollectionService"] {
			t.Error("expected neko.v1.CollectionService to be registered, but it was not found in reflection")
		}
		if !listed["neko.v1.VectorService"] {
			t.Error("expected neko.v1.VectorService to be registered, but it was not found in reflection")
		}
	})
}
