package main

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"firefly-media-gateway/internal/config"
	"firefly-media-gateway/internal/httpapi"
	"firefly-media-gateway/internal/media"
	"firefly-media-gateway/internal/provider"
	"firefly-media-gateway/internal/storage"
)

func main() {
	logger := log.New(os.Stdout, "[media-gateway] ", log.LstdFlags|log.Lmicroseconds)

	cfg, err := config.Load()
	if err != nil {
		logger.Fatalf("load config: %v", err)
	}

	db, err := sql.Open("pgx", cfg.DatabaseURL)
	if err != nil {
		logger.Fatalf("open postgres: %v", err)
	}
	defer db.Close()

	if err := storage.Ping(context.Background(), db); err != nil {
		logger.Fatalf("postgres not ready: %v", err)
	}

	repo := storage.NewPostgresRepository(db)
	providers := map[string]provider.StorageProvider{}

	tgProvider := provider.NewTelegramProvider(cfg.TelegramBotToken, cfg.TelegramChatID, cfg.UploadTimeout)
	providers[tgProvider.Name()] = tgProvider

	r2Provider := provider.NewR2Provider()
	providers[r2Provider.Name()] = r2Provider

	svc := media.NewService(repo, providers, cfg.ProviderDefault, cfg.PublicBaseURL)
	h := httpapi.NewServer(svc, cfg.AuthToken, logger)

	srv := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           h.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      120 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	go func() {
		logger.Printf("server listening on %s", cfg.ListenAddr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Fatalf("listen and serve: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Printf("graceful shutdown failed: %v", err)
	}
}
