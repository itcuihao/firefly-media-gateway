package main

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"firefly-media-gateway/internal/config"
	"firefly-media-gateway/internal/httpapi"
	"firefly-media-gateway/internal/media"
	"firefly-media-gateway/internal/provider"
	"firefly-media-gateway/internal/s3"
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

	// 根据存储模式配置 provider
	switch cfg.StorageMode {
	case config.StorageModeProxy:
		// Proxy 模式：使用 Worker
		workerProvider := provider.NewWorkerProvider(cfg.WorkerBaseURL, cfg.WorkerAuthToken)
		providers[workerProvider.Name()] = workerProvider
		providers["tg"] = workerProvider // 兼容现有代码
	case config.StorageModeDirect:
		// Direct 模式：直接对接 Telegram
		if cfg.HasMultiBots() {
			// 多 bot 配置：使用第一个作为默认
			for botName, botCfg := range cfg.TelegramBotsConfig {
				tgProvider := provider.NewTelegramProviderWithConfig(botCfg.Token, botCfg.DefaultGroup, cfg.UploadTimeout)
				providers[botName] = tgProvider
				// 如果没有指定默认，使用第一个
				if providers["tg"] == nil {
					providers["tg"] = tgProvider
				}
			}
		} else {
			// 单 bot 配置
			tgProvider := provider.NewTelegramProvider(cfg.TelegramBotToken, cfg.TelegramChatID, cfg.UploadTimeout)
			providers[tgProvider.Name()] = tgProvider
		}

		// R2 provider（如果启用）
		if cfg.ProviderDefault == "r2" {
			r2Provider := provider.NewR2Provider()
			providers[r2Provider.Name()] = r2Provider
		}
	}

		svc := media.NewService(repo, providers, cfg.ProviderDefault, cfg.PublicBaseURL)
		h := httpapi.NewServer(svc, cfg.AuthToken, cfg.TelegramBotToken, logger)

		// 创建 S3 Gateway（可选）
		s3Gateway := s3.NewGateway(svc, cfg.PublicBaseURL)

		// 组合处理器
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/s3/") {
				s3Gateway.Handler().ServeHTTP(w, r)
			} else {
				h.Handler().ServeHTTP(w, r)
			}
		})

		srv := &http.Server{
			Addr:              cfg.ListenAddr,
			Handler:           handler,
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
