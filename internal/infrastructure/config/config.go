package config

import (
	"os"
	"time"
)

const (
	defaultHTTPAddr          = ":8080"
	defaultDatabaseURL       = "postgres://propulse:propulse@127.0.0.1:5432/propulse?sslmode=disable"
	defaultRedisAddr         = "127.0.0.1:6379"
	defaultLogLevel          = "info"
	defaultSchedulerInterval = time.Hour
)

type Config struct {
	HTTPAddr          string
	DatabaseURL       string
	RedisAddr         string
	AccessToken       string
	Mode              string
	SchedulerInterval time.Duration
	Log               LogConfig
}

type LogConfig struct {
	Level  string
	Pretty bool
}

func Load() (Config, error) {
	schedulerInterval, err := parseDurationEnv("PROPULSE_SCHEDULER_INTERVAL", defaultSchedulerInterval)
	if err != nil {
		return Config{}, err
	}

	return Config{
		HTTPAddr:          getEnv("PROPULSE_HTTP_ADDR", defaultHTTPAddr),
		DatabaseURL:       getEnv("PROPULSE_DATABASE_URL", defaultDatabaseURL),
		RedisAddr:         getEnv("PROPULSE_REDIS_ADDR", defaultRedisAddr),
		AccessToken:       getEnv("PROPULSE_ACCESS_TOKEN", ""),
		SchedulerInterval: schedulerInterval,
		Log: LogConfig{
			Level:  getEnv("PROPULSE_LOG_LEVEL", defaultLogLevel),
			Pretty: getEnv("PROPULSE_LOG_PRETTY", "") == "true",
		},
	}, nil
}

func parseDurationEnv(key string, fallback time.Duration) (time.Duration, error) {
	value := getEnv(key, "")
	if value == "" {
		return fallback, nil
	}

	return time.ParseDuration(value)
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	return fallback
}
