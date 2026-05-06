package config

import (
	"errors"
	"os"
)

type Config struct {
	AppEnv           string
	HTTPAddr         string
	DatabaseURL      string
	MigrationsPath   string
	PublicBaseURL    string
	WebOrigin        string
	AuthTokenKey     string
	AdminEmail       string
	AdminPassword    string
	AlipayAppID      string
	AlipayGateway    string
	AlipayPrivateKey string
	AlipayPublicKey  string
	AlipayNotifyURL  string
	AlipayReturnURL  string
}

func Load() (Config, error) {
	cfg := Config{
		AppEnv:           getEnv("APP_ENV", "development"),
		HTTPAddr:         getEnv("HTTP_ADDR", ":8080"),
		DatabaseURL:      os.Getenv("DATABASE_URL"),
		MigrationsPath:   getEnv("MIGRATIONS_PATH", "migrations"),
		PublicBaseURL:    getEnv("PUBLIC_BASE_URL", "http://localhost:8080"),
		WebOrigin:        getEnv("WEB_ORIGIN", "http://localhost:5173"),
		AuthTokenKey:     getEnv("AUTH_TOKEN_KEY", "development-insecure-change-me"),
		AdminEmail:       os.Getenv("ADMIN_EMAIL"),
		AdminPassword:    os.Getenv("ADMIN_PASSWORD"),
		AlipayAppID:      os.Getenv("ALIPAY_APP_ID"),
		AlipayGateway:    getEnv("ALIPAY_GATEWAY_URL", "https://openapi-sandbox.dl.alipaydev.com/gateway.do"),
		AlipayPrivateKey: os.Getenv("ALIPAY_APP_PRIVATE_KEY"),
		AlipayPublicKey:  os.Getenv("ALIPAY_PUBLIC_KEY"),
		AlipayNotifyURL:  os.Getenv("ALIPAY_NOTIFY_URL"),
		AlipayReturnURL:  os.Getenv("ALIPAY_RETURN_URL"),
	}

	if cfg.DatabaseURL == "" {
		return Config{}, errors.New("DATABASE_URL is required")
	}
	if cfg.AppEnv == "production" && cfg.AuthTokenKey == "development-insecure-change-me" {
		return Config{}, errors.New("AUTH_TOKEN_KEY is required in production")
	}
	if (cfg.AdminEmail == "") != (cfg.AdminPassword == "") {
		return Config{}, errors.New("ADMIN_EMAIL and ADMIN_PASSWORD must be set together")
	}
	if cfg.AlipayAppID != "" && (cfg.AlipayPrivateKey == "" || cfg.AlipayPublicKey == "" || cfg.AlipayNotifyURL == "" || cfg.AlipayReturnURL == "") {
		return Config{}, errors.New("ALIPAY_APP_ID requires private key, public key, notify URL, and return URL")
	}

	return cfg, nil
}

func getEnv(key string, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
