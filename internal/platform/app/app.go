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
	appdecision "github.com/sine-io/propulse/internal/application/decision"
	appmetric "github.com/sine-io/propulse/internal/application/metric"
	appneighborhood "github.com/sine-io/propulse/internal/application/neighborhood"
	appqueue "github.com/sine-io/propulse/internal/application/queue"
	domaincapacity "github.com/sine-io/propulse/internal/domain/capacity"
	"github.com/sine-io/propulse/internal/infrastructure/config"
	migraterunner "github.com/sine-io/propulse/internal/infrastructure/migrate"
	infrastructurequeue "github.com/sine-io/propulse/internal/infrastructure/queue"
	"github.com/sine-io/propulse/internal/interfaces/http/router"
)

const Usage = "usage: propulse [serve|api|worker|scheduler|migrate up|migrate down]"
const schedulerSourceID = "scheduler.metric_repair"
const schedulerMetricRepairGracePeriod = 5 * time.Minute
const schedulerMetricRepairBatchSize = 100

type CapacityApplication interface {
	CreateCalculation(ctx context.Context, command appcapacity.CreateCalculationCommand) (appcapacity.CalculationRecord, error)
	GetAssumptions(ctx context.Context, query appcapacity.GetAssumptionsQuery) (domaincapacity.Assumptions, error)
	GetCalculation(ctx context.Context, query appcapacity.GetCalculationQuery) (appcapacity.CalculationRecord, error)
	LatestCalculation(ctx context.Context, query appcapacity.LatestCalculationQuery) (appcapacity.CalculationRecord, error)
}

type NeighborhoodApplication interface {
	CreateNeighborhood(ctx context.Context, command appneighborhood.CreateNeighborhoodCommand) (appneighborhood.Neighborhood, error)
	GetNeighborhood(ctx context.Context, query appneighborhood.GetNeighborhoodQuery) (appneighborhood.Neighborhood, error)
	SearchNeighborhoods(ctx context.Context, query appneighborhood.SearchNeighborhoodsQuery) (appneighborhood.SearchNeighborhoodsPage, error)
	LatestMetric(ctx context.Context, query appneighborhood.LatestMetricQuery) (appneighborhood.MetricWithSignal, error)
	MetricHistory(ctx context.Context, query appneighborhood.MetricHistoryQuery) (appneighborhood.MetricHistoryResult, error)
	AddWatchlistItem(ctx context.Context, command appneighborhood.AddWatchlistItemCommand) (appneighborhood.WatchlistItem, error)
	ListWatchlist(ctx context.Context, query appneighborhood.ListWatchlistQuery) ([]appneighborhood.WatchlistItemSummary, error)
}

type CollectionApplication interface {
	CreateDataSource(ctx context.Context, command appcollection.CreateDataSourceCommand) (appcollection.DataSource, error)
	ListDataSources(ctx context.Context, query appcollection.ListDataSourcesQuery) ([]appcollection.DataSource, error)
	ImportCollectionRun(ctx context.Context, command appcollection.ImportCollectionRunCommand) (appcollection.ImportCollectionRunResult, error)
	GetCollectionRun(ctx context.Context, query appcollection.GetCollectionRunQuery) (appcollection.CollectionRunDetail, error)
	ListMetricRefreshCandidates(ctx context.Context, query appcollection.ListMetricRefreshCandidatesQuery) ([]appcollection.MetricRefreshCandidate, error)
}

type DecisionApplication interface {
	GetActionWindow(ctx context.Context, query appdecision.GetActionWindowQuery) (appdecision.ActionWindowResult, error)
}

type MetricApplication interface {
	CalculateCollectionRun(ctx context.Context, command appmetric.CalculateCollectionRunCommand) error
}

type MetricTaskEnqueuer interface {
	EnqueueMetricCalculateNeighborhood(ctx context.Context, neighborhoodID string, collectionRunID string, sourceID string) error
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

	if err := enqueueMetricRepairJobs(ctx, rt.collection, rt.enqueuer, log); err != nil {
		return err
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := enqueueMetricRepairJobs(ctx, rt.collection, rt.enqueuer, log); err != nil {
				return err
			}
		}
	}
}

func enqueueMetricRepairJobs(ctx context.Context, collectionApp CollectionApplication, enqueuer MetricTaskEnqueuer, log zerolog.Logger) error {
	candidates, err := collectionApp.ListMetricRefreshCandidates(ctx, appcollection.ListMetricRefreshCandidatesQuery{
		UpdatedBefore: time.Now().UTC().Add(-schedulerMetricRepairGracePeriod),
		Limit:         schedulerMetricRepairBatchSize,
	})
	if err != nil {
		return err
	}

	for _, candidate := range candidates {
		log.Info().
			Str("task_type", appqueue.TypeMetricCalculateNeighborhood).
			Str("source_id", schedulerSourceID).
			Str("collection_run_id", candidate.CollectionRunID).
			Str("neighborhood_id", candidate.NeighborhoodID).
			Msg("enqueueing metric repair")
		if err := enqueuer.EnqueueMetricCalculateNeighborhood(ctx, candidate.NeighborhoodID, candidate.CollectionRunID, schedulerSourceID); err != nil {
			return err
		}
	}
	return nil
}

func runHTTPServer(ctx context.Context, cfg config.Config, log zerolog.Logger, rt *runtime) error {
	engine, err := router.New(router.Dependencies{
		Log:                     log,
		StaticFS:                webembed.Embedded(),
		CapacityApplication:     rt.capacity,
		NeighborhoodApplication: rt.neighborhood,
		CollectionApplication:   rt.collection,
		DecisionApplication:     rt.decision,
		AccessToken:             cfg.AccessToken,
		UserID:                  cfg.UserID,
		ReadinessChecker:        rt.readiness,
	})
	if err != nil {
		return err
	}

	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           engine,
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- listenAndServe(server)
	}()

	var serveErr error
	select {
	case serveErr = <-errCh:
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_ = server.Shutdown(shutdownCtx)
		cancel()
		serveErr = <-errCh
	}
	if errors.Is(serveErr, http.ErrServerClosed) {
		return nil
	}

	return serveErr
}
