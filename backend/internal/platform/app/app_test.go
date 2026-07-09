package app

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"

	appcapacity "github.com/propulse/propulse/backend/internal/application/capacity"
	appcollection "github.com/propulse/propulse/backend/internal/application/collection"
	appneighborhood "github.com/propulse/propulse/backend/internal/application/neighborhood"
	domaincapacity "github.com/propulse/propulse/backend/internal/domain/capacity"
	domainneighborhood "github.com/propulse/propulse/backend/internal/domain/neighborhood"
	"github.com/propulse/propulse/backend/internal/infrastructure/config"
	"github.com/rs/zerolog"
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

func TestRunStartsHTTPServerForAPIMode(t *testing.T) {
	testRunStartsHTTPServer(t, "api")
}

func TestRunStartsHTTPServerForServeMode(t *testing.T) {
	testRunStartsHTTPServer(t, "serve")
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

func TestRunSchedulerEnqueuesMetricJobsForWatchlist(t *testing.T) {
	originalOpenNeighborhoodApplication := openNeighborhoodApplication
	originalOpenMetricQueueClient := openMetricQueueClient
	defer func() {
		openNeighborhoodApplication = originalOpenNeighborhoodApplication
		openMetricQueueClient = originalOpenMetricQueueClient
	}()

	neighborhoodApp := &stubAppNeighborhoodApplication{
		watchlistNeighborhoodIDs: []string{"neighborhood_1", "neighborhood_2"},
	}
	enqueuer := &stubMetricTaskEnqueuer{}
	openNeighborhoodApplication = func(_ context.Context, _ config.Config, _ zerolog.Logger) (NeighborhoodApplication, io.Closer, error) {
		return neighborhoodApp, noopCloser{}, nil
	}
	openMetricQueueClient = func(_ config.Config) (MetricTaskEnqueuer, io.Closer, error) {
		return enqueuer, noopCloser{}, nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		errCh <- runScheduler(ctx, config.Config{SchedulerInterval: time.Hour}, zerolog.New(io.Discard))
	}()

	deadline := time.After(2 * time.Second)
	for enqueuer.count() < 2 {
		select {
		case <-deadline:
			neighborhoodIDs, _ := enqueuer.snapshot()
			t.Fatalf("scheduler did not enqueue watchlist jobs; got %#v", neighborhoodIDs)
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
	cancel()
	if err := <-errCh; err != nil {
		t.Fatalf("runScheduler() error = %v", err)
	}

	want := []string{"neighborhood_1", "neighborhood_2"}
	neighborhoodIDs, sourceIDs := enqueuer.snapshot()
	if fmt.Sprint(neighborhoodIDs) != fmt.Sprint(want) {
		t.Fatalf("enqueued neighborhood IDs = %#v, want %#v", neighborhoodIDs, want)
	}
	for _, sourceID := range sourceIDs {
		if sourceID != schedulerSourceID {
			t.Fatalf("sourceID = %q, want %q", sourceID, schedulerSourceID)
		}
	}
}

func TestRunSchedulerUsesDistinctWatchlistNeighborhoodIDs(t *testing.T) {
	neighborhoodApp := &stubAppNeighborhoodApplication{
		watchlistNeighborhoodIDs: []string{"neighborhood_1", "neighborhood_2"},
	}
	enqueuer := &stubMetricTaskEnqueuer{}

	if err := enqueueWatchlistMetricJobs(context.Background(), neighborhoodApp, enqueuer, zerolog.New(io.Discard)); err != nil {
		t.Fatalf("enqueueWatchlistMetricJobs() error = %v", err)
	}

	if neighborhoodApp.listCalled {
		t.Fatal("scheduler used user-scoped ListWatchlist instead of distinct watchlist neighborhood IDs")
	}
	if !neighborhoodApp.listNeighborhoodIDsCalled {
		t.Fatal("scheduler did not list distinct watchlist neighborhood IDs")
	}

	want := []string{"neighborhood_1", "neighborhood_2"}
	neighborhoodIDs, _ := enqueuer.snapshot()
	if fmt.Sprint(neighborhoodIDs) != fmt.Sprint(want) {
		t.Fatalf("enqueued neighborhood IDs = %#v, want %#v", neighborhoodIDs, want)
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
	originalOpenCapacityApplication := openCapacityApplication
	originalOpenNeighborhoodApplication := openNeighborhoodApplication
	originalOpenCollectionApplication := openCollectionApplication
	originalListenAndServe := listenAndServe
	defer func() {
		openCapacityApplication = originalOpenCapacityApplication
		openNeighborhoodApplication = originalOpenNeighborhoodApplication
		openCollectionApplication = originalOpenCollectionApplication
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
	openCapacityApplication = func(_ context.Context, cfg config.Config, _ zerolog.Logger) (CapacityApplication, io.Closer, error) {
		opened = true
		if cfg.DatabaseURL != "postgres://test" {
			t.Fatalf("DatabaseURL = %q, want postgres://test", cfg.DatabaseURL)
		}
		return service, noopCloser{}, nil
	}
	openNeighborhoodApplication = func(_ context.Context, _ config.Config, _ zerolog.Logger) (NeighborhoodApplication, io.Closer, error) {
		return &stubAppNeighborhoodApplication{}, noopCloser{}, nil
	}
	openCollectionApplication = func(_ context.Context, _ config.Config, _ zerolog.Logger) (CollectionApplication, io.Closer, error) {
		return &stubAppCollectionApplication{}, noopCloser{}, nil
	}

	listenAndServe = func(server *http.Server) error {
		body := `{"cashOnHand":150,"oldHomeValue":320,"oldLoanBalance":80,"monthlyIncome":3.5,"currentMonthlyMortgage":0,"acceptableMonthlyMortgage":1.5,"targetTotalPrice":550,"renovationBudget":40,"transactionCosts":18,"transitionRentCost":5}`
		req, err := http.NewRequest(http.MethodPost, "/api/v1/capacity/calculations", bytes.NewBufferString(body))
		if err != nil {
			t.Fatalf("http.NewRequest() error = %v", err)
		}
		req.Header.Set("Content-Type", "application/json")
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
	}, zerolog.New(io.Discard))
	if err != nil {
		t.Fatalf("Run(api) error = %v", err)
	}
	if !opened {
		t.Fatal("expected postgres capacity application to be opened")
	}
}

func TestRunStartsAPIModeWithInjectedNeighborhoodApplication(t *testing.T) {
	originalOpenCapacityApplication := openCapacityApplication
	originalOpenNeighborhoodApplication := openNeighborhoodApplication
	originalOpenCollectionApplication := openCollectionApplication
	originalListenAndServe := listenAndServe
	defer func() {
		openCapacityApplication = originalOpenCapacityApplication
		openNeighborhoodApplication = originalOpenNeighborhoodApplication
		openCollectionApplication = originalOpenCollectionApplication
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

	openCapacityApplication = func(_ context.Context, _ config.Config, _ zerolog.Logger) (CapacityApplication, io.Closer, error) {
		return &stubAppCapacityApplication{}, noopCloser{}, nil
	}
	openNeighborhoodApplication = func(_ context.Context, cfg config.Config, _ zerolog.Logger) (NeighborhoodApplication, io.Closer, error) {
		if cfg.DatabaseURL != "postgres://test" {
			t.Fatalf("DatabaseURL = %q, want postgres://test", cfg.DatabaseURL)
		}
		return service, noopCloser{}, nil
	}
	openCollectionApplication = func(_ context.Context, _ config.Config, _ zerolog.Logger) (CollectionApplication, io.Closer, error) {
		return &stubAppCollectionApplication{}, noopCloser{}, nil
	}

	listenAndServe = func(server *http.Server) error {
		req, err := http.NewRequest(http.MethodGet, "/api/v1/watchlist", nil)
		if err != nil {
			t.Fatalf("http.NewRequest() error = %v", err)
		}
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
	}, zerolog.New(io.Discard))
	if err != nil {
		t.Fatalf("Run(api) error = %v", err)
	}
}

func TestRunStartsAPIModeWithInjectedCollectionApplication(t *testing.T) {
	originalOpenCapacityApplication := openCapacityApplication
	originalOpenNeighborhoodApplication := openNeighborhoodApplication
	originalOpenCollectionApplication := openCollectionApplication
	originalListenAndServe := listenAndServe
	defer func() {
		openCapacityApplication = originalOpenCapacityApplication
		openNeighborhoodApplication = originalOpenNeighborhoodApplication
		openCollectionApplication = originalOpenCollectionApplication
		listenAndServe = originalListenAndServe
	}()

	service := &stubAppCollectionApplication{
		result: appcollection.ImportManualListingsResult{
			CollectionRunID:       "collection_run_1",
			ImportedSnapshotCount: 1,
		},
	}

	openCapacityApplication = func(_ context.Context, _ config.Config, _ zerolog.Logger) (CapacityApplication, io.Closer, error) {
		return &stubAppCapacityApplication{}, noopCloser{}, nil
	}
	openNeighborhoodApplication = func(_ context.Context, _ config.Config, _ zerolog.Logger) (NeighborhoodApplication, io.Closer, error) {
		return &stubAppNeighborhoodApplication{}, noopCloser{}, nil
	}
	openCollectionApplication = func(_ context.Context, cfg config.Config, _ zerolog.Logger) (CollectionApplication, io.Closer, error) {
		if cfg.DatabaseURL != "postgres://test" {
			t.Fatalf("DatabaseURL = %q, want postgres://test", cfg.DatabaseURL)
		}
		return service, noopCloser{}, nil
	}

	listenAndServe = func(server *http.Server) error {
		body := `{"sourceType":"manual_json","sourceRef":"demo-weekly-import","neighborhoodId":"neighborhood_1","records":[{"listingPrice":520,"daysOnMarket":0}]}`
		req, err := http.NewRequest(http.MethodPost, "/admin/api/imports", bytes.NewBufferString(body))
		if err != nil {
			t.Fatalf("http.NewRequest() error = %v", err)
		}
		req.Header.Set("Content-Type", "application/json")
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
	}, zerolog.New(io.Discard))
	if err != nil {
		t.Fatalf("Run(api) error = %v", err)
	}
}

func testRunStartsHTTPServer(t *testing.T, mode string) {
	t.Helper()

	originalOpenCapacityApplication := openCapacityApplication
	originalOpenNeighborhoodApplication := openNeighborhoodApplication
	originalOpenCollectionApplication := openCollectionApplication
	originalStartQueueWorker := startQueueWorker
	originalStartScheduler := startScheduler
	defer func() {
		openCapacityApplication = originalOpenCapacityApplication
		openNeighborhoodApplication = originalOpenNeighborhoodApplication
		openCollectionApplication = originalOpenCollectionApplication
		startQueueWorker = originalStartQueueWorker
		startScheduler = originalStartScheduler
	}()
	openCapacityApplication = func(_ context.Context, _ config.Config, _ zerolog.Logger) (CapacityApplication, io.Closer, error) {
		return &stubAppCapacityApplication{}, noopCloser{}, nil
	}
	openNeighborhoodApplication = func(_ context.Context, _ config.Config, _ zerolog.Logger) (NeighborhoodApplication, io.Closer, error) {
		return &stubAppNeighborhoodApplication{}, noopCloser{}, nil
	}
	openCollectionApplication = func(_ context.Context, _ config.Config, _ zerolog.Logger) (CollectionApplication, io.Closer, error) {
		return &stubAppCollectionApplication{}, noopCloser{}, nil
	}
	startQueueWorker = func(ctx context.Context, _ config.Config, _ zerolog.Logger) error {
		<-ctx.Done()
		return nil
	}
	startScheduler = func(ctx context.Context, _ config.Config, _ zerolog.Logger) error {
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

func testRunModeComposition(t *testing.T, mode string) runModeCalls {
	t.Helper()

	originalRunHTTPServer := runHTTPServerFunc
	originalStartQueueWorker := startQueueWorker
	originalStartScheduler := startScheduler
	defer func() {
		runHTTPServerFunc = originalRunHTTPServer
		startQueueWorker = originalStartQueueWorker
		startScheduler = originalStartScheduler
	}()

	calls := runModeCalls{}
	runHTTPServerFunc = func(ctx context.Context, _ config.Config, _ zerolog.Logger) error {
		calls.http++
		<-ctx.Done()
		return nil
	}
	startQueueWorker = func(ctx context.Context, _ config.Config, _ zerolog.Logger) error {
		calls.worker++
		<-ctx.Done()
		return nil
	}
	startScheduler = func(ctx context.Context, _ config.Config, _ zerolog.Logger) error {
		calls.scheduler++
		<-ctx.Done()
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		errCh <- Run(ctx, mode, config.Config{SchedulerInterval: time.Hour}, zerolog.New(io.Discard))
	}()

	deadline := time.After(2 * time.Second)
	for {
		if modeStarted(mode, calls) {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("Run(%q) did not start expected components; calls = %+v", mode, calls)
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
	cancel()
	if err := <-errCh; err != nil {
		t.Fatalf("Run(%q) error = %v", mode, err)
	}

	return calls
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
	defer ln.Close()

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
	createCalled bool
	createRecord appcapacity.CalculationRecord
}

func (s *stubAppCapacityApplication) CreateCalculation(_ context.Context, _ appcapacity.CreateCalculationCommand) (appcapacity.CalculationRecord, error) {
	s.createCalled = true
	return s.createRecord, nil
}

func (s *stubAppCapacityApplication) GetCalculation(_ context.Context, _ appcapacity.GetCalculationQuery) (appcapacity.CalculationRecord, error) {
	return appcapacity.CalculationRecord{}, appcapacity.ErrCalculationNotFound
}

type stubAppNeighborhoodApplication struct {
	listCalled                bool
	listNeighborhoodIDsCalled bool
	watchlist                 []appneighborhood.WatchlistItemSummary
	watchlistNeighborhoodIDs  []string
}

func (s *stubAppNeighborhoodApplication) CreateNeighborhood(_ context.Context, _ appneighborhood.CreateNeighborhoodCommand) (appneighborhood.Neighborhood, error) {
	return appneighborhood.Neighborhood{}, nil
}

func (s *stubAppNeighborhoodApplication) GetNeighborhood(_ context.Context, _ appneighborhood.GetNeighborhoodQuery) (appneighborhood.Neighborhood, error) {
	return appneighborhood.Neighborhood{}, appneighborhood.ErrNeighborhoodNotFound
}

func (s *stubAppNeighborhoodApplication) LatestMetric(_ context.Context, _ appneighborhood.LatestMetricQuery) (appneighborhood.MetricWithSignal, error) {
	return appneighborhood.MetricWithSignal{}, appneighborhood.ErrMetricNotFound
}

func (s *stubAppNeighborhoodApplication) AddWatchlistItem(_ context.Context, _ appneighborhood.AddWatchlistItemCommand) (appneighborhood.WatchlistItem, error) {
	return appneighborhood.WatchlistItem{}, nil
}

func (s *stubAppNeighborhoodApplication) ListWatchlist(_ context.Context, _ appneighborhood.ListWatchlistQuery) ([]appneighborhood.WatchlistItemSummary, error) {
	s.listCalled = true
	return s.watchlist, nil
}

func (s *stubAppNeighborhoodApplication) ListWatchlistNeighborhoodIDs(_ context.Context, _ appneighborhood.ListWatchlistNeighborhoodIDsQuery) ([]string, error) {
	s.listNeighborhoodIDsCalled = true
	return s.watchlistNeighborhoodIDs, nil
}

type stubAppCollectionApplication struct {
	importCalled bool
	result       appcollection.ImportManualListingsResult
}

type stubMetricTaskEnqueuer struct {
	mu              sync.Mutex
	neighborhoodIDs []string
	sourceIDs       []string
}

func (s *stubMetricTaskEnqueuer) EnqueueMetricCalculateNeighborhood(_ context.Context, neighborhoodID string, sourceID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.neighborhoodIDs = append(s.neighborhoodIDs, neighborhoodID)
	s.sourceIDs = append(s.sourceIDs, sourceID)
	return nil
}

func (s *stubMetricTaskEnqueuer) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.neighborhoodIDs)
}

func (s *stubMetricTaskEnqueuer) snapshot() ([]string, []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.neighborhoodIDs...), append([]string(nil), s.sourceIDs...)
}

func (s *stubAppCollectionApplication) ImportManualListings(_ context.Context, _ appcollection.ImportManualListingsCommand) (appcollection.ImportManualListingsResult, error) {
	s.importCalled = true
	return s.result, nil
}

type noopCloser struct{}

func (noopCloser) Close() error { return nil }

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
