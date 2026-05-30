package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// StorageMode 定义存储模式
type StorageMode string

const (
	StorageModeDirect StorageMode = "direct" // 直接对接 Telegram
	StorageModeProxy  StorageMode = "proxy"  // 通过 Worker 代理
)

// BotConfig 单个 Bot 配置
type BotConfig struct {
	Token        string `json:"token"`
	DefaultGroup string `json:"default_group,omitempty"`
}

// BotsConfig 多 Bot 配置
type BotsConfig map[string]BotConfig

type Config struct {
	// 服务配置
	ListenAddr     string
	DatabaseDriver string
	DatabaseURL    string
	AuthToken      string
	PrivateRules   []string
	PublicBaseURL  string

	// 存储模式配置
	StorageMode     StorageMode
	ProviderDefault string // provider key, for example "tg", "r2", "discord", "huggingface"

	// Direct 模式配置（直接对接 Telegram）
	TelegramBotToken   string
	TelegramChatID     string
	TelegramBotsConfig BotsConfig // 多 bot配置（JSON 格式）
	UploadTimeout      time.Duration

	// Proxy 模式配置（通过 Worker）
	WorkerBaseURL   string // Worker 服务 URL
	WorkerAuthToken string // Worker 鉴权 token
}

func Load() (Config, error) {
	cfg := Config{
		ListenAddr:       getenv("APP_LISTEN_ADDR", ":8080"),
		DatabaseURL:      strings.TrimSpace(os.Getenv("DATABASE_URL")),
		AuthToken:        strings.TrimSpace(os.Getenv("MEDIA_GATEWAY_TOKEN")),
		PublicBaseURL:    strings.TrimRight(getenv("PUBLIC_BASE_URL", "http://localhost:8080"), "/"),
		StorageMode:      StorageMode(strings.ToLower(getenv("STORAGE_MODE", "direct"))),
		ProviderDefault:  strings.ToLower(getenv("MEDIA_PROVIDER_DEFAULT", "tg")),
		UploadTimeout:    durationFromEnv("UPLOAD_TIMEOUT_SECONDS", 60),
		TelegramBotToken: strings.TrimSpace(os.Getenv("TELEGRAM_BOT_TOKEN")),
		TelegramChatID:   strings.TrimSpace(os.Getenv("TELEGRAM_CHAT_ID")),
		WorkerBaseURL:    strings.TrimSpace(os.Getenv("WORKER_BASE_URL")),
		WorkerAuthToken:  strings.TrimSpace(os.Getenv("WORKER_AUTH_TOKEN")),
	}
	cfg.DatabaseDriver = resolveDatabaseDriver(strings.TrimSpace(os.Getenv("DATABASE_DRIVER")), cfg.DatabaseURL)
	if cfg.DatabaseURL == "" {
		cfg.DatabaseURL = "data/media_gateway.db"
	}
	cfg.PrivateRules = parseStringSlice(os.Getenv("PRIVATE_RULES"))

	// 解析多 bot 配置
	if botsConfigStr := strings.TrimSpace(os.Getenv("TELEGRAM_BOTS_CONFIG")); botsConfigStr != "" {
		var botsConfig BotsConfig
		if err := json.Unmarshal([]byte(botsConfigStr), &botsConfig); err != nil {
			return Config{}, fmt.Errorf("invalid TELEGRAM_BOTS_CONFIG: %w", err)
		}
		cfg.TelegramBotsConfig = botsConfig
	}

	// 验证必填配置
	if cfg.DatabaseDriver != "postgres" && cfg.DatabaseDriver != "sqlite" {
		return Config{}, fmt.Errorf("DATABASE_DRIVER must be postgres or sqlite")
	}
	if cfg.AuthToken == "" {
		return Config{}, fmt.Errorf("MEDIA_GATEWAY_TOKEN is required")
	}
	if cfg.ProviderDefault == "" {
		return Config{}, fmt.Errorf("MEDIA_PROVIDER_DEFAULT is required")
	}

	// 验证存储模式配置
	switch cfg.StorageMode {
	case StorageModeDirect:
		// Direct 模式：需要 Telegram 配置
		if cfg.ProviderDefault == "tg" {
			// 优先使用多 bot 配置
			if len(cfg.TelegramBotsConfig) > 0 {
				// 多 bot 配置存在，无需单独的 TELEGRAM_BOT_TOKEN
				break
			}
			// 兼容单 bot 配置
			if cfg.TelegramBotToken == "" || cfg.TelegramChatID == "" {
				return Config{}, fmt.Errorf("TELEGRAM_BOT_TOKEN and TELEGRAM_CHAT_ID are required when mode=direct and provider=tg")
			}
		}
	case StorageModeProxy:
		// Proxy 模式：需要 Worker 配置
		if cfg.WorkerBaseURL == "" {
			return Config{}, fmt.Errorf("WORKER_BASE_URL is required when mode=proxy")
		}
		if cfg.WorkerAuthToken == "" {
			return Config{}, fmt.Errorf("WORKER_AUTH_TOKEN is required when mode=proxy")
		}
	default:
		return Config{}, fmt.Errorf("invalid STORAGE_MODE: %s (must be 'direct' or 'proxy')", cfg.StorageMode)
	}

	return cfg, nil
}

func resolveDatabaseDriver(driver, databaseURL string) string {
	driver = strings.ToLower(strings.TrimSpace(driver))
	if driver != "" {
		return driver
	}

	if databaseURL == "" {
		return "sqlite"
	}

	parts := strings.SplitN(databaseURL, ":", 2)
	if len(parts) > 0 {
		scheme := strings.ToLower(parts[0])
		if scheme == "postgresql" {
			return "postgres"
		}
		return scheme
	}
	return "sqlite"
}

// IsProxyMode 判断是否为代理模式
func (c *Config) IsProxyMode() bool {
	return c.StorageMode == StorageModeProxy
}

// HasMultiBots 判断是否配置了多个 bot
func (c *Config) HasMultiBots() bool {
	return len(c.TelegramBotsConfig) > 0
}

// GetBotConfig 获取指定 bot 的配置
func (c *Config) GetBotConfig(botName string) (BotConfig, bool) {
	if c.TelegramBotsConfig == nil {
		return BotConfig{}, false
	}
	cfg, ok := c.TelegramBotsConfig[botName]
	return cfg, ok
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

func getBoolEnv(key string, defaultValue bool) bool {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return defaultValue
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return defaultValue
	}
	return b
}

func parseStringSlice(v string) []string {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil
	}
	var res []string
	for _, s := range strings.Split(v, ",") {
		s = strings.TrimSpace(s)
		if s != "" {
			res = append(res, s)
		}
	}
	return res
}
