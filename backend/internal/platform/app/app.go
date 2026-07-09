package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	appcapacity "github.com/propulse/propulse/backend/internal/application/capacity"
	appcollection "github.com/propulse/propulse/backend/internal/application/collection"
	appneighborhood "github.com/propulse/propulse/backend/internal/application/neighborhood"
	"github.com/propulse/propulse/backend/internal/infrastructure/config"
	migraterunner "github.com/propulse/propulse/backend/internal/infrastructure/migrate"
	postgresgorm "github.com/propulse/propulse/backend/internal/infrastructure/postgres/gorm"
	"github.com/propulse/propulse/backend/internal/interfaces/http/router"
	"github.com/propulse/propulse/backend/web"
	"github.com/rs/zerolog"
)

const Usage = "usage: propulse [serve|api|worker|scheduler|migrate up|migrate down]"

type CapacityApplication interface {
	CreateCalculation(ctx context.Context, command appcapacity.CreateCalculationCommand) (appcapacity.CalculationRecord, error)
	GetCalculation(ctx context.Context, query appcapacity.GetCalculationQuery) (appcapacity.CalculationRecord, error)
}

type NeighborhoodApplication interface {
	CreateNeighborhood(ctx context.Context, command appneighborhood.CreateNeighborhoodCommand) (appneighborhood.Neighborhood, error)
	GetNeighborhood(ctx context.Context, query appneighborhood.GetNeighborhoodQuery) (appneighborhood.Neighborhood, error)
	LatestMetric(ctx context.Context, query appneighborhood.LatestMetricQuery) (appneighborhood.MetricWithSignal, error)
	AddWatchlistItem(ctx context.Context, command appneighborhood.AddWatchlistItemCommand) (appneighborhood.WatchlistItem, error)
	ListWatchlist(ctx context.Context, query appneighborhood.ListWatchlistQuery) ([]appneighborhood.WatchlistItemSummary, error)
}

type CollectionApplication interface {
	ImportManualListings(ctx context.Context, command appcollection.ImportManualListingsCommand) (appcollection.ImportManualListingsResult, error)
}

var runMigrations = migraterunner.Run
var openCapacityApplication = func(ctx context.Context, cfg config.Config, log zerolog.Logger) (CapacityApplication, io.Closer, error) {
	db, sqlDB, err := postgresgorm.Open(cfg.DatabaseURL)
	if err != nil {
		return nil, nil, err
	}

	repo := postgresgorm.NewCapacityRepository(db)
	service := appcapacity.NewService(repo, time.Now, nil)
	return service, sqlDB, nil
}

var openNeighborhoodApplication = func(ctx context.Context, cfg config.Config, log zerolog.Logger) (NeighborhoodApplication, io.Closer, error) {
	db, sqlDB, err := postgresgorm.Open(cfg.DatabaseURL)
	if err != nil {
		return nil, nil, err
	}

	repo := postgresgorm.NewNeighborhoodRepository(db)
	if cfg.SeedDemoData {
		if err := repo.SeedDemoData(ctx); err != nil {
			_ = sqlDB.Close()
			return nil, nil, err
		}
		log.Info().Msg("seeded demo neighborhood data")
	}

	service := appneighborhood.NewService(repo)
	return service, sqlDB, nil
}

var openCollectionApplication = func(ctx context.Context, cfg config.Config, log zerolog.Logger) (CollectionApplication, io.Closer, error) {
	db, sqlDB, err := postgresgorm.Open(cfg.DatabaseURL)
	if err != nil {
		return nil, nil, err
	}

	repo := postgresgorm.NewCollectionRepository(db)
	service := appcollection.NewService(repo, time.Now, nil)
	return service, sqlDB, nil
}

var listenAndServe = func(server *http.Server) error {
	return server.ListenAndServe()
}

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
	case "migrate up":
		return runMigrations(ctx, cfg.DatabaseURL, "up")
	case "migrate down":
		return runMigrations(ctx, cfg.DatabaseURL, "down")
	case "worker", "scheduler":
		<-ctx.Done()
		return nil
	default:
		return errors.New(Usage)
	}
}

func runHTTPServer(ctx context.Context, cfg config.Config, log zerolog.Logger) error {
	capacityApp, closer, err := openCapacityApplication(ctx, cfg, log)
	if err != nil {
		return err
	}
	defer func() {
		_ = closer.Close()
	}()

	neighborhoodApp, neighborhoodCloser, err := openNeighborhoodApplication(ctx, cfg, log)
	if err != nil {
		return err
	}
	defer func() {
		_ = neighborhoodCloser.Close()
	}()

	collectionApp, collectionCloser, err := openCollectionApplication(ctx, cfg, log)
	if err != nil {
		return err
	}
	defer func() {
		_ = collectionCloser.Close()
	}()

	engine := router.New(router.Dependencies{
		Log:                     log,
		StaticFS:                web.Embedded(),
		CapacityApplication:     capacityApp,
		NeighborhoodApplication: neighborhoodApp,
		CollectionApplication:   collectionApp,
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
		errCh <- listenAndServe(server)
	}()

	err = <-errCh
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}

	return err
}
