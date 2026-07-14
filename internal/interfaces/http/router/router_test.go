package router

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
	webembed "github.com/sine-io/propulse/apps/web/embed"
	appcapacity "github.com/sine-io/propulse/internal/application/capacity"
	appcollection "github.com/sine-io/propulse/internal/application/collection"
	appdecision "github.com/sine-io/propulse/internal/application/decision"
	appneighborhood "github.com/sine-io/propulse/internal/application/neighborhood"
	"github.com/sine-io/propulse/internal/application/user"
	domaindecision "github.com/sine-io/propulse/internal/domain/decision"
	domainneighborhood "github.com/sine-io/propulse/internal/domain/neighborhood"
)

func newTestEngine(t *testing.T, deps Dependencies) http.Handler {
	t.Helper()

	marketState := newInMemoryMarketState()
	neighborhoodRepo := newInMemoryNeighborhoodRepository(marketState)
	if deps.CapacityApplication == nil {
		deps.CapacityApplication = appcapacity.NewService(newInMemoryCalculationRepository(), nil, nil)
	}
	if deps.NeighborhoodApplication == nil {
		deps.NeighborhoodApplication = appneighborhood.NewService(neighborhoodRepo)
	}
	if deps.CollectionApplication == nil {
		deps.CollectionApplication = appcollection.NewService(newInMemoryCollectionRepository(neighborhoodRepo, marketState), nil, nil)
	}
	if deps.DecisionApplication == nil {
		deps.DecisionApplication = appdecision.NewService(deps.CapacityApplication, deps.NeighborhoodApplication, user.SingleUserID)
	}
	if deps.UserID == "" {
		deps.UserID = user.SingleUserID
	}

	engine, err := New(deps)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return engine
}

func TestNewRejectsMissingApplicationDependencies(t *testing.T) {
	_, err := New(Dependencies{})
	if err == nil {
		t.Fatal("New() error = nil, want missing dependency error")
	}
	for _, name := range []string{"CapacityApplication", "NeighborhoodApplication", "CollectionApplication", "DecisionApplication"} {
		if !strings.Contains(err.Error(), name) {
			t.Fatalf("New() error = %q, want dependency %q", err, name)
		}
	}
}

func TestInMemoryWatchlistListsItemsByInsertionOrder(t *testing.T) {
	repo := newInMemoryNeighborhoodRepository()
	ctx := context.Background()

	for _, input := range []appneighborhood.CreateNeighborhoodInput{
		{ID: "neighborhood_1", Name: "青枫花园", Area: "滨江核心", TargetLayout: "三房"},
		{ID: "neighborhood_2", Name: "云栖苑", Area: "未来科技城", TargetLayout: "三房"},
		{ID: "neighborhood_3", Name: "晓风印月", Area: "奥体", TargetLayout: "四房"},
	} {
		if _, err := repo.CreateNeighborhood(ctx, input); err != nil {
			t.Fatalf("CreateNeighborhood(%q) error = %v", input.ID, err)
		}
	}

	for _, neighborhoodID := range []string{"neighborhood_2", "neighborhood_1", "neighborhood_3"} {
		if _, err := repo.AddWatchlistItem(ctx, user.SingleUserID, neighborhoodID); err != nil {
			t.Fatalf("AddWatchlistItem(%q) error = %v", neighborhoodID, err)
		}
	}

	for range 100 {
		items, err := repo.ListWatchlist(ctx, user.SingleUserID)
		if err != nil {
			t.Fatalf("ListWatchlist() error = %v", err)
		}
		got := []string{}
		for _, item := range items {
			got = append(got, item.NeighborhoodID)
		}
		want := []string{"neighborhood_2", "neighborhood_1", "neighborhood_3"}
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Fatalf("watchlist order = %#v, want %#v", got, want)
		}
	}
}

func TestHealthAndReadyRoutes(t *testing.T) {
	engine := newTestEngine(t, Dependencies{
		Log:      zerolog.New(io.Discard),
		StaticFS: webembed.Embedded(),
	})

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("/healthz status = %d, want 200", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec = httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("/readyz status = %d, want 503", rec.Code)
	}
}

func TestReadyRouteReturnsOKWhenDependenciesAreReady(t *testing.T) {
	checker := &readinessStub{}
	engine := newReadinessTestEngine(t, checker)

	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if got, want := rec.Body.String(), "{\"status\":\"ready\"}"; got != want {
		t.Fatalf("body = %q, want %q", got, want)
	}
	if checker.calls != 1 {
		t.Fatalf("readiness check calls = %d, want 1", checker.calls)
	}
}

func TestReadyRouteReturnsServiceUnavailableWhenDependencyFails(t *testing.T) {
	dependencyErr := errors.New("database credentials leaked")
	engine := newReadinessTestEngine(t, &readinessStub{err: dependencyErr})

	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	assertNotReadyResponse(t, rec)
	if strings.Contains(rec.Body.String(), dependencyErr.Error()) {
		t.Fatalf("response leaked readiness error: %s", rec.Body.String())
	}
}

func TestReadyRouteFailsClosedWithoutChecker(t *testing.T) {
	engine := newReadinessTestEngine(t, nil)

	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	assertNotReadyResponse(t, rec)
}

func TestHealthRouteRemainsOKWhenReadinessFails(t *testing.T) {
	checker := &readinessStub{err: errors.New("redis unavailable")}
	engine := newReadinessTestEngine(t, checker)

	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if checker.calls != 0 {
		t.Fatalf("readiness check calls = %d, want 0", checker.calls)
	}
}

func TestReadyRouteChecksDependenciesWithTwoSecondDeadline(t *testing.T) {
	requestCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	checker := &readinessStub{checkContext: func(ctx context.Context) {
		deadline, ok := ctx.Deadline()
		if !ok {
			t.Fatal("readiness context has no deadline")
		}
		remaining := time.Until(deadline)
		if remaining < 1900*time.Millisecond || remaining > 2*time.Second {
			t.Fatalf("readiness deadline remaining = %v, want approximately 2s", remaining)
		}
		if context.Cause(ctx) != nil {
			t.Fatalf("readiness context unexpectedly canceled: %v", context.Cause(ctx))
		}
		cancel()
		if !errors.Is(context.Cause(ctx), context.Canceled) {
			t.Fatalf("readiness context cause after request cancellation = %v, want canceled", context.Cause(ctx))
		}
	}}
	engine := newReadinessTestEngine(t, checker)

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(requestCtx, http.MethodGet, "/readyz", nil)
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
}

func newReadinessTestEngine(t *testing.T, checker ReadinessChecker) http.Handler {
	t.Helper()
	return newTestEngine(t, Dependencies{
		Log:              zerolog.New(io.Discard),
		StaticFS:         webembed.Embedded(),
		ReadinessChecker: checker,
	})
}

func assertNotReadyResponse(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503; body=%s", rec.Code, rec.Body.String())
	}
	want := `{"error":{"code":"not_ready","message":"service dependencies are not ready"}}`
	if got := rec.Body.String(); got != want {
		t.Fatalf("body = %q, want exactly %q", got, want)
	}
}

type readinessStub struct {
	err          error
	calls        int
	checkContext func(context.Context)
}

func (s *readinessStub) Check(ctx context.Context) error {
	s.calls++
	if s.checkContext != nil {
		s.checkContext(ctx)
	}
	return s.err
}

func TestAPI404DoesNotReturnFrontend(t *testing.T) {
	engine := newTestEngine(t, Dependencies{
		Log:      zerolog.New(io.Discard),
		StaticFS: webembed.Embedded(),
	})

	for _, path := range []string{"/api/v1/missing", "/admin/api/missing"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		engine.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("%s status = %d, want 404", path, rec.Code)
		}
		if got := rec.Header().Get("Content-Type"); !strings.Contains(got, "application/json") {
			t.Fatalf("%s content-type = %q, want application/json", path, got)
		}
		if strings.Contains(rec.Body.String(), "<!DOCTYPE html>") {
			t.Fatalf("%s unexpectedly returned frontend html", path)
		}
	}
}

func TestFrontendRoutesServeEmbeddedHTML(t *testing.T) {
	engine := newTestEngine(t, Dependencies{
		Log:      zerolog.New(io.Discard),
		StaticFS: webembed.Embedded(),
	})

	for _, path := range []string{"/", "/calculator", "/watchlist", "/action-window", "/neighborhoods", "/methods", "/templates"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		engine.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("%s status = %d, want 200", path, rec.Code)
		}
		if got := rec.Header().Get("Content-Type"); !strings.Contains(got, "text/html") {
			t.Fatalf("%s content-type = %q, want text/html", path, got)
		}
		if !strings.Contains(rec.Body.String(), "<!DOCTYPE html>") {
			t.Fatalf("%s did not return embedded frontend html", path)
		}
	}
}

func TestRequestIDMiddlewareEchoesInboundHeaderAndLogsRequest(t *testing.T) {
	var logBuf bytes.Buffer
	engine := newTestEngine(t, Dependencies{
		Log:      zerolog.New(&logBuf),
		StaticFS: webembed.Embedded(),
	})

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set("X-Request-Id", "req-123")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if got := rec.Header().Get("X-Request-Id"); got != "req-123" {
		t.Fatalf("response request id = %q, want req-123", got)
	}

	var entry map[string]any
	if err := json.Unmarshal(logBuf.Bytes(), &entry); err != nil {
		t.Fatalf("json.Unmarshal(log) error = %v; raw=%q", err, logBuf.String())
	}
	if got := entry["request_id"]; got != "req-123" {
		t.Fatalf("logged request_id = %v, want req-123", got)
	}
	if got := entry["method"]; got != http.MethodGet {
		t.Fatalf("logged method = %v, want GET", got)
	}
	if got := entry["path"]; got != "/healthz" {
		t.Fatalf("logged path = %v, want /healthz", got)
	}
	if got := entry["status"]; got != float64(http.StatusOK) {
		t.Fatalf("logged status = %v, want 200", got)
	}
	if _, ok := entry["latency_ms"]; !ok {
		t.Fatalf("latency_ms missing from log entry: %v", entry)
	}
}

func TestRequestIDMiddlewareGeneratesHeaderWhenMissing(t *testing.T) {
	engine := newTestEngine(t, Dependencies{
		Log:      zerolog.New(io.Discard),
		StaticFS: webembed.Embedded(),
	})

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if got := rec.Header().Get("X-Request-Id"); got == "" {
		t.Fatal("response missing generated X-Request-Id header")
	}
}

func TestRouterStopsWithCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	engine := newTestEngine(t, Dependencies{
		Log:      zerolog.New(io.Discard),
		StaticFS: webembed.Embedded(),
	})

	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestNeighborhoodAndWatchlistAPIRoutes(t *testing.T) {
	engine := newTestEngine(t, Dependencies{
		Log:                     zerolog.New(io.Discard),
		StaticFS:                webembed.Embedded(),
		NeighborhoodApplication: &stubNeighborhoodApplication{},
		AccessToken:             "secret-token",
	})

	for _, route := range []struct {
		method string
		path   string
		body   string
		status int
		auth   bool
	}{
		{method: http.MethodPost, path: "/api/v1/neighborhoods", body: `{"name":"青枫花园","area":"滨江核心","targetLayout":"三房"}`, status: http.StatusCreated, auth: true},
		{method: http.MethodGet, path: "/api/v1/neighborhoods/neighborhood_1", status: http.StatusOK},
		{method: http.MethodGet, path: "/api/v1/neighborhoods/neighborhood_1/metrics", status: http.StatusOK},
		{method: http.MethodPost, path: "/api/v1/watchlist/items", body: `{"neighborhoodId":"neighborhood_1"}`, status: http.StatusCreated, auth: true},
		{method: http.MethodGet, path: "/api/v1/watchlist", status: http.StatusOK, auth: true},
	} {
		req := httptest.NewRequest(route.method, route.path, strings.NewReader(route.body))
		req.Header.Set("Content-Type", "application/json")
		if route.auth {
			req.Header.Set("Authorization", "Bearer secret-token")
		}
		rec := httptest.NewRecorder()
		engine.ServeHTTP(rec, req)
		if rec.Code != route.status {
			t.Fatalf("%s %s status = %d, want %d; body=%s", route.method, route.path, rec.Code, route.status, rec.Body.String())
		}
	}
}

func TestDecisionActionWindowRoute(t *testing.T) {
	engine := newTestEngine(t, Dependencies{
		Log:                 zerolog.New(io.Discard),
		StaticFS:            webembed.Embedded(),
		DecisionApplication: &stubDecisionApplication{},
		AccessToken:         "secret-token",
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/decision/action-window", nil)
	req.Header.Set("Authorization", "Bearer secret-token")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
}

func TestAdminImportRoute(t *testing.T) {
	engine := newTestEngine(t, Dependencies{
		Log:                   zerolog.New(io.Discard),
		StaticFS:              webembed.Embedded(),
		CollectionApplication: &stubCollectionApplication{},
		AccessToken:           "secret-token",
	})

	req := httptest.NewRequest(http.MethodPost, "/admin/api/imports/json", strings.NewReader(`{
		"dataSourceId": "11111111-1111-1111-1111-111111111111",
		"sourceRef": "demo-weekly-import",
		"neighborhoodId": "22222222-2222-2222-2222-222222222222",
		"collectedAt": "2026-07-13T10:00:00Z",
		"coverage": "full",
		"records": [{"recordType":"listing","sourceRecordId":"listing-1","layout":"三房","areaSqm":89,"listingPrice":520,"daysOnMarket":0,"status":"active"}]
	}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer secret-token")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", rec.Code, rec.Body.String())
	}
	var response struct {
		CollectionRun struct {
			ID string `json:"id"`
		} `json:"collectionRun"`
		ListingObservationCount int `json:"listingObservationCount"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if response.CollectionRun.ID != "33333333-3333-3333-3333-333333333333" || response.ListingObservationCount != 1 {
		t.Fatalf("response = %#v", response)
	}
}

func TestInjectedInMemoryApplicationsShareNeighborhoodStateWithCollectionImports(t *testing.T) {
	engine := newTestEngine(t, Dependencies{
		Log:         zerolog.New(io.Discard),
		StaticFS:    webembed.Embedded(),
		AccessToken: "secret-token",
	})

	createNeighborhood := httptest.NewRequest(http.MethodPost, "/api/v1/neighborhoods", strings.NewReader(`{
		"name": "青枫花园",
		"area": "滨江核心",
		"targetLayout": "三房"
	}`))
	createNeighborhood.Header.Set("Content-Type", "application/json")
	createNeighborhood.Header.Set("Authorization", "Bearer secret-token")
	createNeighborhoodRecorder := httptest.NewRecorder()
	engine.ServeHTTP(createNeighborhoodRecorder, createNeighborhood)
	if createNeighborhoodRecorder.Code != http.StatusCreated {
		t.Fatalf("create neighborhood status = %d, want 201; body=%s", createNeighborhoodRecorder.Code, createNeighborhoodRecorder.Body.String())
	}
	var neighborhood struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(createNeighborhoodRecorder.Body.Bytes(), &neighborhood); err != nil {
		t.Fatalf("json.Unmarshal(neighborhood) error = %v", err)
	}
	if neighborhood.ID == "" {
		t.Fatal("created neighborhood ID is empty")
	}

	createSource := httptest.NewRequest(http.MethodPost, "/admin/api/data-sources", strings.NewReader(`{
		"name": "链家手工导入",
		"sourceType": "manual_json",
		"city": "杭州"
	}`))
	createSource.Header.Set("Content-Type", "application/json")
	createSource.Header.Set("Authorization", "Bearer secret-token")
	createSourceRecorder := httptest.NewRecorder()
	engine.ServeHTTP(createSourceRecorder, createSource)
	if createSourceRecorder.Code != http.StatusCreated {
		t.Fatalf("create source status = %d, want 201; body=%s", createSourceRecorder.Code, createSourceRecorder.Body.String())
	}
	var source struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(createSourceRecorder.Body.Bytes(), &source); err != nil || source.ID == "" {
		t.Fatalf("source response = %s, error=%v", createSourceRecorder.Body.String(), err)
	}

	importRequest := httptest.NewRequest(http.MethodPost, "/admin/api/imports/json", strings.NewReader(`{
		"dataSourceId": "`+source.ID+`",
		"sourceRef": "fallback-weekly-import",
		"neighborhoodId": "`+neighborhood.ID+`",
		"collectedAt": "2026-07-13T10:00:00Z",
		"coverage": "full",
		"records": [{"recordType":"listing","sourceRecordId":"listing-1","layout":"三房","areaSqm":89,"listingPrice":520,"daysOnMarket":0,"status":"active"}]
	}`))
	importRequest.Header.Set("Content-Type", "application/json")
	importRequest.Header.Set("Authorization", "Bearer secret-token")
	importRecorder := httptest.NewRecorder()
	engine.ServeHTTP(importRecorder, importRequest)
	if importRecorder.Code != http.StatusCreated {
		t.Fatalf("import status = %d, want 201; body=%s", importRecorder.Code, importRecorder.Body.String())
	}
}

func TestInMemoryCollectionRepositorySavesTrustedRunsInSharedMarketState(t *testing.T) {
	ctx := context.Background()
	marketState := newInMemoryMarketState()
	neighborhoods := newInMemoryNeighborhoodRepository(marketState)
	repo := newInMemoryCollectionRepository(neighborhoods, marketState)
	neighborhood, err := neighborhoods.CreateNeighborhood(ctx, appneighborhood.CreateNeighborhoodInput{
		ID:           "neighborhood_1",
		Name:         "青枫花园",
		Area:         "滨江核心",
		TargetLayout: "三房",
	})
	if err != nil {
		t.Fatalf("CreateNeighborhood() error = %v", err)
	}
	source, err := repo.CreateDataSource(ctx, appcollection.DataSource{
		ID:         "source_1",
		Name:       "链家手工",
		SourceType: "manual_json",
		City:       "杭州",
		Notes:      "fallback source",
	})
	if err != nil {
		t.Fatalf("CreateDataSource() error = %v", err)
	}

	exists, err := repo.NeighborhoodExists(ctx, neighborhood.ID)
	if err != nil {
		t.Fatalf("NeighborhoodExists() error = %v", err)
	}
	if !exists {
		t.Fatal("NeighborhoodExists() = false, want true")
	}
	run := appcollection.CollectionRun{
		ID:              "run_1",
		DataSourceID:    source.ID,
		NeighborhoodID:  neighborhood.ID,
		SourceRef:       "weekly-2026-07-09",
		CollectedAt:     time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC),
		Coverage:        domainneighborhood.CoverageFull,
		Format:          appcollection.ImportFormatJSON,
		ContentChecksum: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		RawPayload:      []byte(`{"records":[]}`),
		RawContentType:  "application/json",
		ValidationSummary: appcollection.ValidationSummary{
			RecordCount:      1,
			ListingCount:     1,
			TransactionCount: 0,
			Issues:           []appcollection.ValidationIssue{},
		},
		Status:       appcollection.CollectionRunStatusCompleted,
		MetricStatus: appcollection.MetricStatusPending,
	}
	result, err := repo.SaveCollectionRun(ctx, appcollection.ImportBatch{
		Run: run,
		Listings: []appcollection.ListingObservation{
			{
				ID:              "listing_1",
				CollectionRunID: "wrong-run",
				NeighborhoodID:  "wrong-neighborhood",
				SourceListingID: "listing-source-1",
				SourceRow:       1,
				Layout:          "三房",
				AreaSQM:         89,
				ListingPrice:    520,
				DaysOnMarket:    0,
				Status:          appcollection.ListingStatusActive,
			},
		},
	})
	if err != nil {
		t.Fatalf("SaveCollectionRun() error = %v", err)
	}
	if !result.Created {
		t.Fatal("SaveCollectionRun() Created = false, want true")
	}
	replay := run
	replay.ID = "run_2"
	replayResult, err := repo.SaveCollectionRun(ctx, appcollection.ImportBatch{Run: replay})
	if err != nil {
		t.Fatalf("SaveCollectionRun(replay) error = %v", err)
	}
	if replayResult.Created || replayResult.Run.ID != run.ID {
		t.Fatalf("SaveCollectionRun(replay) = %#v, want existing run", replayResult)
	}
	detail, err := repo.GetCollectionRun(ctx, run.ID)
	if err != nil {
		t.Fatalf("GetCollectionRun() error = %v", err)
	}
	if detail.Source.ID != source.ID || detail.Run.ID != run.ID || len(detail.Listings) != 1 {
		t.Fatalf("detail = %#v", detail)
	}
	if detail.Listings[0].CollectionRunID != run.ID || detail.Listings[0].NeighborhoodID != neighborhood.ID {
		t.Fatalf("listing IDs = %#v, want batch run/neighborhood ids", detail.Listings[0])
	}
}

func TestProtectedRoutesRequireAccessToken(t *testing.T) {
	tests := []struct {
		method string
		path   string
		body   string
	}{
		{method: http.MethodPost, path: "/api/v1/capacity/calculations", body: `{}`},
		{method: http.MethodGet, path: "/api/v1/access"},
		{method: http.MethodGet, path: "/api/v1/capacity/calculations/calculation_1"},
		{method: http.MethodPost, path: "/api/v1/neighborhoods", body: `{}`},
		{method: http.MethodPost, path: "/api/v1/watchlist/items", body: `{}`},
		{method: http.MethodGet, path: "/api/v1/watchlist"},
		{method: http.MethodGet, path: "/api/v1/decision/action-window"},
		{method: http.MethodPost, path: "/admin/api/data-sources", body: `{}`},
		{method: http.MethodGet, path: "/admin/api/data-sources"},
		{method: http.MethodPost, path: "/admin/api/imports/json", body: `{}`},
		{method: http.MethodGet, path: "/admin/api/imports/33333333-3333-3333-3333-333333333333"},
	}

	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			capacityApp := &stubCapacityApplication{}
			neighborhoodApp := &stubNeighborhoodApplication{}
			collectionApp := &stubCollectionApplication{}
			decisionApp := &stubDecisionApplication{}
			engine := newTestEngine(t, Dependencies{
				Log:                     zerolog.New(io.Discard),
				StaticFS:                webembed.Embedded(),
				CapacityApplication:     capacityApp,
				NeighborhoodApplication: neighborhoodApp,
				CollectionApplication:   collectionApp,
				DecisionApplication:     decisionApp,
				AccessToken:             "secret-token",
			})

			req := httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			engine.ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want 401; body=%s", rec.Code, rec.Body.String())
			}
			if calls := capacityApp.calls + neighborhoodApp.calls + collectionApp.calls + decisionApp.calls; calls != 0 {
				t.Fatalf("application calls = %d, want 0", calls)
			}
		})
	}
}

func TestPublicNeighborhoodReadsDoNotRequireAccessToken(t *testing.T) {
	for _, path := range []string{
		"/api/v1/neighborhoods/neighborhood_1",
		"/api/v1/neighborhoods/neighborhood_1/metrics",
	} {
		t.Run(path, func(t *testing.T) {
			service := &stubNeighborhoodApplication{}
			engine := newTestEngine(t, Dependencies{
				Log:                     zerolog.New(io.Discard),
				StaticFS:                webembed.Embedded(),
				NeighborhoodApplication: service,
				AccessToken:             "secret-token",
			})

			req := httptest.NewRequest(http.MethodGet, path, nil)
			rec := httptest.NewRecorder()
			engine.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
			}
			if service.calls != 1 {
				t.Fatalf("application calls = %d, want 1", service.calls)
			}
		})
	}
}

type stubCapacityApplication struct {
	calls int
}

func (s *stubCapacityApplication) CreateCalculation(_ context.Context, _ appcapacity.CreateCalculationCommand) (appcapacity.CalculationRecord, error) {
	s.calls++
	return appcapacity.CalculationRecord{}, nil
}

func (s *stubCapacityApplication) GetCalculation(_ context.Context, _ appcapacity.GetCalculationQuery) (appcapacity.CalculationRecord, error) {
	s.calls++
	return appcapacity.CalculationRecord{}, nil
}

func (s *stubCapacityApplication) LatestCalculation(_ context.Context, _ appcapacity.LatestCalculationQuery) (appcapacity.CalculationRecord, error) {
	s.calls++
	return appcapacity.CalculationRecord{}, nil
}

type stubNeighborhoodApplication struct {
	calls int
}

func (s *stubNeighborhoodApplication) CreateNeighborhood(_ context.Context, _ appneighborhood.CreateNeighborhoodCommand) (appneighborhood.Neighborhood, error) {
	s.calls++
	return appneighborhood.Neighborhood{ID: "neighborhood_1", Name: "青枫花园", Area: "滨江核心", TargetLayout: "三房"}, nil
}

func (s *stubNeighborhoodApplication) GetNeighborhood(_ context.Context, _ appneighborhood.GetNeighborhoodQuery) (appneighborhood.Neighborhood, error) {
	s.calls++
	return appneighborhood.Neighborhood{ID: "neighborhood_1", Name: "青枫花园", Area: "滨江核心", TargetLayout: "三房"}, nil
}

func (s *stubNeighborhoodApplication) SearchNeighborhoods(_ context.Context, _ appneighborhood.SearchNeighborhoodsQuery) (appneighborhood.SearchNeighborhoodsPage, error) {
	s.calls++
	return appneighborhood.SearchNeighborhoodsPage{
		Items:    []appneighborhood.Neighborhood{{ID: "neighborhood_1", Name: "青枫花园", Area: "滨江核心", TargetLayout: "三房"}},
		Total:    1,
		Page:     1,
		PageSize: 20,
	}, nil
}

func (s *stubNeighborhoodApplication) LatestMetric(_ context.Context, _ appneighborhood.LatestMetricQuery) (appneighborhood.MetricWithSignal, error) {
	s.calls++
	return appneighborhood.MetricWithSignal{
		Metric: appneighborhood.MetricSnapshot{
			ID:                  "metric_1",
			NeighborhoodID:      "neighborhood_1",
			ListedHomes:         42,
			PriceCutHomes:       11,
			TransactionMomentum: domainneighborhood.TransactionMomentumWeak,
		},
		Signal: domainneighborhood.SignalResult{
			Status:         domainneighborhood.NeighborhoodStatusBargain,
			SupplyPressure: domainneighborhood.SupplyPressureHigh,
			NextAction:     "重点看 495-545 万成交区间附近房源，对挂牌久、降价过的房源试探底价。",
		},
	}, nil
}

func (s *stubNeighborhoodApplication) AddWatchlistItem(_ context.Context, _ appneighborhood.AddWatchlistItemCommand) (appneighborhood.WatchlistItem, error) {
	s.calls++
	return appneighborhood.WatchlistItem{ID: "watch_1", UserID: user.SingleUserID, NeighborhoodID: "neighborhood_1"}, nil
}

func (s *stubNeighborhoodApplication) ListWatchlist(_ context.Context, _ appneighborhood.ListWatchlistQuery) ([]appneighborhood.WatchlistItemSummary, error) {
	s.calls++
	return []appneighborhood.WatchlistItemSummary{
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
	}, nil
}

type stubCollectionApplication struct {
	calls int
}

func (s *stubCollectionApplication) CreateDataSource(_ context.Context, command appcollection.CreateDataSourceCommand) (appcollection.DataSource, error) {
	s.calls++
	now := time.Date(2026, 7, 13, 10, 0, 0, 0, time.UTC)
	return appcollection.DataSource{ID: "11111111-1111-1111-1111-111111111111", Name: command.Name, SourceType: command.SourceType, City: command.City, Notes: command.Notes, CreatedAt: now, UpdatedAt: now}, nil
}

func (s *stubCollectionApplication) ListDataSources(context.Context, appcollection.ListDataSourcesQuery) ([]appcollection.DataSource, error) {
	s.calls++
	return []appcollection.DataSource{}, nil
}

func (s *stubCollectionApplication) ImportCollectionRun(_ context.Context, command appcollection.ImportCollectionRunCommand) (appcollection.ImportCollectionRunResult, error) {
	s.calls++
	now := time.Date(2026, 7, 13, 10, 0, 0, 0, time.UTC)
	return appcollection.ImportCollectionRunResult{
		Run: appcollection.CollectionRun{
			ID: "33333333-3333-3333-3333-333333333333", DataSourceID: command.DataSourceID,
			NeighborhoodID: command.NeighborhoodID, SourceRef: command.SourceRef, CollectedAt: command.CollectedAt,
			Coverage: command.Coverage, Format: command.Format, RawContentType: command.RawContentType,
			Status: appcollection.CollectionRunStatusCompleted, MetricStatus: appcollection.MetricStatusCompleted,
			CreatedAt: now, UpdatedAt: now,
		},
		ListingCount: len(command.Records), MetricRefreshStatus: appcollection.MetricStatusCompleted,
	}, nil
}

func (s *stubCollectionApplication) GetCollectionRun(context.Context, appcollection.GetCollectionRunQuery) (appcollection.CollectionRunDetail, error) {
	s.calls++
	return appcollection.CollectionRunDetail{}, nil
}

type stubDecisionApplication struct {
	calls int
}

func (s *stubDecisionApplication) GetActionWindow(_ context.Context, _ appdecision.GetActionWindowQuery) (domaindecision.ActionWindowResult, error) {
	s.calls++
	return domaindecision.ActionWindowResult{
		Action:     domaindecision.ActionBargain,
		Confidence: domaindecision.ConfidenceHigh,
		Summary:    "预算仍可服务，且目标小区供应与降价信号支持买方试探底价。",
		Checklist:  []string{"约看 3 套成交区间附近、挂牌超过 60 天的目标户型。"},
		Risks:      []string{"预算不是完全宽松，砍价失败时不要上调总价硬追。"},
	}, nil
}
