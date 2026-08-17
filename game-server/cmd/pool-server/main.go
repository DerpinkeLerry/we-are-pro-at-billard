package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"poolarena/game-server/internal/app"
	"poolarena/game-server/internal/auth"
	"poolarena/game-server/internal/config"
	"poolarena/game-server/internal/lobby"
	"poolarena/game-server/internal/persistence"
	"syscall"
	"time"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)
	cfg, err := config.Load()
	if err != nil {
		slog.Error("load config", "error", err)
		os.Exit(1)
	}
	secret := os.Getenv("JOIN_TOKEN_SECRET")
	if len(secret) < 32 {
		slog.Error("JOIN_TOKEN_SECRET must be at least 32 bytes")
		os.Exit(1)
	}
	internalSecret := os.Getenv("GAME_INTERNAL_SECRET")
	if len(internalSecret) < 32 {
		slog.Error("GAME_INTERNAL_SECRET must be at least 32 bytes")
		os.Exit(1)
	}
	var store persistence.Store = persistence.Memory{}
	if base := os.Getenv("PERSISTENCE_URL"); base != "" {
		store = persistence.OpenHTTP(base, internalSecret)
	}
	defer store.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	if pingErr := store.Ping(ctx); pingErr != nil {
		slog.Warn("persistence starts degraded", "error", pingErr)
	}
	cancel()
	origins := os.Getenv("ALLOWED_ORIGINS")
	if origins == "" {
		origins = "http://localhost:8080"
	}
	manager := lobby.NewManager(cfg, store)
	server := app.NewServer(manager, auth.NewValidator(secret), store, internalSecret, origins)
	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}
	httpServer := &http.Server{Addr: ":" + port, Handler: server.Routes(), ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 60 * time.Second}
	go func() {
		slog.Info("game server listening", "port", port)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("http server", "error", err)
			os.Exit(1)
		}
	}()
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT)
	<-sig
	slog.Info("shutdown requested")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 50*time.Second)
	defer shutdownCancel()
	_ = httpServer.Shutdown(shutdownCtx)
	manager.Shutdown(shutdownCtx)
	slog.Info("shutdown complete")
}
