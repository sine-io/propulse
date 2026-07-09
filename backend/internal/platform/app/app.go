package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/propulse/propulse/backend/internal/infrastructure/config"
	"github.com/propulse/propulse/backend/internal/interfaces/http/router"
	"github.com/propulse/propulse/backend/web"
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
	case "serve", "api":
		return runHTTPServer(ctx, cfg, log)
	case "worker", "scheduler", "migrate up", "migrate down":
		<-ctx.Done()
		return nil
	default:
		return errors.New(Usage)
	}
}

func runHTTPServer(ctx context.Context, cfg config.Config, log zerolog.Logger) error {
	engine := router.New(router.Dependencies{
		Log:      log,
		StaticFS: web.Embedded(),
	})

	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           engine,
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	go func() {
		errCh <- server.ListenAndServe()
	}()

	err := <-errCh
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}

	return err
}
