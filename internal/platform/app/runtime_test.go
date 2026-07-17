package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	redisclient "github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/sine-io/propulse/internal/infrastructure/config"
	infrastructureredis "github.com/sine-io/propulse/internal/infrastructure/redis"
	"gorm.io/gorm"
)

func TestOpenRuntimeCleansUpOpenedResourcesOnFailure(t *testing.T) {
	openErr := errors.New("open failed")
	tests := []struct {
		name       string
		configure  func()
		cfg        config.Config
		wantClosed []string
	}{
		{
			name: "postgres",
			configure: func() {
				openPostgres = func(string) (*gorm.DB, *sql.DB, error) {
					return nil, nil, openErr
				}
			},
			wantClosed: nil,
		},
		{
			name: "pgx",
			configure: func() {
				openPGXPool = func(context.Context, string) (*pgxpool.Pool, error) {
					return nil, openErr
				}
			},
			wantClosed: []string{"sql"},
		},
		{
			name: "redis",
			configure: func() {
				openRedisClient = func(string) (*redisclient.Client, error) {
					return nil, openErr
				}
			},
			wantClosed: []string{"pgx", "sql"},
		},
		{
			name: "queue",
			configure: func() {
				openQueueClient = func(string) (MetricTaskEnqueuer, io.Closer, error) {
					return nil, nil, openErr
				}
			},
			wantClosed: []string{"redis", "pgx", "sql"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			preserveRuntimeSeams(t)
			tracker := newResourceCloseTracker()

			openPostgres = func(string) (*gorm.DB, *sql.DB, error) {
				return &gorm.DB{}, &sql.DB{}, nil
			}
			openPGXPool = func(context.Context, string) (*pgxpool.Pool, error) {
				return &pgxpool.Pool{}, nil
			}
			openRedisClient = func(string) (*redisclient.Client, error) {
				return &redisclient.Client{}, nil
			}
			openQueueClient = func(string) (MetricTaskEnqueuer, io.Closer, error) {
				queue := &trackedQueueClient{closer: tracker.closer("queue", nil)}
				return queue, queue, nil
			}
			closeRedisClient = func(*redisclient.Client) error {
				return tracker.close("redis", nil)
			}
			closePGXPool = func(*pgxpool.Pool) {
				_ = tracker.close("pgx", nil)
			}
			closeSQLDB = func(*sql.DB) error {
				return tracker.close("sql", nil)
			}

			tt.configure()
			rt, err := openRuntime(context.Background(), tt.cfg, zerolog.New(io.Discard))
			if !errors.Is(err, openErr) {
				t.Fatalf("openRuntime() error = %v, want %v", err, openErr)
			}
			if rt != nil {
				t.Fatalf("openRuntime() runtime = %p, want nil", rt)
			}
			tracker.assertClosed(t, tt.wantClosed)
		})
	}
}

func TestRuntimeCloseAttemptsEveryResourceInReverseOrderAndReturnsFirstError(t *testing.T) {
	preserveRuntimeSeams(t)
	tracker := newResourceCloseTracker()
	queueErr := errors.New("queue close failed")
	redisErr := errors.New("redis close failed")
	sqlErr := errors.New("sql close failed")

	closeRedisClient = func(*redisclient.Client) error {
		return tracker.close("redis", redisErr)
	}
	closePGXPool = func(*pgxpool.Pool) {
		_ = tracker.close("pgx", nil)
	}
	closeSQLDB = func(*sql.DB) error {
		return tracker.close("sql", sqlErr)
	}

	rt := &runtime{
		sqlDB:       &sql.DB{},
		pgxPool:     &pgxpool.Pool{},
		redis:       &redisclient.Client{},
		queueClient: tracker.closer("queue", queueErr),
	}
	if err := rt.Close(); !errors.Is(err, queueErr) {
		t.Fatalf("Close() error = %v, want first error %v", err, queueErr)
	}
	if err := rt.Close(); !errors.Is(err, queueErr) {
		t.Fatalf("second Close() error = %v, want cached first error %v", err, queueErr)
	}
	tracker.assertClosed(t, []string{"queue", "redis", "pgx", "sql"})
}

func TestOpenRuntimeBuildsOneSharedApplicationSet(t *testing.T) {
	preserveRuntimeSeams(t)
	tracker := newResourceCloseTracker()
	queue := &trackedQueueClient{closer: tracker.closer("queue", nil)}

	openPostgres = func(string) (*gorm.DB, *sql.DB, error) {
		return &gorm.DB{}, &sql.DB{}, nil
	}
	openPGXPool = func(context.Context, string) (*pgxpool.Pool, error) {
		return &pgxpool.Pool{}, nil
	}
	openRedisClient = func(string) (*redisclient.Client, error) {
		return &redisclient.Client{}, nil
	}
	openQueueClient = func(string) (MetricTaskEnqueuer, io.Closer, error) {
		return queue, queue, nil
	}
	closeRedisClient = func(*redisclient.Client) error {
		return tracker.close("redis", nil)
	}
	closePGXPool = func(*pgxpool.Pool) {
		_ = tracker.close("pgx", nil)
	}
	closeSQLDB = func(*sql.DB) error {
		return tracker.close("sql", nil)
	}

	rt, err := openRuntime(context.Background(), config.Config{}, zerolog.New(io.Discard))
	if err != nil {
		t.Fatalf("openRuntime() error = %v", err)
	}
	if rt.capacity == nil || rt.neighborhood == nil || rt.collection == nil || rt.communityMarket == nil || rt.metric == nil || rt.review == nil {
		t.Fatalf("runtime application set is incomplete: %+v", rt)
	}
	if rt.enqueuer != queue || rt.queueClient != queue {
		t.Fatal("runtime does not share the opened queue client with the scheduler enqueuer")
	}
	if err := rt.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	tracker.assertClosed(t, []string{"queue", "redis", "pgx", "sql"})
}

func TestOpenRuntimeBuildsReadinessCheckerFromOwnedResources(t *testing.T) {
	preserveRuntimeSeams(t)

	sqlDB := &sql.DB{}
	pgxPool := &pgxpool.Pool{}
	redisClient := &redisclient.Client{}
	queue := &trackedQueueClient{closer: noopCloser{}}
	openPostgres = func(string) (*gorm.DB, *sql.DB, error) {
		return &gorm.DB{}, sqlDB, nil
	}
	openPGXPool = func(context.Context, string) (*pgxpool.Pool, error) {
		return pgxPool, nil
	}
	openRedisClient = func(string) (*redisclient.Client, error) {
		return redisClient, nil
	}
	openQueueClient = func(string) (MetricTaskEnqueuer, io.Closer, error) {
		return queue, queue, nil
	}
	closeSQLDB = func(*sql.DB) error { return nil }
	closePGXPool = func(*pgxpool.Pool) {}
	closeRedisClient = func(*redisclient.Client) error { return nil }

	rt, err := openRuntime(context.Background(), config.Config{AccessToken: "runtime-token"}, zerolog.New(io.Discard))
	if err != nil {
		t.Fatalf("openRuntime() error = %v", err)
	}
	t.Cleanup(func() { _ = rt.Close() })

	checker, ok := rt.readiness.(readinessChecker)
	if !ok {
		t.Fatalf("runtime readiness type = %T, want readinessChecker", rt.readiness)
	}
	if checker.sql != sqlDB {
		t.Fatal("readiness checker does not use runtime SQL DB")
	}
	if checker.pgx != pgxPool {
		t.Fatal("readiness checker does not use runtime pgx pool")
	}
	redisPinger, ok := checker.redis.(infrastructureredis.PingClient)
	if !ok {
		t.Fatalf("readiness Redis pinger type = %T, want redis.PingClient", checker.redis)
	}
	if redisPinger != infrastructureredis.NewPingClient(redisClient) {
		t.Fatal("readiness checker does not use runtime Redis client")
	}
	if checker.accessToken != "runtime-token" {
		t.Fatalf("readiness access token = %q, want runtime-token", checker.accessToken)
	}
}

func preserveRuntimeSeams(t *testing.T) {
	t.Helper()
	originalOpenPostgres := openPostgres
	originalOpenPGXPool := openPGXPool
	originalOpenRedisClient := openRedisClient
	originalOpenQueueClient := openQueueClient
	originalCloseSQLDB := closeSQLDB
	originalClosePGXPool := closePGXPool
	originalCloseRedisClient := closeRedisClient
	t.Cleanup(func() {
		openPostgres = originalOpenPostgres
		openPGXPool = originalOpenPGXPool
		openRedisClient = originalOpenRedisClient
		openQueueClient = originalOpenQueueClient
		closeSQLDB = originalCloseSQLDB
		closePGXPool = originalClosePGXPool
		closeRedisClient = originalCloseRedisClient
	})
}

type resourceCloseTracker struct {
	closed []string
	counts map[string]int
}

func newResourceCloseTracker() *resourceCloseTracker {
	return &resourceCloseTracker{counts: map[string]int{}}
}

func (t *resourceCloseTracker) closer(name string, err error) io.Closer {
	return trackedResourceCloser{tracker: t, name: name, err: err}
}

func (t *resourceCloseTracker) close(name string, err error) error {
	t.closed = append(t.closed, name)
	t.counts[name]++
	return err
}

func (t *resourceCloseTracker) assertClosed(testingT *testing.T, want []string) {
	testingT.Helper()
	if got := fmt.Sprint(t.closed); got != fmt.Sprint(want) {
		testingT.Fatalf("close order = %s, want %s", got, fmt.Sprint(want))
	}
	for _, name := range want {
		if t.counts[name] != 1 {
			testingT.Fatalf("%s close count = %d, want 1", name, t.counts[name])
		}
	}
}

type trackedResourceCloser struct {
	tracker *resourceCloseTracker
	name    string
	err     error
}

func (c trackedResourceCloser) Close() error {
	return c.tracker.close(c.name, c.err)
}

type trackedQueueClient struct {
	closer io.Closer
}

func (*trackedQueueClient) EnqueueMetricCalculateNeighborhood(context.Context, string, string, string) error {
	return nil
}

func (c *trackedQueueClient) Close() error {
	return c.closer.Close()
}
