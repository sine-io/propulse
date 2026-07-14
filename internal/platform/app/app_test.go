package app

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
	appcapacity "github.com/sine-io/propulse/internal/application/capacity"
	appcollection "github.com/sine-io/propulse/internal/application/collection"
	appdecision "github.com/sine-io/propulse/internal/application/decision"
	appneighborhood "github.com/sine-io/propulse/internal/application/neighborhood"
	domaincapacity "github.com/sine-io/propulse/internal/domain/capacity"
	domaindecision "github.com/sine-io/propulse/internal/domain/decision"
	domainneighborhood "github.com/sine-io/propulse/internal/domain/neighborhood"
	"github.com/sine-io/propulse/internal/infrastructure/config"
)

func TestNormalizeModeAcceptsDocumentedModes(t *testing.T) {
	for _, args := range [][]string{
		{"serve"},
		{"api"},
		{"worker"},
		{"scheduler"},
		{"migrate", "up"},
		{"migrate", "down"},
	} {
		mode, err := NormalizeMode(args)
		if err != nil {
			t.Fatalf("NormalizeMode(%v) error = %v", args, err)
		}
		if mode == "" {
			t.Fatalf("NormalizeMode(%v) returned empty mode", args)
		}
	}
}

func TestRunAPIModeOpensAndClosesOneRuntime(t *testing.T) {
	originalOpenRuntime := openRuntimeFunc
	originalRunHTTPServer := runHTTPServerFunc
	defer func() {
		openRuntimeFunc = originalOpenRuntime
		runHTTPServerFunc = originalRunHTTPServer
	}()

	closer := &countingCloser{}
	rt := &runtime{queueClient: closer}
	openCount := 0
	openRuntimeFunc = func(_ context.Context, _ config.Config, _ zerolog.Logger) (*runtime, error) {
		openCount++
		return rt, nil
	}

	var received *runtime
	runHTTPServerFunc = func(_ context.Context, _ config.Config, _ zerolog.Logger, got *runtime) error {
		received = got
		return nil
	}

	if err := Run(context.Background(), "api", config.Config{}, zerolog.New(io.Discard)); err != nil {
		t.Fatalf("Run(api) error = %v", err)
	}
	if openCount != 1 {
		t.Fatalf("openRuntime call count = %d, want 1", openCount)
	}
	if received != rt {
		t.Fatalf("HTTP runtime = %p, want %p", received, rt)
	}
	if closer.count != 1 {
		t.Fatalf("runtime close count = %d, want 1", closer.count)
	}
}

func TestRunServeModeSharesOneRuntimeAcrossHTTPWorkerScheduler(t *testing.T) {
	originalOpenRuntime := openRuntimeFunc
	originalRunHTTPServer := runHTTPServerFunc
	originalStartQueueWorker := startQueueWorker
	originalStartScheduler := startScheduler
	defer func() {
		openRuntimeFunc = originalOpenRuntime
		runHTTPServerFunc = originalRunHTTPServer
		startQueueWorker = originalStartQueueWorker
		startScheduler = originalStartScheduler
	}()

	rt := &runtime{queueClient: noopCloser{}}
	openCount := 0
	openRuntimeFunc = func(_ context.Context, _ config.Config, _ zerolog.Logger) (*runtime, error) {
		openCount++
		return rt, nil
	}

	var (
		mu         sync.Mutex
		httpRT     *runtime
		workerRT   *runtime
		scheduleRT *runtime
	)
	runHTTPServerFunc = func(_ context.Context, _ config.Config, _ zerolog.Logger, got *runtime) error {
		mu.Lock()
		httpRT = got
		mu.Unlock()
		return nil
	}
	startQueueWorker = func(_ context.Context, _ config.Config, _ zerolog.Logger, got *runtime) error {
		mu.Lock()
		workerRT = got
		mu.Unlock()
		return nil
	}
	startScheduler = func(_ context.Context, _ config.Config, _ zerolog.Logger, got *runtime) error {
		mu.Lock()
		scheduleRT = got
		mu.Unlock()
		return nil
	}

	if err := Run(context.Background(), "serve", config.Config{}, zerolog.New(io.Discard)); err != nil {
		t.Fatalf("Run(serve) error = %v", err)
	}
	if openCount != 1 {
		t.Fatalf("openRuntime call count = %d, want 1", openCount)
	}
	mu.Lock()
	defer mu.Unlock()
	if httpRT != rt || workerRT != rt || scheduleRT != rt {
		t.Fatalf("runtime pointers = HTTP %p worker %p scheduler %p, want all %p", httpRT, workerRT, scheduleRT, rt)
	}
}

func TestRunMigrateModeDoesNotOpenRuntime(t *testing.T) {
	originalOpenRuntime := openRuntimeFunc
	originalRunMigrations := runMigrations
	defer func() {
		openRuntimeFunc = originalOpenRuntime
		runMigrations = originalRunMigrations
	}()

	openCount := 0
	openRuntimeFunc = func(_ context.Context, _ config.Config, _ zerolog.Logger) (*runtime, error) {
		openCount++
		return nil, errors.New("runtime must not be opened for migrations")
	}
	runMigrations = func(_ context.Context, _ string, direction string) error {
		if direction != "up" {
			t.Fatalf("migration direction = %q, want up", direction)
		}
		return nil
	}

	if err := Run(context.Background(), "migrate up", config.Config{}, zerolog.New(io.Discard)); err != nil {
		t.Fatalf("Run(migrate up) error = %v", err)
	}
	if openCount != 0 {
		t.Fatalf("openRuntime call count = %d, want 0", openCount)
	}
}

func TestRunStartsHTTPServerForAPIMode(t *testing.T) {
	testRunStartsHTTPServer(t, "api")
}

func TestRunStartsHTTPServerForServeMode(t *testing.T) {
	testRunStartsHTTPServer(t, "serve")
}

func TestRunWaitsForInFlightHTTPHandlerBeforeClosingRuntime(t *testing.T) {
	originalOpenRuntime := openRuntimeFunc
	defer func() { openRuntimeFunc = originalOpenRuntime }()

	handlerStarted := make(chan struct{})
	releaseHandler := make(chan struct{})
	var releaseOnce sync.Once
	release := func() {
		releaseOnce.Do(func() { close(releaseHandler) })
	}
	defer release()

	capacity := &stubAppCapacityApplication{
		createRecord: appcapacity.CalculationRecord{
			ID: "calc_1",
			Result: domaincapacity.HousingCapacityResult{
				PressureLevel: domaincapacity.PressureSafe,
				Strategy:      "hold",
			},
		},
		createStarted: handlerStarted,
		releaseCreate: releaseHandler,
	}
	runtimeClosed := make(chan struct{})
	rt := &runtime{
		capacity:     capacity,
		neighborhood: &stubAppNeighborhoodApplication{},
		collection:   &stubAppCollectionApplication{},
		decision:     &stubAppDecisionApplication{},
		queueClient:  closeSignalCloser{closed: runtimeClosed},
	}
	openRuntimeFunc = func(_ context.Context, _ config.Config, _ zerolog.Logger) (*runtime, error) {
		return rt, nil
	}

	addr := freeLocalAddr(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runErr := make(chan error, 1)
	go func() {
		runErr <- Run(ctx, "api", config.Config{
			HTTPAddr:    addr,
			AccessToken: "test-access-token",
		}, zerolog.New(io.Discard))
	}()

	waitForHTTP(t, "http://"+addr+"/healthz")
	requestErr := make(chan error, 1)
	go func() {
		body := `{"cashOnHand":150,"oldHomeValue":320,"oldLoanBalance":80,"monthlyIncome":3.5,"currentMonthlyMortgage":0,"acceptableMonthlyMortgage":1.5,"targetTotalPrice":550,"renovationBudget":40,"transactionCosts":18,"transitionRentCost":5}`
		req, err := http.NewRequest(http.MethodPost, "http://"+addr+"/api/v1/capacity/calculations", bytes.NewBufferString(body))
		if err != nil {
			requestErr <- err
			return
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer test-access-token")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			requestErr <- err
			return
		}
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusCreated {
			requestErr <- fmt.Errorf("response status = %d, want %d", resp.StatusCode, http.StatusCreated)
			return
		}
		requestErr <- nil
	}()

	select {
	case <-handlerStarted:
	case <-time.After(3 * time.Second):
		t.Fatal("capacity handler did not start")
	}

	cancel()
	select {
	case <-runtimeClosed:
		t.Fatal("runtime closed while an HTTP handler was still running")
	case err := <-runErr:
		t.Fatalf("Run(api) returned before the in-flight handler completed: %v", err)
	case <-time.After(100 * time.Millisecond):
	}

	release()
	select {
	case err := <-requestErr:
		if err != nil {
			t.Fatalf("in-flight request error = %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("in-flight request did not complete after release")
	}
	select {
	case err := <-runErr:
		if err != nil {
			t.Fatalf("Run(api) error = %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Run(api) did not return after the in-flight handler completed")
	}
	select {
	case <-runtimeClosed:
	case <-time.After(3 * time.Second):
		t.Fatal("runtime was not closed after Run(api) returned")
	}
}

func TestRunAPIModeDoesNotStartWorkerOrScheduler(t *testing.T) {
	calls := testRunModeComposition(t, "api")

	if calls.http != 1 || calls.worker != 0 || calls.scheduler != 0 {
		t.Fatalf("calls = %+v, want api to start HTTP only", calls)
	}
}

func TestRunServeModeStartsHTTPWorkerAndScheduler(t *testing.T) {
	calls := testRunModeComposition(t, "serve")

	if calls.http != 1 || calls.worker != 1 || calls.scheduler != 1 {
		t.Fatalf("calls = %+v, want serve to start HTTP, worker, and scheduler", calls)
	}
}

func TestRunWorkerModeStartsOnlyWorker(t *testing.T) {
	calls := testRunModeComposition(t, "worker")

	if calls.http != 0 || calls.worker != 1 || calls.scheduler != 0 {
		t.Fatalf("calls = %+v, want worker mode to start only worker", calls)
	}
}

func TestRunSchedulerModeStartsOnlyScheduler(t *testing.T) {
	calls := testRunModeComposition(t, "scheduler")

	if calls.http != 0 || calls.worker != 0 || calls.scheduler != 1 {
		t.Fatalf("calls = %+v, want scheduler mode to start only scheduler", calls)
	}
}

func TestRunSchedulerEnqueuesMetricRepairJobsForStaleRuns(t *testing.T) {
	collectionApp := &stubAppCollectionApplication{
		refreshCandidates: []appcollection.MetricRefreshCandidate{
			{CollectionRunID: "run_1", NeighborhoodID: "neighborhood_1"},
			{CollectionRunID: "run_2", NeighborhoodID: "neighborhood_2"},
		},
	}
	enqueuer := &stubMetricTaskEnqueuer{}
	rt := &runtime{collection: collectionApp, enqueuer: enqueuer}

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		errCh <- runScheduler(ctx, config.Config{SchedulerInterval: time.Hour}, zerolog.New(io.Discard), rt)
	}()

	deadline := time.After(2 * time.Second)
	for enqueuer.count() < 2 {
		select {
		case <-deadline:
			neighborhoodIDs, collectionRunIDs, _ := enqueuer.snapshot()
			t.Fatalf("scheduler did not enqueue metric repairs; got neighborhoods=%#v runs=%#v", neighborhoodIDs, collectionRunIDs)
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
	cancel()
	if err := <-errCh; err != nil {
		t.Fatalf("runScheduler() error = %v", err)
	}

	wantNeighborhoods := []string{"neighborhood_1", "neighborhood_2"}
	wantRuns := []string{"run_1", "run_2"}
	neighborhoodIDs, collectionRunIDs, sourceIDs := enqueuer.snapshot()
	if fmt.Sprint(neighborhoodIDs) != fmt.Sprint(wantNeighborhoods) || fmt.Sprint(collectionRunIDs) != fmt.Sprint(wantRuns) {
		t.Fatalf("enqueued jobs = neighborhoods %#v runs %#v", neighborhoodIDs, collectionRunIDs)
	}
	for _, sourceID := range sourceIDs {
		if sourceID != schedulerSourceID {
			t.Fatalf("sourceID = %q, want %q", sourceID, schedulerSourceID)
		}
	}
}

func TestEnqueueMetricRepairJobsUsesBoundedGracePeriodQuery(t *testing.T) {
	collectionApp := &stubAppCollectionApplication{
		refreshCandidates: []appcollection.MetricRefreshCandidate{
			{CollectionRunID: "run_1", NeighborhoodID: "neighborhood_1"},
		},
	}
	enqueuer := &stubMetricTaskEnqueuer{}
	startedAt := time.Now().UTC()

	if err := enqueueMetricRepairJobs(context.Background(), collectionApp, enqueuer, zerolog.New(io.Discard)); err != nil {
		t.Fatalf("enqueueMetricRepairJobs() error = %v", err)
	}

	if !collectionApp.refreshCalled {
		t.Fatal("scheduler did not query metric refresh candidates")
	}
	if collectionApp.refreshQuery.Limit != schedulerMetricRepairBatchSize {
		t.Fatalf("candidate limit = %d, want %d", collectionApp.refreshQuery.Limit, schedulerMetricRepairBatchSize)
	}
	wantBefore := startedAt.Add(-schedulerMetricRepairGracePeriod)
	if collectionApp.refreshQuery.UpdatedBefore.Before(wantBefore) || collectionApp.refreshQuery.UpdatedBefore.After(time.Now().UTC().Add(-schedulerMetricRepairGracePeriod)) {
		t.Fatalf("candidate cutoff = %v, want approximately %v", collectionApp.refreshQuery.UpdatedBefore, wantBefore)
	}
	_, collectionRunIDs, _ := enqueuer.snapshot()
	if fmt.Sprint(collectionRunIDs) != fmt.Sprint([]string{"run_1"}) {
		t.Fatalf("enqueued collection run IDs = %#v", collectionRunIDs)
	}
}

func TestRunMigrateUpRunsMigrations(t *testing.T) {
	originalRunMigrations := runMigrations
	defer func() { runMigrations = originalRunMigrations }()

	called := false
	runMigrations = func(_ context.Context, databaseURL string, direction string) error {
		called = true
		if databaseURL != "postgres://test" {
			t.Fatalf("databaseURL = %q, want postgres://test", databaseURL)
		}
		if direction != "up" {
			t.Fatalf("direction = %q, want up", direction)
		}
		return nil
	}

	err := Run(context.Background(), "migrate up", config.Config{
		DatabaseURL: "postgres://test",
	}, zerolog.New(io.Discard))
	if err != nil {
		t.Fatalf("Run(migrate up) error = %v", err)
	}
	if !called {
		t.Fatal("expected migrate runner to be called")
	}
}

func TestRunStartsAPIModeWithInjectedCapacityApplication(t *testing.T) {
	originalOpenRuntime := openRuntimeFunc
	originalListenAndServe := listenAndServe
	defer func() {
		openRuntimeFunc = originalOpenRuntime
		listenAndServe = originalListenAndServe
	}()

	service := &stubAppCapacityApplication{
		createRecord: appcapacity.CalculationRecord{
			ID: "calc_db",
			Result: domaincapacity.HousingCapacityResult{
				PressureLevel: domaincapacity.PressureStrained,
				Strategy:      "先卖后买或同步推进",
			},
		},
	}

	opened := false
	openRuntimeFunc = func(_ context.Context, cfg config.Config, _ zerolog.Logger) (*runtime, error) {
		opened = true
		if cfg.DatabaseURL != "postgres://test" {
			t.Fatalf("DatabaseURL = %q, want postgres://test", cfg.DatabaseURL)
		}
		return &runtime{
			capacity:     service,
			neighborhood: &stubAppNeighborhoodApplication{},
			collection:   &stubAppCollectionApplication{},
			decision:     &stubAppDecisionApplication{},
			queueClient:  noopCloser{},
		}, nil
	}

	listenAndServe = func(server *http.Server) error {
		body := `{"cashOnHand":150,"oldHomeValue":320,"oldLoanBalance":80,"monthlyIncome":3.5,"currentMonthlyMortgage":0,"acceptableMonthlyMortgage":1.5,"targetTotalPrice":550,"renovationBudget":40,"transactionCosts":18,"transitionRentCost":5}`
		req, err := http.NewRequest(http.MethodPost, "/api/v1/capacity/calculations", bytes.NewBufferString(body))
		if err != nil {
			t.Fatalf("http.NewRequest() error = %v", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer test-access-token")
		rec := newInMemoryHTTPResponseWriter()

		server.Handler.ServeHTTP(rec, req)

		if rec.statusCode != http.StatusCreated {
			t.Fatalf("status = %d, want 201", rec.statusCode)
		}
		if !service.createCalled {
			t.Fatal("expected injected capacity application to handle request")
		}

		return http.ErrServerClosed
	}

	err := Run(context.Background(), "api", config.Config{
		HTTPAddr:    "127.0.0.1:0",
		DatabaseURL: "postgres://test",
		AccessToken: "test-access-token",
	}, zerolog.New(io.Discard))
	if err != nil {
		t.Fatalf("Run(api) error = %v", err)
	}
	if !opened {
		t.Fatal("expected shared runtime to be opened")
	}
}

func TestRunStartsAPIModeWithInjectedNeighborhoodApplication(t *testing.T) {
	originalOpenRuntime := openRuntimeFunc
	originalListenAndServe := listenAndServe
	defer func() {
		openRuntimeFunc = originalOpenRuntime
		listenAndServe = originalListenAndServe
	}()

	service := &stubAppNeighborhoodApplication{
		watchlist: []appneighborhood.WatchlistItemSummary{
			{
				ID:                  "watch_1",
				NeighborhoodID:      "neighborhood_1",
				Name:                "青枫花园",
				Area:                "滨江核心",
				TargetLayout:        "三房",
				Status:              domainneighborhood.NeighborhoodStatusBargain,
				ListedHomes:         42,
				PriceCutHomes:       11,
				TransactionMomentum: domainneighborhood.TransactionMomentumWeak,
				Advice:              "重点看 495-545 万成交区间附近房源，对挂牌久、降价过的房源试探底价。",
			},
		},
	}

	openRuntimeFunc = func(_ context.Context, cfg config.Config, _ zerolog.Logger) (*runtime, error) {
		if cfg.DatabaseURL != "postgres://test" {
			t.Fatalf("DatabaseURL = %q, want postgres://test", cfg.DatabaseURL)
		}
		return &runtime{
			capacity:     &stubAppCapacityApplication{},
			neighborhood: service,
			collection:   &stubAppCollectionApplication{},
			decision:     &stubAppDecisionApplication{},
			queueClient:  noopCloser{},
		}, nil
	}

	listenAndServe = func(server *http.Server) error {
		req, err := http.NewRequest(http.MethodGet, "/api/v1/watchlist", nil)
		if err != nil {
			t.Fatalf("http.NewRequest() error = %v", err)
		}
		req.Header.Set("Authorization", "Bearer test-access-token")
		rec := newInMemoryHTTPResponseWriter()

		server.Handler.ServeHTTP(rec, req)

		if rec.statusCode != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.statusCode)
		}
		if !service.listCalled {
			t.Fatal("expected injected neighborhood application to handle request")
		}

		return http.ErrServerClosed
	}

	err := Run(context.Background(), "api", config.Config{
		HTTPAddr:    "127.0.0.1:0",
		DatabaseURL: "postgres://test",
		AccessToken: "test-access-token",
	}, zerolog.New(io.Discard))
	if err != nil {
		t.Fatalf("Run(api) error = %v", err)
	}
}

func TestRunStartsAPIModeWithInjectedCollectionApplication(t *testing.T) {
	originalOpenRuntime := openRuntimeFunc
	originalListenAndServe := listenAndServe
	defer func() {
		openRuntimeFunc = originalOpenRuntime
		listenAndServe = originalListenAndServe
	}()

	service := &stubAppCollectionApplication{
		result: appcollection.ImportCollectionRunResult{
			Run: appcollection.CollectionRun{
				ID: "33333333-3333-3333-3333-333333333333",
			},
			ListingCount: 1,
		},
	}

	openRuntimeFunc = func(_ context.Context, cfg config.Config, _ zerolog.Logger) (*runtime, error) {
		if cfg.DatabaseURL != "postgres://test" {
			t.Fatalf("DatabaseURL = %q, want postgres://test", cfg.DatabaseURL)
		}
		return &runtime{
			capacity:     &stubAppCapacityApplication{},
			neighborhood: &stubAppNeighborhoodApplication{},
			collection:   service,
			decision:     &stubAppDecisionApplication{},
			queueClient:  noopCloser{},
		}, nil
	}

	listenAndServe = func(server *http.Server) error {
		body := `{"dataSourceId":"11111111-1111-1111-1111-111111111111","neighborhoodId":"22222222-2222-2222-2222-222222222222","sourceRef":"demo-weekly-import","collectedAt":"2026-07-13T10:00:00Z","coverage":"full","records":[{"recordType":"listing","sourceRecordId":"listing-1","layout":"三房","areaSqm":89,"listingPrice":520,"daysOnMarket":0,"status":"active"}]}`
		req, err := http.NewRequest(http.MethodPost, "/admin/api/imports/json", bytes.NewBufferString(body))
		if err != nil {
			t.Fatalf("http.NewRequest() error = %v", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer test-access-token")
		rec := newInMemoryHTTPResponseWriter()

		server.Handler.ServeHTTP(rec, req)

		if rec.statusCode != http.StatusCreated {
			t.Fatalf("status = %d, want 201", rec.statusCode)
		}
		if !service.importCalled {
			t.Fatal("expected injected collection application to handle request")
		}

		return http.ErrServerClosed
	}

	err := Run(context.Background(), "api", config.Config{
		HTTPAddr:    "127.0.0.1:0",
		DatabaseURL: "postgres://test",
		AccessToken: "test-access-token",
	}, zerolog.New(io.Discard))
	if err != nil {
		t.Fatalf("Run(api) error = %v", err)
	}
}

func TestRunHTTPServerPassesRuntimeReadinessCheckerToRouter(t *testing.T) {
	originalListenAndServe := listenAndServe
	defer func() { listenAndServe = originalListenAndServe }()

	checker := &appReadinessStub{}
	rt := &runtime{
		capacity:     &stubAppCapacityApplication{},
		neighborhood: &stubAppNeighborhoodApplication{},
		collection:   &stubAppCollectionApplication{},
		decision:     &stubAppDecisionApplication{},
		readiness:    checker,
	}
	listenAndServe = func(server *http.Server) error {
		req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
		rec := httptest.NewRecorder()
		server.Handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			return fmt.Errorf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
		}
		if checker.calls != 1 {
			return fmt.Errorf("runtime readiness check calls = %d, want 1", checker.calls)
		}
		return http.ErrServerClosed
	}

	if err := runHTTPServer(context.Background(), config.Config{}, zerolog.New(io.Discard), rt); err != nil {
		t.Fatalf("runHTTPServer() error = %v", err)
	}
}

func testRunStartsHTTPServer(t *testing.T, mode string) {
	t.Helper()

	originalOpenRuntime := openRuntimeFunc
	originalStartQueueWorker := startQueueWorker
	originalStartScheduler := startScheduler
	defer func() {
		openRuntimeFunc = originalOpenRuntime
		startQueueWorker = originalStartQueueWorker
		startScheduler = originalStartScheduler
	}()
	openRuntimeFunc = func(_ context.Context, _ config.Config, _ zerolog.Logger) (*runtime, error) {
		return &runtime{
			capacity:     &stubAppCapacityApplication{},
			neighborhood: &stubAppNeighborhoodApplication{},
			collection:   &stubAppCollectionApplication{},
			decision:     &stubAppDecisionApplication{},
			queueClient:  noopCloser{},
		}, nil
	}
	startQueueWorker = func(ctx context.Context, _ config.Config, _ zerolog.Logger, _ *runtime) error {
		<-ctx.Done()
		return nil
	}
	startScheduler = func(ctx context.Context, _ config.Config, _ zerolog.Logger, _ *runtime) error {
		<-ctx.Done()
		return nil
	}

	addr := freeLocalAddr(t)
	cfg := config.Config{
		HTTPAddr: addr,
		Log: config.LogConfig{
			Level: "info",
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- Run(ctx, mode, cfg, zerolog.New(io.Discard))
	}()

	waitForHTTP(t, "http://"+addr+"/healthz")
	cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("Run(%q) error = %v", mode, err)
		}
	case <-time.After(3 * time.Second):
		t.Fatalf("Run(%q) did not stop after cancel", mode)
	}
}

type runModeCalls struct {
	http      int
	worker    int
	scheduler int
}

type runModeCallRecorder struct {
	mu    sync.Mutex
	calls runModeCalls
}

func (r *runModeCallRecorder) increment(component string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	switch component {
	case "http":
		r.calls.http++
	case "worker":
		r.calls.worker++
	case "scheduler":
		r.calls.scheduler++
	}
}

func (r *runModeCallRecorder) snapshot() runModeCalls {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.calls
}

func testRunModeComposition(t *testing.T, mode string) runModeCalls {
	t.Helper()

	originalOpenRuntime := openRuntimeFunc
	originalRunHTTPServer := runHTTPServerFunc
	originalStartQueueWorker := startQueueWorker
	originalStartScheduler := startScheduler
	defer func() {
		openRuntimeFunc = originalOpenRuntime
		runHTTPServerFunc = originalRunHTTPServer
		startQueueWorker = originalStartQueueWorker
		startScheduler = originalStartScheduler
	}()

	openRuntimeFunc = func(_ context.Context, _ config.Config, _ zerolog.Logger) (*runtime, error) {
		return &runtime{queueClient: noopCloser{}}, nil
	}
	calls := &runModeCallRecorder{}
	runHTTPServerFunc = func(ctx context.Context, _ config.Config, _ zerolog.Logger, _ *runtime) error {
		calls.increment("http")
		<-ctx.Done()
		return nil
	}
	startQueueWorker = func(ctx context.Context, _ config.Config, _ zerolog.Logger, _ *runtime) error {
		calls.increment("worker")
		<-ctx.Done()
		return nil
	}
	startScheduler = func(ctx context.Context, _ config.Config, _ zerolog.Logger, _ *runtime) error {
		calls.increment("scheduler")
		<-ctx.Done()
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		errCh <- Run(ctx, mode, config.Config{SchedulerInterval: time.Hour}, zerolog.New(io.Discard))
	}()

	deadline := time.After(2 * time.Second)
	for !modeStarted(mode, calls.snapshot()) {
		select {
		case <-deadline:
			t.Fatalf("Run(%q) did not start expected components; calls = %+v", mode, calls.snapshot())
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
	cancel()
	if err := <-errCh; err != nil {
		t.Fatalf("Run(%q) error = %v", mode, err)
	}

	return calls.snapshot()
}

func modeStarted(mode string, calls runModeCalls) bool {
	switch mode {
	case "api":
		return calls.http == 1
	case "serve":
		return calls.http == 1 && calls.worker == 1 && calls.scheduler == 1
	case "worker":
		return calls.worker == 1
	case "scheduler":
		return calls.scheduler == 1
	default:
		return false
	}
}

func freeLocalAddr(t *testing.T) string {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen error = %v", err)
	}
	defer func() {
		if err := ln.Close(); err != nil {
			t.Errorf("listener Close() error = %v", err)
		}
	}()

	return ln.Addr().String()
}

func waitForHTTP(t *testing.T, url string) {
	t.Helper()

	client := &http.Client{Timeout: 200 * time.Millisecond}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := client.Get(url)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(50 * time.Millisecond)
	}

	t.Fatalf("server at %s did not become healthy before timeout", url)
}

func TestRunRejectsUnknownMode(t *testing.T) {
	err := Run(context.Background(), "unknown", config.Config{}, zerolog.New(io.Discard))
	if err == nil {
		t.Fatal("Run returned nil for unknown mode")
	}
	if got, want := err.Error(), Usage; got != want {
		t.Fatalf("Run error = %q, want %q", got, want)
	}
}

func TestUsageStringMatchesDocumentedCLI(t *testing.T) {
	want := "usage: propulse [serve|api|worker|scheduler|migrate up|migrate down]"
	if Usage != want {
		t.Fatalf("Usage = %q, want %q", Usage, want)
	}
}

func ExampleNormalizeMode() {
	mode, _ := NormalizeMode([]string{"api"})
	fmt.Println(mode)
	// Output: api
}

type stubAppCapacityApplication struct {
	createCalled  bool
	createRecord  appcapacity.CalculationRecord
	createStarted chan<- struct{}
	releaseCreate <-chan struct{}
}

func (s *stubAppCapacityApplication) CreateCalculation(_ context.Context, _ appcapacity.CreateCalculationCommand) (appcapacity.CalculationRecord, error) {
	if s.createStarted != nil {
		s.createStarted <- struct{}{}
	}
	if s.releaseCreate != nil {
		<-s.releaseCreate
	}
	s.createCalled = true
	return s.createRecord, nil
}

func (s *stubAppCapacityApplication) GetAssumptions(_ context.Context, _ appcapacity.GetAssumptionsQuery) (domaincapacity.Assumptions, error) {
	return domaincapacity.Assumptions{}, nil
}

func (s *stubAppCapacityApplication) GetCalculation(_ context.Context, _ appcapacity.GetCalculationQuery) (appcapacity.CalculationRecord, error) {
	return appcapacity.CalculationRecord{}, appcapacity.ErrCalculationNotFound
}

func (s *stubAppCapacityApplication) LatestCalculation(_ context.Context, _ appcapacity.LatestCalculationQuery) (appcapacity.CalculationRecord, error) {
	return appcapacity.CalculationRecord{}, appcapacity.ErrCalculationNotFound
}

type stubAppNeighborhoodApplication struct {
	listCalled bool
	watchlist  []appneighborhood.WatchlistItemSummary
}

func (s *stubAppNeighborhoodApplication) CreateNeighborhood(_ context.Context, _ appneighborhood.CreateNeighborhoodCommand) (appneighborhood.Neighborhood, error) {
	return appneighborhood.Neighborhood{}, nil
}

func (s *stubAppNeighborhoodApplication) GetNeighborhood(_ context.Context, _ appneighborhood.GetNeighborhoodQuery) (appneighborhood.Neighborhood, error) {
	return appneighborhood.Neighborhood{}, appneighborhood.ErrNeighborhoodNotFound
}

func (s *stubAppNeighborhoodApplication) SearchNeighborhoods(_ context.Context, _ appneighborhood.SearchNeighborhoodsQuery) (appneighborhood.SearchNeighborhoodsPage, error) {
	return appneighborhood.SearchNeighborhoodsPage{}, nil
}

func (s *stubAppNeighborhoodApplication) LatestMetric(_ context.Context, _ appneighborhood.LatestMetricQuery) (appneighborhood.MetricWithSignal, error) {
	return appneighborhood.MetricWithSignal{}, appneighborhood.ErrMetricNotFound
}

func (s *stubAppNeighborhoodApplication) MetricHistory(_ context.Context, _ appneighborhood.MetricHistoryQuery) (appneighborhood.MetricHistoryResult, error) {
	return appneighborhood.MetricHistoryResult{Items: []appneighborhood.MetricHistoryPoint{}}, nil
}

func (s *stubAppNeighborhoodApplication) AddWatchlistItem(_ context.Context, _ appneighborhood.AddWatchlistItemCommand) (appneighborhood.WatchlistItem, error) {
	return appneighborhood.WatchlistItem{}, nil
}

func (s *stubAppNeighborhoodApplication) ListWatchlist(_ context.Context, _ appneighborhood.ListWatchlistQuery) ([]appneighborhood.WatchlistItemSummary, error) {
	s.listCalled = true
	return s.watchlist, nil
}

type stubAppCollectionApplication struct {
	importCalled      bool
	result            appcollection.ImportCollectionRunResult
	refreshCalled     bool
	refreshQuery      appcollection.ListMetricRefreshCandidatesQuery
	refreshCandidates []appcollection.MetricRefreshCandidate
}

type stubMetricTaskEnqueuer struct {
	mu               sync.Mutex
	neighborhoodIDs  []string
	collectionRunIDs []string
	sourceIDs        []string
}

func (s *stubMetricTaskEnqueuer) EnqueueMetricCalculateNeighborhood(_ context.Context, neighborhoodID string, collectionRunID string, sourceID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.neighborhoodIDs = append(s.neighborhoodIDs, neighborhoodID)
	s.collectionRunIDs = append(s.collectionRunIDs, collectionRunID)
	s.sourceIDs = append(s.sourceIDs, sourceID)
	return nil
}

func (s *stubMetricTaskEnqueuer) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.neighborhoodIDs)
}

func (s *stubMetricTaskEnqueuer) snapshot() ([]string, []string, []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.neighborhoodIDs...), append([]string(nil), s.collectionRunIDs...), append([]string(nil), s.sourceIDs...)
}

func (s *stubAppCollectionApplication) CreateDataSource(_ context.Context, command appcollection.CreateDataSourceCommand) (appcollection.DataSource, error) {
	return appcollection.DataSource{Name: command.Name, SourceType: command.SourceType, City: command.City}, nil
}

func (s *stubAppCollectionApplication) ListDataSources(context.Context, appcollection.ListDataSourcesQuery) ([]appcollection.DataSource, error) {
	return []appcollection.DataSource{}, nil
}

func (s *stubAppCollectionApplication) ImportCollectionRun(_ context.Context, _ appcollection.ImportCollectionRunCommand) (appcollection.ImportCollectionRunResult, error) {
	s.importCalled = true
	return s.result, nil
}

func (s *stubAppCollectionApplication) GetCollectionRun(context.Context, appcollection.GetCollectionRunQuery) (appcollection.CollectionRunDetail, error) {
	return appcollection.CollectionRunDetail{}, nil
}

func (s *stubAppCollectionApplication) ListMetricRefreshCandidates(_ context.Context, query appcollection.ListMetricRefreshCandidatesQuery) ([]appcollection.MetricRefreshCandidate, error) {
	s.refreshCalled = true
	s.refreshQuery = query
	return s.refreshCandidates, nil
}

type stubAppDecisionApplication struct{}

func (*stubAppDecisionApplication) GetActionWindow(context.Context, appdecision.GetActionWindowQuery) (domaindecision.ActionWindowResult, error) {
	return domaindecision.ActionWindowResult{}, nil
}

type noopCloser struct{}

func (noopCloser) Close() error { return nil }

type appReadinessStub struct {
	calls int
}

func (s *appReadinessStub) Check(context.Context) error {
	s.calls++
	return nil
}

type countingCloser struct {
	count int
}

func (c *countingCloser) Close() error {
	c.count++
	return nil
}

type closeSignalCloser struct {
	closed chan<- struct{}
}

func (c closeSignalCloser) Close() error {
	close(c.closed)
	return nil
}

type inMemoryHTTPResponseWriter struct {
	header     http.Header
	statusCode int
}

func newInMemoryHTTPResponseWriter() *inMemoryHTTPResponseWriter {
	return &inMemoryHTTPResponseWriter{
		header: http.Header{},
	}
}

func (w *inMemoryHTTPResponseWriter) Header() http.Header {
	return w.header
}

func (w *inMemoryHTTPResponseWriter) Write(data []byte) (int, error) {
	if w.statusCode == 0 {
		w.statusCode = http.StatusOK
	}
	return len(data), nil
}

func (w *inMemoryHTTPResponseWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
}
