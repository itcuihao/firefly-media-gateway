package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	ListenAddr       string
	DatabaseURL      string
	AuthToken        string
	PublicBaseURL    string
	ProviderDefault  string
	UploadTimeout    time.Duration
	TelegramBotToken string
	TelegramChatID   string
}

func Load() (Config, error) {
	cfg := Config{
		ListenAddr:       getenv("APP_LISTEN_ADDR", ":8080"),
		DatabaseURL:      strings.TrimSpace(os.Getenv("DATABASE_URL")),
		AuthToken:        strings.TrimSpace(os.Getenv("MEDIA_GATEWAY_TOKEN")),
		PublicBaseURL:    strings.TrimRight(getenv("PUBLIC_BASE_URL", "http://localhost:8080"), "/"),
		ProviderDefault:  strings.ToLower(getenv("MEDIA_PROVIDER_DEFAULT", "tg")),
		UploadTimeout:    durationFromEnv("UPLOAD_TIMEOUT_SECONDS", 60),
		TelegramBotToken: strings.TrimSpace(os.Getenv("TELEGRAM_BOT_TOKEN")),
		TelegramChatID:   strings.TrimSpace(os.Getenv("TELEGRAM_CHAT_ID")),
	}

	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}
	if cfg.AuthToken == "" {
		return Config{}, fmt.Errorf("MEDIA_GATEWAY_TOKEN is required")
	}
	if cfg.ProviderDefault != "tg" && cfg.ProviderDefault != "r2" {
		return Config{}, fmt.Errorf("MEDIA_PROVIDER_DEFAULT must be tg or r2")
	}
	if cfg.ProviderDefault == "tg" {
		if cfg.TelegramBotToken == "" || cfg.TelegramChatID == "" {
			return Config{}, fmt.Errorf("TELEGRAM_BOT_TOKEN and TELEGRAM_CHAT_ID are required when provider=tg")
		}
	}

	return cfg, nil
}

func getenv(key, defaultValue string) string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return defaultValue
	}
	return v
}

func durationFromEnv(key string, defaultSeconds int) time.Duration {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return time.Duration(defaultSeconds) * time.Second
	}
	sec, err := strconv.Atoi(v)
	if err != nil || sec <= 0 {
		return time.Duration(defaultSeconds) * time.Second
	}
	return time.Duration(sec) * time.Second
}
