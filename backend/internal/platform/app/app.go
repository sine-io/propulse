package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/propulse/propulse/backend/internal/infrastructure/config"
	"github.com/rs/zerolog"
)

const Usage = "usage: propulse [serve|api|worker|scheduler|migrate up|migrate down]"

func NormalizeMode(args []string) (string, error) {
	if len(args) == 0 {
		return "", errors.New(Usage)
	}

	switch args[0] {
	case "serve", "api", "worker", "scheduler":
		if len(args) != 1 {
			return "", errors.New(Usage)
		}
		return args[0], nil
	case "migrate":
		if len(args) != 2 {
			return "", errors.New(Usage)
		}
		switch args[1] {
		case "up", "down":
			return fmt.Sprintf("migrate %s", args[1]), nil
		}
	}

	return "", errors.New(Usage)
}

func Run(ctx context.Context, mode string, cfg config.Config, log zerolog.Logger) error {
	cfg.Mode = mode
	log.Info().
		Str("mode", mode).
		Str("http_addr", cfg.HTTPAddr).
		Str("redis_addr", cfg.RedisAddr).
		Msg("starting propulse backend")

	switch mode {
	case "serve", "api", "worker", "scheduler", "migrate up", "migrate down":
		<-ctx.Done()
		return nil
	default:
		return errors.New(Usage)
	}
}
