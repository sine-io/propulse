package app

import (
	"context"
	"database/sql"
	"io"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	redisclient "github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	appcapacity "github.com/sine-io/propulse/internal/application/capacity"
	appcollection "github.com/sine-io/propulse/internal/application/collection"
	appdecision "github.com/sine-io/propulse/internal/application/decision"
	appmetric "github.com/sine-io/propulse/internal/application/metric"
	appneighborhood "github.com/sine-io/propulse/internal/application/neighborhood"
	"github.com/sine-io/propulse/internal/infrastructure/config"
	postgresgorm "github.com/sine-io/propulse/internal/infrastructure/postgres/gorm"
	"github.com/sine-io/propulse/internal/infrastructure/postgres/sqlmetric"
	infrastructurequeue "github.com/sine-io/propulse/internal/infrastructure/queue"
	infrastructureredis "github.com/sine-io/propulse/internal/infrastructure/redis"
	"github.com/sine-io/propulse/internal/interfaces/http/router"
	"gorm.io/gorm"
)

type runtime struct {
	gormDB      *gorm.DB
	sqlDB       *sql.DB
	pgxPool     *pgxpool.Pool
	redis       *redisclient.Client
	queueClient io.Closer

	capacity     CapacityApplication
	neighborhood NeighborhoodApplication
	collection   CollectionApplication
	decision     DecisionApplication
	metric       MetricApplication
	enqueuer     MetricTaskEnqueuer
	readiness    router.ReadinessChecker

	closeOnce sync.Once
	closeErr  error
}

var openPostgres = postgresgorm.Open
var openPGXPool = pgxpool.New
var openRedisClient = func(addr string) (*redisclient.Client, error) {
	return infrastructureredis.New(addr), nil
}
var openQueueClient = func(addr string) (MetricTaskEnqueuer, io.Closer, error) {
	client := infrastructurequeue.NewClient(addr)
	return client, client, nil
}
var closeSQLDB = (*sql.DB).Close
var closePGXPool = (*pgxpool.Pool).Close
var closeRedisClient = (*redisclient.Client).Close

func openRuntime(ctx context.Context, cfg config.Config, _ zerolog.Logger) (*runtime, error) {
	rt := &runtime{}

	gormDB, sqlDB, err := openPostgres(cfg.DatabaseURL)
	if err != nil {
		return nil, err
	}
	rt.gormDB = gormDB
	rt.sqlDB = sqlDB

	pgxPool, err := openPGXPool(ctx, cfg.DatabaseURL)
	if err != nil {
		_ = rt.Close()
		return nil, err
	}
	rt.pgxPool = pgxPool

	redisClient, err := openRedisClient(cfg.RedisAddr)
	if err != nil {
		_ = rt.Close()
		return nil, err
	}
	rt.redis = redisClient
	rt.readiness = newReadinessChecker(
		rt.sqlDB,
		rt.pgxPool,
		infrastructureredis.NewPingClient(rt.redis),
		cfg.AccessToken,
	)

	enqueuer, queueClient, err := openQueueClient(cfg.RedisAddr)
	if err != nil {
		_ = rt.Close()
		return nil, err
	}
	rt.enqueuer = enqueuer
	rt.queueClient = queueClient

	capacityRepo := postgresgorm.NewCapacityRepository(gormDB)
	metricRepo := sqlmetric.NewRepository(pgxPool, cfg.MetricAlgorithmVersion)
	neighborhoodRepo := postgresgorm.NewNeighborhoodRepositoryWithMetricReader(gormDB, metricRepo)
	collectionRepo := postgresgorm.NewCollectionRepository(gormDB)

	rt.capacity = appcapacity.NewService(capacityRepo, cfg.CapacityAssumptions, time.Now, nil)
	rt.metric = appmetric.NewService(metricRepo, cfg.MetricAlgorithmVersion)
	rt.neighborhood = appneighborhood.NewServiceWithMetricConfig(neighborhoodRepo, cfg.MetricAlgorithmVersion, time.Now)
	rt.collection = appcollection.NewServiceWithMetricRefresh(collectionRepo, time.Now, nil, rt.metric, rt.enqueuer, cfg.MetricAlgorithmVersion)
	rt.decision = appdecision.NewService(rt.capacity, rt.neighborhood, cfg.UserID)

	return rt, nil
}

func (r *runtime) Close() error {
	r.closeOnce.Do(func() {
		var firstErr error
		if r.queueClient != nil {
			if err := r.queueClient.Close(); err != nil && firstErr == nil {
				firstErr = err
			}
		}
		if r.redis != nil {
			if err := closeRedisClient(r.redis); err != nil && firstErr == nil {
				firstErr = err
			}
		}
		if r.pgxPool != nil {
			closePGXPool(r.pgxPool)
		}
		if r.sqlDB != nil {
			if err := closeSQLDB(r.sqlDB); err != nil && firstErr == nil {
				firstErr = err
			}
		}
		r.closeErr = firstErr
	})
	return r.closeErr
}
