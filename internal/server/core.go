package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/adit-prawira/neko/internal/api"
	grpcserver "github.com/adit-prawira/neko/internal/grpc"
	"github.com/soheilhy/cmux"
	"google.golang.org/grpc"
)

type Config struct {
	Port          int
	DataDirectory string
}

func Start(cfg Config) error {
	apiServer := api.NewServer()
	if err := apiServer.InitEngine(cfg.DataDirectory); err != nil {
		return fmt.Errorf("init engine: %w", err)
	}

	defer func() {
		if err := apiServer.ShutDown(); err != nil {
			slog.Error("engine shutdown error", "error", err)
		}
	}()

	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.Port))
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}

	mux := cmux.New(listener)
	grpcListener := mux.Match(cmux.HTTP2())
	httpListener := mux.Match(cmux.HTTP1Fast())
	anyListener := mux.Match(cmux.Any())

	httpServer := api.NewHttpServer(apiServer)
	grpcServer := grpcserver.NewGRPCServer()
	grpcserver.RegisterServices(grpcServer)

	errChannel := make(chan error, 4)
	go func() { errChannel <- httpServer.Serve(httpListener) }()
	go func() { errChannel <- grpcServer.Serve(grpcListener) }()
	go func() { errChannel <- closeServes(anyListener) }()
	go func() { errChannel <- mux.Serve() }()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	slog.Info("neko server starting", "port", cfg.Port, "protocol", []string{"rest", "grpc"})

	select {
	case sig := <-quit:
		slog.Info("received signal, shutting down", "signal", sig.String())
	case err := <-errChannel:
		if err != nil && !errors.Is(err, http.ErrServerClosed) && !errors.Is(err, grpc.ErrServerStopped) {
			slog.Error("server error", "error", err)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	shutdownDone := make(chan struct{})
	go func() {
		defer close(shutdownDone)
		httpServer.Shutdown(ctx)
		grpcServer.Stop()
		_ = listener.Close()
	}()

	select {
	case <-shutdownDone:
		slog.Info("server stopped")
		return nil
	case <-ctx.Done():
		slog.Warn("shutdown timeout exceeded, forcing close")
		httpServer.Close()
		grpcServer.Stop()
		return nil
	}
}

func closeServes(ln net.Listener) error {
	for {
		connection, err := ln.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return nil
			}
			return err
		}
		_ = connection.Close()
	}
}
