package config

import "os"

const (
	defaultHTTPAddr    = ":8080"
	defaultDatabaseURL = "postgres://propulse:propulse@127.0.0.1:5432/propulse?sslmode=disable"
	defaultRedisAddr   = "127.0.0.1:6379"
	defaultLogLevel    = "info"
)

type Config struct {
	HTTPAddr     string
	DatabaseURL  string
	RedisAddr    string
	Mode         string
	SeedDemoData bool
	Log          LogConfig
}

type LogConfig struct {
	Level  string
	Pretty bool
}

func Load() (Config, error) {
	return Config{
		HTTPAddr:     getEnv("PROPULSE_HTTP_ADDR", defaultHTTPAddr),
		DatabaseURL:  getEnv("PROPULSE_DATABASE_URL", defaultDatabaseURL),
		RedisAddr:    getEnv("PROPULSE_REDIS_ADDR", defaultRedisAddr),
		SeedDemoData: getEnv("PROPULSE_SEED_DEMO_DATA", "") == "true",
		Log: LogConfig{
			Level:  getEnv("PROPULSE_LOG_LEVEL", defaultLogLevel),
			Pretty: getEnv("PROPULSE_LOG_PRETTY", "") == "true",
		},
	}, nil
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	return fallback
}
