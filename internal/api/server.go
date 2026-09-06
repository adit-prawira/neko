package api

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type Config struct {
	Port          int
	DataDirectory string
}

func Start(cfg Config) error {
	server := NewServer()
	if err := server.InitEngine(cfg.DataDirectory); err != nil {
		return fmt.Errorf("init engine: %w", err)
	}

	defer func() {
		if err := server.ShutDown(); err != nil {
			slog.Error("engine shutdown error", "error", err)
		}
	}()

	handler := buildRoutes(server)
	handler = CORS(RequestLogging(PanicRecovery(handler)))

	httpServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Port),
		Handler: handler,
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-quit
		slog.Info("received signal, shutting down", "signal", sig.String())
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(ctx); err != nil {
			slog.Error("http shutdown error", "error", err)
		}
	}()

	slog.Info("neko serve starting", "port", cfg.Port)
	if err := httpServer.ListenAndServe(); err != http.ErrServerClosed {
		return fmt.Errorf("http server: %w", err)
	}

	slog.Info("server stopped")
	return nil
}
