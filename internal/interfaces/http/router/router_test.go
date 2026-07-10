package router

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
	engine := New(Dependencies{
		Log:      zerolog.New(io.Discard),
		StaticFS: webembed.Embedded(),
	})

	for _, path := range []string{"/healthz", "/readyz"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		engine.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status = %d, want 200", path, rec.Code)
		}
	}
}

func TestAPI404DoesNotReturnFrontend(t *testing.T) {
	engine := New(Dependencies{
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
	engine := New(Dependencies{
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
	engine := New(Dependencies{
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
	engine := New(Dependencies{
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

	engine := New(Dependencies{
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
	engine := New(Dependencies{
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
	engine := New(Dependencies{
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
	engine := New(Dependencies{
		Log:                   zerolog.New(io.Discard),
		StaticFS:              webembed.Embedded(),
		CollectionApplication: &stubCollectionApplication{},
		AccessToken:           "secret-token",
	})

	req := httptest.NewRequest(http.MethodPost, "/admin/api/imports", strings.NewReader(`{
		"sourceType": "manual_json",
		"sourceRef": "demo-weekly-import",
		"neighborhoodId": "neighborhood_1",
		"records": [{"listingPrice": 520, "daysOnMarket": 0}]
	}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer secret-token")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", rec.Code, rec.Body.String())
	}
	var response struct {
		CollectionRunID       string `json:"collectionRunId"`
		ImportedSnapshotCount int    `json:"importedSnapshotCount"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if response.CollectionRunID != "collection_run_1" || response.ImportedSnapshotCount != 1 {
		t.Fatalf("response = %#v", response)
	}
}

func TestProtectedRoutesRequireAccessToken(t *testing.T) {
	tests := []struct {
		method string
		path   string
		body   string
	}{
		{method: http.MethodPost, path: "/api/v1/capacity/calculations", body: `{}`},
		{method: http.MethodGet, path: "/api/v1/capacity/calculations/calculation_1"},
		{method: http.MethodPost, path: "/api/v1/neighborhoods", body: `{}`},
		{method: http.MethodPost, path: "/api/v1/watchlist/items", body: `{}`},
		{method: http.MethodGet, path: "/api/v1/watchlist"},
		{method: http.MethodGet, path: "/api/v1/decision/action-window"},
		{method: http.MethodPost, path: "/admin/api/imports", body: `{}`},
	}

	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			capacityApp := &stubCapacityApplication{}
			neighborhoodApp := &stubNeighborhoodApplication{}
			collectionApp := &stubCollectionApplication{}
			decisionApp := &stubDecisionApplication{}
			engine := New(Dependencies{
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
			engine := New(Dependencies{
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

func (s *stubCollectionApplication) ImportManualListings(_ context.Context, _ appcollection.ImportManualListingsCommand) (appcollection.ImportManualListingsResult, error) {
	s.calls++
	return appcollection.ImportManualListingsResult{CollectionRunID: "collection_run_1", ImportedSnapshotCount: 1}, nil
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
