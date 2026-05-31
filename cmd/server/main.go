package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"

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

	db, repo, err := openRepository(context.Background(), cfg)
	if err != nil {
		logger.Fatalf("open repository: %v", err)
	}
	defer db.Close()
	providers, err := buildProviders(cfg)
	if err != nil {
		logger.Fatalf("build providers: %v", err)
	}

	svc := media.NewService(repo, providers, cfg.ProviderDefault, cfg.PublicBaseURL)
	h := httpapi.NewServer(svc, cfg.AuthToken, cfg.TelegramBotToken, cfg.WorkerBaseURL, cfg.WorkerAuthToken, cfg.PublicBaseURL, cfg.PrivateRules, cfg.DatabaseDriver, string(cfg.StorageMode), logger)

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

func openRepository(ctx context.Context, cfg config.Config) (*sql.DB, media.Repository, error) {
	switch cfg.DatabaseDriver {
	case "postgres":
		db, err := sql.Open("pgx", cfg.DatabaseURL)
		if err != nil {
			return nil, nil, fmt.Errorf("open postgres: %w", err)
		}
		if err := storage.Ping(ctx, db); err != nil {
			db.Close()
			return nil, nil, fmt.Errorf("postgres not ready: %w", err)
		}
		return db, storage.NewPostgresRepository(db), nil
	case "sqlite":
		if err := ensureSQLiteDir(cfg.DatabaseURL); err != nil {
			return nil, nil, err
		}
		db, err := sql.Open("sqlite", cfg.DatabaseURL)
		if err != nil {
			return nil, nil, fmt.Errorf("open sqlite: %w", err)
		}
		db.SetMaxOpenConns(1)
		if err := storage.Ping(ctx, db); err != nil {
			db.Close()
			return nil, nil, fmt.Errorf("sqlite not ready: %w", err)
		}
		if err := storage.EnsureSQLiteSchema(ctx, db); err != nil {
			db.Close()
			return nil, nil, err
		}
		return db, storage.NewSQLiteRepository(db), nil
	default:
		return nil, nil, fmt.Errorf("unsupported database driver %q", cfg.DatabaseDriver)
	}
}

func ensureSQLiteDir(databaseURL string) error {
	if databaseURL == "" || strings.HasPrefix(databaseURL, "file:") || databaseURL == ":memory:" {
		return nil
	}
	dir := filepath.Dir(databaseURL)
	if dir == "." || dir == "" {
		return nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create sqlite directory: %w", err)
	}
	return nil
}

func buildProviders(cfg config.Config) (map[string]provider.StorageProvider, error) {
	providers := map[string]provider.StorageProvider{}

	switch cfg.StorageMode {
	case config.StorageModeProxy:
		workerProvider := provider.NewWorkerProvider(cfg.WorkerBaseURL, cfg.WorkerAuthToken)
		providers["tg"] = workerProvider // 兼容现有配置
		providers["worker"] = workerProvider
	case config.StorageModeDirect:
		if cfg.HasMultiBots() {
			for botName, botCfg := range cfg.TelegramBotsConfig {
				tgProvider := provider.NewTelegramProviderWithConfig(botCfg.Token, botCfg.DefaultGroup, cfg.UploadTimeout)
				providers[botName] = tgProvider
				if providers["tg"] == nil {
					providers["tg"] = tgProvider
				}
			}
		} else if cfg.TelegramBotToken != "" && cfg.TelegramChatID != "" {
			tgProvider := provider.NewTelegramProvider(cfg.TelegramBotToken, cfg.TelegramChatID, cfg.UploadTimeout)
			providers[tgProvider.Name()] = tgProvider
		}

		if cfg.ProviderDefault == "r2" {
			r2Provider := provider.NewR2Provider()
			providers[r2Provider.Name()] = r2Provider
		}
	default:
		return nil, fmt.Errorf("unsupported storage mode %q", cfg.StorageMode)
	}

	if providers[cfg.ProviderDefault] == nil {
		return nil, fmt.Errorf("provider %q is not registered", cfg.ProviderDefault)
	}
	return providers, nil
}
