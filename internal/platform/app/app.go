package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/rs/zerolog"
	webembed "github.com/sine-io/propulse/apps/web/embed"
	appcapacity "github.com/sine-io/propulse/internal/application/capacity"
	appcollection "github.com/sine-io/propulse/internal/application/collection"
	appneighborhood "github.com/sine-io/propulse/internal/application/neighborhood"
	appqueue "github.com/sine-io/propulse/internal/application/queue"
	"github.com/sine-io/propulse/internal/infrastructure/config"
	migraterunner "github.com/sine-io/propulse/internal/infrastructure/migrate"
	infrastructurequeue "github.com/sine-io/propulse/internal/infrastructure/queue"
	"github.com/sine-io/propulse/internal/interfaces/http/router"
)

const Usage = "usage: propulse [serve|api|worker|scheduler|migrate up|migrate down]"
const schedulerSourceID = "scheduler.watchlist"

type CapacityApplication interface {
	CreateCalculation(ctx context.Context, command appcapacity.CreateCalculationCommand) (appcapacity.CalculationRecord, error)
	GetCalculation(ctx context.Context, query appcapacity.GetCalculationQuery) (appcapacity.CalculationRecord, error)
	LatestCalculation(ctx context.Context, query appcapacity.LatestCalculationQuery) (appcapacity.CalculationRecord, error)
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
var openRuntimeFunc = openRuntime
var runHTTPServerFunc = runHTTPServer
var startQueueWorker = runQueueWorker
var startScheduler = runScheduler

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
	case "migrate up":
		return runMigrations(ctx, cfg.DatabaseURL, "up")
	case "migrate down":
		return runMigrations(ctx, cfg.DatabaseURL, "down")
	case "serve", "api", "worker", "scheduler":
	default:
		return errors.New(Usage)
	}

	rt, err := openRuntimeFunc(ctx, cfg, log)
	if err != nil {
		return err
	}
	defer func() {
		_ = rt.Close()
	}()

	switch mode {
	case "serve":
		return runComponents(ctx,
			func(ctx context.Context) error { return runHTTPServerFunc(ctx, cfg, log, rt) },
			func(ctx context.Context) error { return startQueueWorker(ctx, cfg, log, rt) },
			func(ctx context.Context) error { return startScheduler(ctx, cfg, log, rt) },
		)
	case "api":
		return runHTTPServerFunc(ctx, cfg, log, rt)
	case "worker":
		return startQueueWorker(ctx, cfg, log, rt)
	case "scheduler":
		return startScheduler(ctx, cfg, log, rt)
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

func runQueueWorker(ctx context.Context, cfg config.Config, log zerolog.Logger, rt *runtime) error {
	log.Info().Msg("starting asynq worker")
	return infrastructurequeue.NewServer(cfg.RedisAddr, rt.metric, log).Run(ctx)
}

func runScheduler(ctx context.Context, cfg config.Config, log zerolog.Logger, rt *runtime) error {
	interval := cfg.SchedulerInterval
	if interval <= 0 {
		interval = time.Hour
	}

	if err := enqueueWatchlistMetricJobs(ctx, rt.neighborhood, rt.enqueuer, log); err != nil {
		return err
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := enqueueWatchlistMetricJobs(ctx, rt.neighborhood, rt.enqueuer, log); err != nil {
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

func runHTTPServer(ctx context.Context, cfg config.Config, log zerolog.Logger, rt *runtime) error {
	engine := router.New(router.Dependencies{
		Log:                     log,
		StaticFS:                webembed.Embedded(),
		CapacityApplication:     rt.capacity,
		NeighborhoodApplication: rt.neighborhood,
		CollectionApplication:   rt.collection,
		AccessToken:             cfg.AccessToken,
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

	err := <-errCh
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}

	return err
}
