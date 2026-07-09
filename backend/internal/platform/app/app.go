package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	appcapacity "github.com/propulse/propulse/backend/internal/application/capacity"
	appcollection "github.com/propulse/propulse/backend/internal/application/collection"
	appmetric "github.com/propulse/propulse/backend/internal/application/metric"
	appneighborhood "github.com/propulse/propulse/backend/internal/application/neighborhood"
	appqueue "github.com/propulse/propulse/backend/internal/application/queue"
	"github.com/propulse/propulse/backend/internal/infrastructure/config"
	migraterunner "github.com/propulse/propulse/backend/internal/infrastructure/migrate"
	postgresgorm "github.com/propulse/propulse/backend/internal/infrastructure/postgres/gorm"
	"github.com/propulse/propulse/backend/internal/infrastructure/postgres/sqlmetric"
	infrastructurequeue "github.com/propulse/propulse/backend/internal/infrastructure/queue"
	"github.com/propulse/propulse/backend/internal/interfaces/http/router"
	"github.com/propulse/propulse/backend/web"
	"github.com/rs/zerolog"
)

const Usage = "usage: propulse [serve|api|worker|scheduler|migrate up|migrate down]"
const schedulerSourceID = "scheduler.watchlist"

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
	ListWatchlistNeighborhoodIDs(ctx context.Context, query appneighborhood.ListWatchlistNeighborhoodIDsQuery) ([]string, error)
}

type CollectionApplication interface {
	ImportManualListings(ctx context.Context, command appcollection.ImportManualListingsCommand) (appcollection.ImportManualListingsResult, error)
}

type MetricApplication interface {
	CalculateNeighborhood(ctx context.Context, neighborhoodID string) error
}

type MetricTaskEnqueuer interface {
	EnqueueMetricCalculateNeighborhood(ctx context.Context, neighborhoodID string, sourceID string) error
}

var runMigrations = migraterunner.Run
var runHTTPServerFunc = runHTTPServer
var startQueueWorker = runQueueWorker
var startScheduler = runScheduler
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

	metricPool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		_ = sqlDB.Close()
		return nil, nil, err
	}

	repo := postgresgorm.NewNeighborhoodRepositoryWithMetricReader(db, sqlmetric.NewRepository(metricPool))
	if cfg.SeedDemoData {
		if err := repo.SeedDemoData(ctx); err != nil {
			metricPool.Close()
			_ = sqlDB.Close()
			return nil, nil, err
		}
		log.Info().Msg("seeded demo neighborhood data")
	}

	service := appneighborhood.NewService(repo)
	return service, multiCloser{
		closers: []io.Closer{
			sqlDB,
			closerFunc(func() error {
				metricPool.Close()
				return nil
			}),
		},
	}, nil
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

var openMetricApplication = func(ctx context.Context, cfg config.Config, log zerolog.Logger) (MetricApplication, io.Closer, error) {
	metricPool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, nil, err
	}

	service := appmetric.NewService(sqlmetric.NewRepository(metricPool))
	return service, closerFunc(func() error {
		metricPool.Close()
		return nil
	}), nil
}

var openMetricQueueClient = func(cfg config.Config) (MetricTaskEnqueuer, io.Closer, error) {
	client := infrastructurequeue.NewClient(cfg.RedisAddr)
	return client, client, nil
}

var listenAndServe = func(server *http.Server) error {
	return server.ListenAndServe()
}

type closerFunc func() error

func (f closerFunc) Close() error {
	return f()
}

type multiCloser struct {
	closers []io.Closer
}

func (c multiCloser) Close() error {
	var closeErr error
	for _, closer := range c.closers {
		if err := closer.Close(); err != nil && closeErr == nil {
			closeErr = err
		}
	}
	return closeErr
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
	case "serve":
		return runComponents(ctx,
			func(ctx context.Context) error { return runHTTPServerFunc(ctx, cfg, log) },
			func(ctx context.Context) error { return startQueueWorker(ctx, cfg, log) },
			func(ctx context.Context) error { return startScheduler(ctx, cfg, log) },
		)
	case "api":
		return runHTTPServerFunc(ctx, cfg, log)
	case "migrate up":
		return runMigrations(ctx, cfg.DatabaseURL, "up")
	case "migrate down":
		return runMigrations(ctx, cfg.DatabaseURL, "down")
	case "worker":
		return startQueueWorker(ctx, cfg, log)
	case "scheduler":
		return startScheduler(ctx, cfg, log)
	default:
		return errors.New(Usage)
	}
}

func runComponents(parent context.Context, components ...func(context.Context) error) error {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()

	errCh := make(chan error, len(components))
	for _, component := range components {
		go func(component func(context.Context) error) {
			errCh <- component(ctx)
		}(component)
	}

	var firstErr error
	for range components {
		err := <-errCh
		cancel()
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func runQueueWorker(ctx context.Context, cfg config.Config, log zerolog.Logger) error {
	metricApp, closer, err := openMetricApplication(ctx, cfg, log)
	if err != nil {
		return err
	}
	defer func() {
		_ = closer.Close()
	}()

	log.Info().Msg("starting asynq worker")
	return infrastructurequeue.NewServer(cfg.RedisAddr, metricApp, log).Run(ctx)
}

func runScheduler(ctx context.Context, cfg config.Config, log zerolog.Logger) error {
	neighborhoodApp, neighborhoodCloser, err := openNeighborhoodApplication(ctx, cfg, log)
	if err != nil {
		return err
	}
	defer func() {
		_ = neighborhoodCloser.Close()
	}()

	enqueuer, queueCloser, err := openMetricQueueClient(cfg)
	if err != nil {
		return err
	}
	defer func() {
		_ = queueCloser.Close()
	}()

	interval := cfg.SchedulerInterval
	if interval <= 0 {
		interval = time.Hour
	}

	if err := enqueueWatchlistMetricJobs(ctx, neighborhoodApp, enqueuer, log); err != nil {
		return err
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := enqueueWatchlistMetricJobs(ctx, neighborhoodApp, enqueuer, log); err != nil {
				return err
			}
		}
	}
}

func enqueueWatchlistMetricJobs(ctx context.Context, neighborhoodApp NeighborhoodApplication, enqueuer MetricTaskEnqueuer, log zerolog.Logger) error {
	neighborhoodIDs, err := neighborhoodApp.ListWatchlistNeighborhoodIDs(ctx, appneighborhood.ListWatchlistNeighborhoodIDsQuery{})
	if err != nil {
		return err
	}

	for _, neighborhoodID := range neighborhoodIDs {
		log.Info().
			Str("task_type", appqueue.TypeMetricCalculateNeighborhood).
			Str("source_id", schedulerSourceID).
			Str("neighborhood_id", neighborhoodID).
			Msg("enqueueing metric calculation")
		if err := enqueuer.EnqueueMetricCalculateNeighborhood(ctx, neighborhoodID, schedulerSourceID); err != nil {
			return err
		}
	}
	return nil
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
