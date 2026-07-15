package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	appneighborhood "github.com/sine-io/propulse/internal/application/neighborhood"
	"github.com/sine-io/propulse/internal/application/user"
	domainneighborhood "github.com/sine-io/propulse/internal/domain/neighborhood"
)

func TestSearchNeighborhoodsReturnsPagedResults(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	service := &stubNeighborhoodApplication{
		searchPage: appneighborhood.SearchNeighborhoodsPage{
			Items: []appneighborhood.Neighborhood{
				{ID: "neighborhood_1", Name: "青枫花园", City: handlerStringPtr("杭州"), Area: "滨江核心", AvailableLayouts: []string{"三房", "四房"}},
			},
			Total:    1,
			Page:     2,
			PageSize: 10,
			Filters: appneighborhood.NeighborhoodSearchFilters{
				Cities: []string{"杭州", "上海"},
				Areas:  []appneighborhood.NeighborhoodAreaFilter{{City: "杭州", Area: "滨江核心"}},
			},
		},
	}
	engine := gin.New()
	engine.GET("/api/v1/neighborhoods", NewNeighborhood(service).SearchNeighborhoods)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/neighborhoods?q=青枫&city=杭州&area=滨江核心&targetLayout=三房&page=2&pageSize=10", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if service.searchQuery.Query != "青枫" || service.searchQuery.City != "杭州" || service.searchQuery.Area != "滨江核心" || service.searchQuery.TargetLayout != "三房" {
		t.Fatalf("search query = %#v, want q=青枫 area=滨江核心", service.searchQuery)
	}
	if service.searchQuery.Page != 2 || service.searchQuery.PageSize != 10 {
		t.Fatalf("pagination = page %d size %d, want 2/10", service.searchQuery.Page, service.searchQuery.PageSize)
	}

	var body neighborhoodSearchResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body.Total != 1 || len(body.Items) != 1 || body.Items[0].ID != "neighborhood_1" || len(body.Items[0].AvailableLayouts) != 2 || len(body.Filters.Cities) != 2 || len(body.Filters.Areas) != 1 {
		t.Fatalf("body = %#v, want 1 item neighborhood_1", body)
	}
}

func TestSearchNeighborhoodsRejectsInvalidPage(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	engine.GET("/api/v1/neighborhoods", NewNeighborhood(&stubNeighborhoodApplication{}).SearchNeighborhoods)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/neighborhoods?page=0", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestCreateNeighborhoodReturnsCreatedNeighborhood(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	service := &stubNeighborhoodApplication{
		createNeighborhood: appneighborhood.Neighborhood{
			ID:               "neighborhood_1",
			Name:             "青枫花园",
			City:             handlerStringPtr("杭州"),
			Area:             "滨江核心",
			AvailableLayouts: []string{"三房", "四房"},
			CreatedAt:        time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC),
		},
	}
	engine := gin.New()
	engine.POST("/api/v1/neighborhoods", NewNeighborhood(service).CreateNeighborhood)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/neighborhoods", bytes.NewBufferString(`{"name":"青枫花园","city":"杭州","area":"滨江核心","availableLayouts":["三房","四房"]}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201", rec.Code)
	}

	var response struct {
		ID               string   `json:"id"`
		Name             string   `json:"name"`
		City             *string  `json:"city"`
		Area             string   `json:"area"`
		AvailableLayouts []string `json:"availableLayouts"`
		CreatedAt        string   `json:"createdAt"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if response.ID != "neighborhood_1" || response.Name != "青枫花园" || response.City == nil || *response.City != "杭州" || len(response.AvailableLayouts) != 2 {
		t.Fatalf("response = %#v", response)
	}
	if response.CreatedAt != "2026-07-09T12:00:00Z" {
		t.Fatalf("CreatedAt = %q", response.CreatedAt)
	}
	if service.createCommand.City != "杭州" || len(service.createCommand.AvailableLayouts) != 2 {
		t.Fatalf("create command = %#v", service.createCommand)
	}
}

func TestCreateNeighborhoodRejectsMissingRequiredFields(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	service := &stubNeighborhoodApplication{createNeighborhoodErr: appneighborhood.ErrInvalidNeighborhood}
	engine := gin.New()
	engine.POST("/api/v1/neighborhoods", NewNeighborhood(service).CreateNeighborhood)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/neighborhoods", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if !service.createCalled {
		t.Fatal("CreateNeighborhood was not called for application validation")
	}
	var response struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if response.Error.Code != "invalid_request" {
		t.Fatalf("error code = %q, want invalid_request", response.Error.Code)
	}
}

func TestGetNeighborhoodReturnsStoredNeighborhood(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	service := &stubNeighborhoodApplication{
		getNeighborhood: appneighborhood.Neighborhood{
			ID:               "neighborhood_1",
			Name:             "青枫花园",
			City:             handlerStringPtr("杭州"),
			Area:             "滨江核心",
			AvailableLayouts: []string{"三房"},
			CreatedAt:        time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC),
		},
	}
	engine := gin.New()
	engine.GET("/api/v1/neighborhoods/:id", NewNeighborhood(service).GetNeighborhood)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/neighborhoods/neighborhood_1", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var response struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if response.ID != "neighborhood_1" {
		t.Fatalf("ID = %q, want neighborhood_1", response.ID)
	}
}

func TestGetNeighborhoodReturnsNotFound(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	service := &stubNeighborhoodApplication{getNeighborhoodErr: appneighborhood.ErrNeighborhoodNotFound}
	engine := gin.New()
	engine.GET("/api/v1/neighborhoods/:id", NewNeighborhood(service).GetNeighborhood)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/neighborhoods/missing", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestGetNeighborhoodMetricsReturnsLatestSignal(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	evidence := domainneighborhood.NewTransactionMomentumEvidence(time.Date(2026, 7, 9, 0, 0, 0, 0, time.UTC), 1, 2)
	service := &stubNeighborhoodApplication{
		latestMetric: appneighborhood.MetricWithSignal{
			Metric: appneighborhood.MetricSnapshot{
				ID:                  "metric_1",
				NeighborhoodID:      "neighborhood_1",
				CollectionRunID:     "11111111-1111-1111-1111-111111111111",
				AlgorithmVersion:    "market-metrics/test.1",
				CollectedAt:         time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC),
				ListedHomes:         42,
				PriceCutHomes:       11,
				AvgDaysOnMarket:     handlerFloatPtr(78),
				ListingPriceMin:     handlerFloatPtr(520),
				ListingPriceMax:     handlerFloatPtr(620),
				TransactionPriceMin: handlerFloatPtr(495),
				TransactionPriceMax: handlerFloatPtr(545),
				TransactionMomentum: domainneighborhood.TransactionMomentumWeak,
				TransactionEvidence: &evidence,
				TargetLayout:        "三房",
				TargetLayoutSupply:  12,
				CalculatedAt:        time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC),
			},
			Signal: domainneighborhood.SignalResult{
				Status:         domainneighborhood.NeighborhoodStatusBargain,
				SupplyPressure: domainneighborhood.SupplyPressureHigh,
				NextAction:     "重点看 495-545 万成交区间附近房源，对挂牌久、降价过的房源试探底价。",
			},
		},
	}
	engine := gin.New()
	engine.GET("/api/v1/neighborhoods/:id/metrics", NewNeighborhood(service).GetMetrics)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/neighborhoods/neighborhood_1/metrics?targetLayout=三房", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var response struct {
		Status              string `json:"status"`
		SupplyPressure      string `json:"supplyPressure"`
		Advice              string `json:"advice"`
		CollectedAt         string `json:"collectedAt"`
		AlgorithmVersion    string `json:"algorithmVersion"`
		TargetLayout        string `json:"targetLayout"`
		TargetLayoutSupply  int    `json:"targetLayoutSupply"`
		TransactionEvidence struct {
			WindowStart                     string  `json:"windowStart"`
			RecentThirtyDayMonthlyFrequency float64 `json:"recent30DayMonthlyFrequency"`
		} `json:"transactionEvidence"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if response.Status != "适合砍价" || response.SupplyPressure != "high" {
		t.Fatalf("response = %#v", response)
	}
	if response.TargetLayout != "三房" || response.TargetLayoutSupply != 12 || service.latestMetricQuery.TargetLayout != "三房" {
		t.Fatalf("target projection response/query = %#v/%#v", response, service.latestMetricQuery)
	}
	if response.CollectedAt != "2026-07-09T12:00:00Z" || response.AlgorithmVersion != "market-metrics/test.1" || response.TransactionEvidence.WindowStart != "2026-04-10" || response.TransactionEvidence.RecentThirtyDayMonthlyFrequency != 1 {
		t.Fatalf("versioned evidence response = %#v", response)
	}
}

func TestGetNeighborhoodMetricsRejectsMissingOrUnavailableTargetLayout(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	service := &stubNeighborhoodApplication{latestMetricErr: appneighborhood.ErrInvalidTargetLayout}
	engine := gin.New()
	engine.GET("/api/v1/neighborhoods/:id/metrics", NewNeighborhood(service).GetMetrics)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/neighborhoods/neighborhood_1/metrics", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), `"code":"invalid_target_layout"`) {
		t.Fatalf("status/body = %d/%s", rec.Code, rec.Body.String())
	}
	if service.latestMetricQuery.TargetLayout != "" {
		t.Fatalf("target layout = %q, want empty input passed to application validation", service.latestMetricQuery.TargetLayout)
	}
}

func TestGetMetricHistoryReturnsWindowSourcesAndComparisons(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	collectedAt := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	evidence := domainneighborhood.NewTransactionMomentumEvidence(collectedAt, 2, 2)
	currentBatch := appneighborhood.CollectionRunReference{
		CollectionRunID: "11111111-1111-1111-1111-111111111111",
		DataSourceID:    "22222222-2222-2222-2222-222222222222",
		SourceRef:       "weekly-2026-07-14",
		CollectedAt:     collectedAt,
		Coverage:        domainneighborhood.CoverageFull,
	}
	baselineBatch := appneighborhood.CollectionRunReference{
		CollectionRunID: "33333333-3333-3333-3333-333333333333",
		DataSourceID:    currentBatch.DataSourceID,
		SourceRef:       "weekly-2026-07-07",
		CollectedAt:     collectedAt.Add(-7 * 24 * time.Hour),
		Coverage:        domainneighborhood.CoverageFull,
	}
	listedChange := domainneighborhood.CalculateMetricChange(12, 10)
	service := &stubNeighborhoodApplication{metricHistory: appneighborhood.MetricHistoryResult{
		Status:           appneighborhood.MetricHistoryReady,
		NeighborhoodID:   "neighborhood_1",
		TargetLayout:     "三房",
		AlgorithmVersion: "market-metrics/test.1",
		From:             collectedAt.Add(-8 * 7 * 24 * time.Hour),
		To:               collectedAt,
		Items: []appneighborhood.MetricHistoryPoint{{
			Metric: appneighborhood.MetricSnapshot{
				ID: "metric_1", NeighborhoodID: "neighborhood_1", AlgorithmVersion: "market-metrics/test.1",
				CollectedAt: collectedAt, LatestObservedAt: collectedAt, CalculatedAt: collectedAt.Add(time.Minute),
				SourceIDs: []string{currentBatch.DataSourceID}, ListedHomes: 12, PriceCutHomes: 3,
				TransactionMomentum: domainneighborhood.TransactionMomentumStable, TransactionEvidence: &evidence,
				TargetLayoutSupply: 5,
				ListingSampleCount: 12, TransactionSampleCount: 4, Coverage: domainneighborhood.CoverageFull,
				Freshness: domainneighborhood.FreshnessCurrent, QualityState: domainneighborhood.MarketQualitySufficient,
				QualityWarnings: []domainneighborhood.QualityWarning{},
			},
			Batch: currentBatch,
			WeeklyComparison: appneighborhood.MetricComparison{
				Status: domainneighborhood.MetricComparisonAvailable, CurrentBatch: currentBatch, BaselineBatch: &baselineBatch, ListedHomes: &listedChange,
			},
			MonthlyComparison: appneighborhood.MetricComparison{
				Status: domainneighborhood.MetricComparisonUnavailable, Reason: domainneighborhood.ComparisonReasonFullBaselineNotFound, CurrentBatch: currentBatch,
			},
		}},
	}}
	engine := gin.New()
	engine.GET("/api/v1/neighborhoods/:id/metrics/history", NewNeighborhood(service).GetMetricHistory)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/neighborhoods/neighborhood_1/metrics/history?targetLayout=三房&from=2026-05-19T12%3A00%3A00Z&to=2026-07-14T12%3A00%3A00Z", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; body=%s", rec.Code, rec.Body.String())
	}
	var response struct {
		Status           string `json:"status"`
		TargetLayout     string `json:"targetLayout"`
		AlgorithmVersion string `json:"algorithmVersion"`
		Items            []struct {
			Batch struct {
				SourceRef string `json:"sourceRef"`
			} `json:"batch"`
			WeeklyComparison struct {
				BaselineBatch *struct {
					CollectionRunID string `json:"collectionRunId"`
				} `json:"baselineBatch"`
				ListedHomes *struct {
					AbsoluteChange int `json:"absoluteChange"`
				} `json:"listedHomes"`
			} `json:"weeklyComparison"`
			MonthlyComparison struct {
				Status string `json:"status"`
				Reason string `json:"reason"`
			} `json:"monthlyComparison"`
			TargetLayoutSupply int `json:"targetLayoutSupply"`
		} `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if response.Status != "ready" || response.TargetLayout != "三房" || service.metricHistoryQuery.TargetLayout != "三房" || response.AlgorithmVersion != "market-metrics/test.1" || len(response.Items) != 1 || response.Items[0].TargetLayoutSupply != 5 || response.Items[0].Batch.SourceRef != "weekly-2026-07-14" || response.Items[0].WeeklyComparison.BaselineBatch == nil || response.Items[0].WeeklyComparison.BaselineBatch.CollectionRunID != baselineBatch.CollectionRunID || response.Items[0].WeeklyComparison.ListedHomes == nil || response.Items[0].WeeklyComparison.ListedHomes.AbsoluteChange != 2 || response.Items[0].MonthlyComparison.Status != "unavailable" || response.Items[0].MonthlyComparison.Reason != "full_baseline_not_found" {
		t.Fatalf("response = %#v", response)
	}
}

func TestGetMetricHistoryRejectsInvalidTimeQuery(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	engine.GET("/api/v1/neighborhoods/:id/metrics/history", NewNeighborhood(&stubNeighborhoodApplication{}).GetMetricHistory)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/neighborhoods/neighborhood_1/metrics/history?from=not-a-time", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "invalid_time_window") {
		t.Fatalf("status/body = %d/%s", rec.Code, rec.Body.String())
	}
}

func TestGetMetricHistoryRejectsMissingOrUnavailableTargetLayout(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	service := &stubNeighborhoodApplication{metricHistoryErr: appneighborhood.ErrInvalidTargetLayout}
	engine := gin.New()
	engine.GET("/api/v1/neighborhoods/:id/metrics/history", NewNeighborhood(service).GetMetricHistory)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/neighborhoods/neighborhood_1/metrics/history", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), `"code":"invalid_target_layout"`) {
		t.Fatalf("status/body = %d/%s", rec.Code, rec.Body.String())
	}
}

func handlerFloatPtr(value float64) *float64 {
	return &value
}

func TestCreateWatchlistItemUsesSingleUser(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	const neighborhoodID = "11111111-1111-4111-8111-111111111111"
	service := &stubNeighborhoodApplication{
		addWatchlistItem: appneighborhood.WatchlistItem{
			ID:             "22222222-2222-4222-8222-222222222222",
			UserID:         user.SingleUserID,
			NeighborhoodID: neighborhoodID,
			TargetLayout:   "三房",
			CreatedAt:      time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC),
		},
	}
	engine := gin.New()
	engine.POST("/api/v1/watchlist/items", NewWatchlist(service, user.SingleUserID).AddItem)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/watchlist/items", bytes.NewBufferString(`{"neighborhoodId":"`+neighborhoodID+`","targetLayout":"三房"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201", rec.Code)
	}
	if service.addCommand.UserID != user.SingleUserID || service.addCommand.NeighborhoodID != neighborhoodID || service.addCommand.TargetLayout != "三房" {
		t.Fatalf("add command = %#v", service.addCommand)
	}
	var response addWatchlistItemResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil || response.NeighborhoodID != neighborhoodID || response.TargetLayout != "三房" {
		t.Fatalf("response/error = %#v/%v", response, err)
	}
}

func TestCreateWatchlistItemRejectsMissingNeighborhoodID(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	service := &stubNeighborhoodApplication{addWatchlistItemErr: appneighborhood.ErrInvalidNeighborhoodID}
	engine := gin.New()
	engine.POST("/api/v1/watchlist/items", NewWatchlist(service, user.SingleUserID).AddItem)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/watchlist/items", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if !service.addCalled {
		t.Fatal("AddWatchlistItem was not called for application validation")
	}
	var response struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if response.Error.Code != "invalid_neighborhood_id" {
		t.Fatalf("error code = %q, want invalid_neighborhood_id", response.Error.Code)
	}
}

func TestCreateWatchlistItemMapsApplicationErrorsWithoutReturningAnItem(t *testing.T) {
	for _, test := range []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{name: "invalid layout", err: appneighborhood.ErrInvalidTargetLayout, wantStatus: http.StatusBadRequest, wantCode: "invalid_target_layout"},
		{name: "missing neighborhood", err: appneighborhood.ErrNeighborhoodNotFound, wantStatus: http.StatusNotFound, wantCode: "not_found"},
		{name: "duplicate", err: appneighborhood.ErrWatchlistItemExists, wantStatus: http.StatusConflict, wantCode: "watchlist_item_exists"},
	} {
		t.Run(test.name, func(t *testing.T) {
			gin.SetMode(gin.ReleaseMode)
			service := &stubNeighborhoodApplication{addWatchlistItemErr: test.err}
			engine := gin.New()
			engine.POST("/api/v1/watchlist/items", NewWatchlist(service, user.SingleUserID).AddItem)
			req := httptest.NewRequest(http.MethodPost, "/api/v1/watchlist/items", bytes.NewBufferString(`{"neighborhoodId":"11111111-1111-4111-8111-111111111111","targetLayout":"三房"}`))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			engine.ServeHTTP(rec, req)
			if rec.Code != test.wantStatus || !strings.Contains(rec.Body.String(), `"code":"`+test.wantCode+`"`) {
				t.Fatalf("status/body = %d/%s", rec.Code, rec.Body.String())
			}
			var payload map[string]json.RawMessage
			if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil || len(payload) != 1 || payload["error"] == nil {
				t.Fatalf("failure response contains a fabricated item: %s", rec.Body.String())
			}
		})
	}
}

func TestListWatchlistReturnsBriefShape(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	service := &stubNeighborhoodApplication{
		watchlist: []appneighborhood.WatchlistItemSummary{
			{
				ID:                     "watch_1",
				NeighborhoodID:         "neighborhood_1",
				Name:                   "青枫花园",
				City:                   handlerStringPtr("杭州"),
				Area:                   "滨江核心",
				TargetLayout:           "三房",
				Status:                 domainneighborhood.NeighborhoodStatusBargain,
				ListedHomes:            42,
				PriceCutHomes:          11,
				TransactionMomentum:    domainneighborhood.TransactionMomentumWeak,
				TargetLayoutSupply:     12,
				TargetLayoutScarcity:   domainneighborhood.ScarcityLow,
				Advice:                 "约看 500-530 万三房，尝试砍价，窗口期已打开。",
				HasMetric:              true,
				CollectionRunID:        "11111111-1111-1111-1111-111111111111",
				AlgorithmVersion:       "market-metrics/test.1",
				SourceIDs:              []string{"22222222-2222-2222-2222-222222222222"},
				CollectedAt:            handlerTimePtr(time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC)),
				TransactionSampleCount: 3,
				Coverage:               domainneighborhood.CoverageFull,
				Freshness:              domainneighborhood.FreshnessCurrent,
				QualityState:           domainneighborhood.MarketQualitySufficient,
				QualityWarnings:        []domainneighborhood.QualityWarning{},
				WeeklyComparison: &appneighborhood.MetricComparison{
					Status: domainneighborhood.MetricComparisonAvailable,
					CurrentBatch: appneighborhood.CollectionRunReference{
						CollectionRunID: "11111111-1111-1111-1111-111111111111",
						DataSourceID:    "22222222-2222-2222-2222-222222222222",
						SourceRef:       "current",
						CollectedAt:     time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC),
						Coverage:        domainneighborhood.CoverageFull,
					},
					PriceCutHomes: &domainneighborhood.MetricChangeValue{Current: 11, Baseline: 8, AbsoluteChange: 3, PercentageStatus: domainneighborhood.PercentageChangeAvailable},
				},
			},
		},
	}
	engine := gin.New()
	engine.GET("/api/v1/watchlist", NewWatchlist(service, user.SingleUserID).List)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/watchlist", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var response struct {
		Items []struct {
			ID                     string  `json:"id"`
			NeighborhoodID         string  `json:"neighborhoodId"`
			Name                   string  `json:"name"`
			City                   *string `json:"city"`
			Area                   string  `json:"area"`
			TargetLayout           string  `json:"targetLayout"`
			Status                 string  `json:"status"`
			ListedHomes            *int    `json:"listedHomes"`
			PriceCutHomes          *int    `json:"priceCutHomes"`
			TransactionMomentum    *string `json:"transactionMomentum"`
			TargetLayoutSupply     *int    `json:"targetLayoutSupply"`
			TargetLayoutScarcity   *string `json:"targetLayoutScarcity"`
			Advice                 string  `json:"advice"`
			HasMetric              bool    `json:"hasMetric"`
			AlgorithmVersion       string  `json:"algorithmVersion"`
			CollectedAt            *string `json:"collectedAt"`
			TransactionSampleCount *int    `json:"transactionSampleCount"`
			QualityState           string  `json:"qualityState"`
			WeeklyComparison       *struct {
				Status string `json:"status"`
			} `json:"weeklyComparison"`
		} `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if len(response.Items) != 1 {
		t.Fatalf("items = %d, want 1", len(response.Items))
	}
	if response.Items[0].NeighborhoodID != "neighborhood_1" || response.Items[0].City == nil || *response.Items[0].City != "杭州" || response.Items[0].TargetLayoutSupply == nil || *response.Items[0].TargetLayoutSupply != 12 || response.Items[0].TargetLayoutScarcity == nil || *response.Items[0].TargetLayoutScarcity != "low" || response.Items[0].Status != "适合砍价" || !response.Items[0].HasMetric || response.Items[0].AlgorithmVersion != "market-metrics/test.1" || response.Items[0].CollectedAt == nil || response.Items[0].TransactionSampleCount == nil || *response.Items[0].TransactionSampleCount != 3 || response.Items[0].QualityState != "sufficient" || response.Items[0].WeeklyComparison == nil || response.Items[0].WeeklyComparison.Status != "available" {
		t.Fatalf("item = %#v", response.Items[0])
	}
}

func TestListWatchlistReturnsNeutralSummaryWithoutMetric(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	service := &stubNeighborhoodApplication{
		watchlist: []appneighborhood.WatchlistItemSummary{
			{
				ID:                  "watch_1",
				NeighborhoodID:      "neighborhood_1",
				Name:                "青枫花园",
				Area:                "滨江核心",
				TargetLayout:        "三房",
				Status:              domainneighborhood.NeighborhoodStatusInsufficientData,
				TransactionMomentum: domainneighborhood.TransactionMomentumUnknown,
				Advice:              "暂无指标数据，等待导入或计算后再判断。",
				HasMetric:           false,
				SourceIDs:           []string{},
				Coverage:            domainneighborhood.CoverageUnknown,
				Freshness:           domainneighborhood.FreshnessUnknown,
				QualityState:        domainneighborhood.MarketQualityInsufficientData,
				QualityWarnings:     []domainneighborhood.QualityWarning{domainneighborhood.WarningMetricUnavailable},
			},
		},
	}
	engine := gin.New()
	engine.GET("/api/v1/watchlist", NewWatchlist(service, user.SingleUserID).List)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/watchlist", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var response struct {
		Items []struct {
			Status               string    `json:"status"`
			ListedHomes          *int      `json:"listedHomes"`
			PriceCutHomes        *int      `json:"priceCutHomes"`
			TransactionMomentum  *string   `json:"transactionMomentum"`
			TargetLayoutSupply   *int      `json:"targetLayoutSupply"`
			TargetLayoutScarcity *string   `json:"targetLayoutScarcity"`
			Advice               string    `json:"advice"`
			WeeklyComparison     *struct{} `json:"weeklyComparison"`
		} `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if len(response.Items) != 1 {
		t.Fatalf("items = %d, want 1", len(response.Items))
	}
	item := response.Items[0]
	if item.Status != "数据不足" || item.Advice != "暂无指标数据，等待导入或计算后再判断。" {
		t.Fatalf("item = %#v", item)
	}
	if item.ListedHomes != nil || item.PriceCutHomes != nil || item.TransactionMomentum != nil || item.TargetLayoutSupply != nil || item.TargetLayoutScarcity != nil || item.WeeklyComparison != nil {
		t.Fatalf("metric fields = listed %#v, price cuts %#v, momentum %#v, comparison %#v", item.ListedHomes, item.PriceCutHomes, item.TransactionMomentum, item.WeeklyComparison)
	}
}

func handlerTimePtr(value time.Time) *time.Time { return &value }

func handlerStringPtr(value string) *string { return &value }

type stubNeighborhoodApplication struct {
	createNeighborhood    appneighborhood.Neighborhood
	createNeighborhoodErr error
	createCalled          bool
	createCommand         appneighborhood.CreateNeighborhoodCommand
	getNeighborhood       appneighborhood.Neighborhood
	getNeighborhoodErr    error
	latestMetric          appneighborhood.MetricWithSignal
	latestMetricErr       error
	latestMetricQuery     appneighborhood.LatestMetricQuery
	addWatchlistItem      appneighborhood.WatchlistItem
	addWatchlistItemErr   error
	addCommand            appneighborhood.AddWatchlistItemCommand
	addCalled             bool
	watchlist             []appneighborhood.WatchlistItemSummary
	watchlistErr          error
	searchPage            appneighborhood.SearchNeighborhoodsPage
	searchErr             error
	searchQuery           appneighborhood.SearchNeighborhoodsQuery
	metricHistory         appneighborhood.MetricHistoryResult
	metricHistoryErr      error
	metricHistoryQuery    appneighborhood.MetricHistoryQuery
}

func (s *stubNeighborhoodApplication) CreateNeighborhood(_ context.Context, command appneighborhood.CreateNeighborhoodCommand) (appneighborhood.Neighborhood, error) {
	s.createCalled = true
	s.createCommand = command
	return s.createNeighborhood, s.createNeighborhoodErr
}

func (s *stubNeighborhoodApplication) SearchNeighborhoods(_ context.Context, query appneighborhood.SearchNeighborhoodsQuery) (appneighborhood.SearchNeighborhoodsPage, error) {
	s.searchQuery = query
	if s.searchErr != nil {
		return appneighborhood.SearchNeighborhoodsPage{}, s.searchErr
	}
	return s.searchPage, nil
}

func (s *stubNeighborhoodApplication) GetNeighborhood(_ context.Context, _ appneighborhood.GetNeighborhoodQuery) (appneighborhood.Neighborhood, error) {
	if s.getNeighborhoodErr != nil {
		return appneighborhood.Neighborhood{}, s.getNeighborhoodErr
	}
	return s.getNeighborhood, nil
}

func (s *stubNeighborhoodApplication) LatestMetric(_ context.Context, query appneighborhood.LatestMetricQuery) (appneighborhood.MetricWithSignal, error) {
	s.latestMetricQuery = query
	if s.latestMetricErr != nil {
		return appneighborhood.MetricWithSignal{}, s.latestMetricErr
	}
	return s.latestMetric, nil
}

func (s *stubNeighborhoodApplication) MetricHistory(_ context.Context, query appneighborhood.MetricHistoryQuery) (appneighborhood.MetricHistoryResult, error) {
	s.metricHistoryQuery = query
	if s.metricHistoryErr != nil {
		return appneighborhood.MetricHistoryResult{}, s.metricHistoryErr
	}
	if s.metricHistory.Items == nil {
		s.metricHistory.Items = []appneighborhood.MetricHistoryPoint{}
	}
	return s.metricHistory, nil
}

func (s *stubNeighborhoodApplication) AddWatchlistItem(_ context.Context, command appneighborhood.AddWatchlistItemCommand) (appneighborhood.WatchlistItem, error) {
	s.addCalled = true
	s.addCommand = command
	if s.addWatchlistItemErr != nil {
		return appneighborhood.WatchlistItem{}, s.addWatchlistItemErr
	}
	return s.addWatchlistItem, nil
}

func (s *stubNeighborhoodApplication) ListWatchlist(_ context.Context, _ appneighborhood.ListWatchlistQuery) ([]appneighborhood.WatchlistItemSummary, error) {
	if s.watchlistErr != nil {
		return nil, s.watchlistErr
	}
	return s.watchlist, nil
}
