package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	appneighborhood "github.com/sine-io/propulse/internal/application/neighborhood"
	"github.com/sine-io/propulse/internal/application/user"
	domainneighborhood "github.com/sine-io/propulse/internal/domain/neighborhood"
)

func TestCreateNeighborhoodReturnsCreatedNeighborhood(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	service := &stubNeighborhoodApplication{
		createNeighborhood: appneighborhood.Neighborhood{
			ID:           "neighborhood_1",
			Name:         "青枫花园",
			Area:         "滨江核心",
			TargetLayout: "三房",
			CreatedAt:    time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC),
		},
	}
	engine := gin.New()
	engine.POST("/api/v1/neighborhoods", NewNeighborhood(service).CreateNeighborhood)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/neighborhoods", bytes.NewBufferString(`{"name":"青枫花园","area":"滨江核心","targetLayout":"三房"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201", rec.Code)
	}

	var response struct {
		ID           string `json:"id"`
		Name         string `json:"name"`
		Area         string `json:"area"`
		TargetLayout string `json:"targetLayout"`
		CreatedAt    string `json:"createdAt"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if response.ID != "neighborhood_1" || response.Name != "青枫花园" || response.TargetLayout != "三房" {
		t.Fatalf("response = %#v", response)
	}
	if response.CreatedAt != "2026-07-09T12:00:00Z" {
		t.Fatalf("CreatedAt = %q", response.CreatedAt)
	}
}

func TestCreateNeighborhoodRejectsMissingRequiredFields(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	service := &stubNeighborhoodApplication{}
	engine := gin.New()
	engine.POST("/api/v1/neighborhoods", NewNeighborhood(service).CreateNeighborhood)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/neighborhoods", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if service.createCalled {
		t.Fatal("CreateNeighborhood was called for invalid request")
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
			ID:           "neighborhood_1",
			Name:         "青枫花园",
			Area:         "滨江核心",
			TargetLayout: "三房",
			CreatedAt:    time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC),
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
	service := &stubNeighborhoodApplication{
		latestMetric: appneighborhood.MetricWithSignal{
			Metric: appneighborhood.MetricSnapshot{
				ID:                  "metric_1",
				NeighborhoodID:      "neighborhood_1",
				ListedHomes:         42,
				PriceCutHomes:       11,
				AvgDaysOnMarket:     78,
				ListingPriceMin:     520,
				ListingPriceMax:     620,
				TransactionPriceMin: 495,
				TransactionPriceMax: 545,
				TransactionMomentum: domainneighborhood.TransactionMomentumWeak,
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

	req := httptest.NewRequest(http.MethodGet, "/api/v1/neighborhoods/neighborhood_1/metrics", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var response struct {
		Status         string `json:"status"`
		SupplyPressure string `json:"supplyPressure"`
		Advice         string `json:"advice"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if response.Status != "适合砍价" || response.SupplyPressure != "high" {
		t.Fatalf("response = %#v", response)
	}
}

func TestCreateWatchlistItemUsesSingleUser(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	service := &stubNeighborhoodApplication{
		addWatchlistItem: appneighborhood.WatchlistItem{
			ID:             "watch_1",
			UserID:         user.SingleUserID,
			NeighborhoodID: "neighborhood_1",
			CreatedAt:      time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC),
		},
	}
	engine := gin.New()
	engine.POST("/api/v1/watchlist/items", NewWatchlist(service).AddItem)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/watchlist/items", bytes.NewBufferString(`{"neighborhoodId":"neighborhood_1"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201", rec.Code)
	}
	if service.addCommand.UserID != user.SingleUserID {
		t.Fatalf("UserID = %q, want %q", service.addCommand.UserID, user.SingleUserID)
	}
}

func TestCreateWatchlistItemRejectsMissingNeighborhoodID(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	service := &stubNeighborhoodApplication{}
	engine := gin.New()
	engine.POST("/api/v1/watchlist/items", NewWatchlist(service).AddItem)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/watchlist/items", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if service.addCalled {
		t.Fatal("AddWatchlistItem was called for invalid request")
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

func TestListWatchlistReturnsBriefShape(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	service := &stubNeighborhoodApplication{
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
				Advice:              "约看 500-530 万三房，尝试砍价，窗口期已打开。",
			},
		},
	}
	engine := gin.New()
	engine.GET("/api/v1/watchlist", NewWatchlist(service).List)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/watchlist", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var response struct {
		Items []struct {
			ID                  string `json:"id"`
			NeighborhoodID      string `json:"neighborhoodId"`
			Name                string `json:"name"`
			Area                string `json:"area"`
			TargetLayout        string `json:"targetLayout"`
			Status              string `json:"status"`
			ListedHomes         int    `json:"listedHomes"`
			PriceCutHomes       int    `json:"priceCutHomes"`
			TransactionMomentum string `json:"transactionMomentum"`
			Advice              string `json:"advice"`
		} `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if len(response.Items) != 1 {
		t.Fatalf("items = %d, want 1", len(response.Items))
	}
	if response.Items[0].NeighborhoodID != "neighborhood_1" || response.Items[0].Status != "适合砍价" {
		t.Fatalf("item = %#v", response.Items[0])
	}
}

func TestListWatchlistReturnsNeutralSummaryWithoutMetric(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	service := &stubNeighborhoodApplication{
		watchlist: []appneighborhood.WatchlistItemSummary{
			{
				ID:             "watch_1",
				NeighborhoodID: "neighborhood_1",
				Name:           "青枫花园",
				Area:           "滨江核心",
				TargetLayout:   "三房",
				Status:         domainneighborhood.NeighborhoodStatusObserve,
				Advice:         "暂无指标数据，等待导入或计算后再判断。",
			},
		},
	}
	engine := gin.New()
	engine.GET("/api/v1/watchlist", NewWatchlist(service).List)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/watchlist", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var response struct {
		Items []struct {
			Status              string `json:"status"`
			ListedHomes         int    `json:"listedHomes"`
			PriceCutHomes       int    `json:"priceCutHomes"`
			TransactionMomentum string `json:"transactionMomentum"`
			Advice              string `json:"advice"`
		} `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if len(response.Items) != 1 {
		t.Fatalf("items = %d, want 1", len(response.Items))
	}
	item := response.Items[0]
	if item.Status != "继续观察" || item.Advice != "暂无指标数据，等待导入或计算后再判断。" {
		t.Fatalf("item = %#v", item)
	}
	if item.ListedHomes != 0 || item.PriceCutHomes != 0 || item.TransactionMomentum != "" {
		t.Fatalf("metric fields = listed %d, price cuts %d, momentum %q", item.ListedHomes, item.PriceCutHomes, item.TransactionMomentum)
	}
}

type stubNeighborhoodApplication struct {
	createNeighborhood    appneighborhood.Neighborhood
	createNeighborhoodErr error
	createCalled          bool
	getNeighborhood       appneighborhood.Neighborhood
	getNeighborhoodErr    error
	latestMetric          appneighborhood.MetricWithSignal
	latestMetricErr       error
	addWatchlistItem      appneighborhood.WatchlistItem
	addWatchlistItemErr   error
	addCommand            appneighborhood.AddWatchlistItemCommand
	addCalled             bool
	watchlist             []appneighborhood.WatchlistItemSummary
	watchlistErr          error
}

func (s *stubNeighborhoodApplication) CreateNeighborhood(_ context.Context, command appneighborhood.CreateNeighborhoodCommand) (appneighborhood.Neighborhood, error) {
	s.createCalled = true
	return s.createNeighborhood, s.createNeighborhoodErr
}

func (s *stubNeighborhoodApplication) GetNeighborhood(_ context.Context, query appneighborhood.GetNeighborhoodQuery) (appneighborhood.Neighborhood, error) {
	if s.getNeighborhoodErr != nil {
		return appneighborhood.Neighborhood{}, s.getNeighborhoodErr
	}
	return s.getNeighborhood, nil
}

func (s *stubNeighborhoodApplication) LatestMetric(_ context.Context, query appneighborhood.LatestMetricQuery) (appneighborhood.MetricWithSignal, error) {
	if s.latestMetricErr != nil {
		return appneighborhood.MetricWithSignal{}, s.latestMetricErr
	}
	return s.latestMetric, nil
}

func (s *stubNeighborhoodApplication) AddWatchlistItem(_ context.Context, command appneighborhood.AddWatchlistItemCommand) (appneighborhood.WatchlistItem, error) {
	s.addCalled = true
	s.addCommand = command
	if s.addWatchlistItemErr != nil {
		return appneighborhood.WatchlistItem{}, s.addWatchlistItemErr
	}
	return s.addWatchlistItem, nil
}

func (s *stubNeighborhoodApplication) ListWatchlist(_ context.Context, query appneighborhood.ListWatchlistQuery) ([]appneighborhood.WatchlistItemSummary, error) {
	if s.watchlistErr != nil {
		return nil, s.watchlistErr
	}
	return s.watchlist, nil
}

var errNeighborhoodBoom = errors.New("boom")
