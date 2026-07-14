package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	appneighborhood "github.com/sine-io/propulse/internal/application/neighborhood"
	domainneighborhood "github.com/sine-io/propulse/internal/domain/neighborhood"
)

type NeighborhoodApplication interface {
	CreateNeighborhood(ctx context.Context, command appneighborhood.CreateNeighborhoodCommand) (appneighborhood.Neighborhood, error)
	GetNeighborhood(ctx context.Context, query appneighborhood.GetNeighborhoodQuery) (appneighborhood.Neighborhood, error)
	SearchNeighborhoods(ctx context.Context, query appneighborhood.SearchNeighborhoodsQuery) (appneighborhood.SearchNeighborhoodsPage, error)
	LatestMetric(ctx context.Context, query appneighborhood.LatestMetricQuery) (appneighborhood.MetricWithSignal, error)
	AddWatchlistItem(ctx context.Context, command appneighborhood.AddWatchlistItemCommand) (appneighborhood.WatchlistItem, error)
	ListWatchlist(ctx context.Context, query appneighborhood.ListWatchlistQuery) ([]appneighborhood.WatchlistItemSummary, error)
}

type Neighborhood struct {
	app NeighborhoodApplication
}

func NewNeighborhood(app NeighborhoodApplication) Neighborhood {
	return Neighborhood{app: app}
}

type createNeighborhoodRequest struct {
	Name         string `json:"name"`
	Area         string `json:"area"`
	TargetLayout string `json:"targetLayout"`
}

type neighborhoodResponse struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Area         string `json:"area"`
	TargetLayout string `json:"targetLayout"`
	CreatedAt    string `json:"createdAt,omitempty"`
}

type neighborhoodSearchResponse struct {
	Items    []neighborhoodResponse `json:"items"`
	Total    int                    `json:"total"`
	Page     int                    `json:"page"`
	PageSize int                    `json:"pageSize"`
}

type metricResponse struct {
	ID                       string                                 `json:"id"`
	NeighborhoodID           string                                 `json:"neighborhoodId"`
	CollectionRunID          string                                 `json:"collectionRunId"`
	InventoryCollectionRunID *string                                `json:"inventoryCollectionRunId,omitempty"`
	SourceIDs                []string                               `json:"sourceIds"`
	LatestObservedAt         string                                 `json:"latestObservedAt"`
	ListedHomes              int                                    `json:"listedHomes"`
	PriceCutHomes            int                                    `json:"priceCutHomes"`
	AvgDaysOnMarket          *float64                               `json:"avgDaysOnMarket"`
	ListingPriceMin          *float64                               `json:"listingPriceMin"`
	ListingPriceMax          *float64                               `json:"listingPriceMax"`
	TransactionPriceMin      *float64                               `json:"transactionPriceMin"`
	TransactionPriceMax      *float64                               `json:"transactionPriceMax"`
	TransactionMomentum      domainneighborhood.TransactionMomentum `json:"transactionMomentum"`
	TargetLayoutSupply       int                                    `json:"targetLayoutSupply"`
	ListingSampleCount       int                                    `json:"listingSampleCount"`
	TransactionSampleCount   int                                    `json:"transactionSampleCount"`
	Coverage                 domainneighborhood.Coverage            `json:"coverage"`
	Freshness                domainneighborhood.Freshness           `json:"freshness"`
	QualityState             domainneighborhood.MarketQualityState  `json:"qualityState"`
	QualityWarnings          []domainneighborhood.QualityWarning    `json:"qualityWarnings"`
	Status                   domainneighborhood.NeighborhoodStatus  `json:"status"`
	SupplyPressure           domainneighborhood.SupplyPressure      `json:"supplyPressure"`
	PriceCutShare            float64                                `json:"priceCutShare"`
	PriceGapPct              float64                                `json:"priceGapPct"`
	TargetLayoutScarcity     domainneighborhood.Scarcity            `json:"targetLayoutScarcity"`
	Advice                   string                                 `json:"advice"`
	Reasons                  []string                               `json:"reasons"`
	CalculatedAt             string                                 `json:"calculatedAt"`
}

func (h Neighborhood) CreateNeighborhood(c *gin.Context) {
	var request createNeighborhoodRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", "request body is invalid")
		return
	}
	request.Name = strings.TrimSpace(request.Name)
	request.Area = strings.TrimSpace(request.Area)
	request.TargetLayout = strings.TrimSpace(request.TargetLayout)
	if request.Name == "" || request.Area == "" || request.TargetLayout == "" {
		writeError(c, http.StatusBadRequest, "invalid_request", "name, area, and targetLayout are required")
		return
	}

	neighborhood, err := h.app.CreateNeighborhood(c.Request.Context(), appneighborhood.CreateNeighborhoodCommand{
		Name:         request.Name,
		Area:         request.Area,
		TargetLayout: request.TargetLayout,
	})
	if err != nil {
		writeError(c, http.StatusInternalServerError, "internal_error", "internal server error")
		return
	}

	c.JSON(http.StatusCreated, newNeighborhoodResponse(neighborhood))
}

func (h Neighborhood) SearchNeighborhoods(c *gin.Context) {
	page, err := parsePositiveIntQuery(c.Query("page"))
	if err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", "page must be a positive integer")
		return
	}
	pageSize, err := parsePositiveIntQuery(c.Query("pageSize"))
	if err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", "pageSize must be a positive integer")
		return
	}

	result, err := h.app.SearchNeighborhoods(c.Request.Context(), appneighborhood.SearchNeighborhoodsQuery{
		Query:        strings.TrimSpace(c.Query("q")),
		Area:         strings.TrimSpace(c.Query("area")),
		TargetLayout: strings.TrimSpace(c.Query("targetLayout")),
		Page:         page,
		PageSize:     pageSize,
	})
	if err != nil {
		writeError(c, http.StatusInternalServerError, "internal_error", "internal server error")
		return
	}

	items := make([]neighborhoodResponse, 0, len(result.Items))
	for _, item := range result.Items {
		items = append(items, newNeighborhoodResponse(item))
	}
	c.JSON(http.StatusOK, neighborhoodSearchResponse{
		Items:    items,
		Total:    result.Total,
		Page:     result.Page,
		PageSize: result.PageSize,
	})
}

// parsePositiveIntQuery 解析可选的正整数查询参数；空串返回 0（交由 service 取默认）。
func parsePositiveIntQuery(raw string) (int, error) {
	if raw == "" {
		return 0, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 1 {
		return 0, errors.New("invalid positive integer")
	}
	return value, nil
}

func (h Neighborhood) GetNeighborhood(c *gin.Context) {
	neighborhood, err := h.app.GetNeighborhood(c.Request.Context(), appneighborhood.GetNeighborhoodQuery{ID: c.Param("id")})
	if err != nil {
		if errors.Is(err, appneighborhood.ErrNeighborhoodNotFound) {
			writeError(c, http.StatusNotFound, "not_found", "neighborhood not found")
			return
		}
		writeError(c, http.StatusInternalServerError, "internal_error", "internal server error")
		return
	}

	c.JSON(http.StatusOK, newNeighborhoodResponse(neighborhood))
}

func (h Neighborhood) GetMetrics(c *gin.Context) {
	metric, err := h.app.LatestMetric(c.Request.Context(), appneighborhood.LatestMetricQuery{NeighborhoodID: c.Param("id")})
	if err != nil {
		if errors.Is(err, appneighborhood.ErrMetricNotFound) || errors.Is(err, appneighborhood.ErrNeighborhoodNotFound) {
			writeError(c, http.StatusNotFound, "not_found", "neighborhood metric not found")
			return
		}
		writeError(c, http.StatusInternalServerError, "internal_error", "internal server error")
		return
	}

	c.JSON(http.StatusOK, newMetricResponse(metric))
}

func newNeighborhoodResponse(neighborhood appneighborhood.Neighborhood) neighborhoodResponse {
	response := neighborhoodResponse{
		ID:           neighborhood.ID,
		Name:         neighborhood.Name,
		Area:         neighborhood.Area,
		TargetLayout: neighborhood.TargetLayout,
	}
	if !neighborhood.CreatedAt.IsZero() {
		response.CreatedAt = neighborhood.CreatedAt.UTC().Format(time.RFC3339)
	}
	return response
}

func newMetricResponse(metric appneighborhood.MetricWithSignal) metricResponse {
	return metricResponse{
		ID:                       metric.Metric.ID,
		NeighborhoodID:           metric.Metric.NeighborhoodID,
		CollectionRunID:          metric.Metric.CollectionRunID,
		InventoryCollectionRunID: metric.Metric.InventoryCollectionRunID,
		SourceIDs:                metric.Metric.SourceIDs,
		LatestObservedAt:         metric.Metric.LatestObservedAt.UTC().Format(time.RFC3339),
		ListedHomes:              metric.Metric.ListedHomes,
		PriceCutHomes:            metric.Metric.PriceCutHomes,
		AvgDaysOnMarket:          metric.Metric.AvgDaysOnMarket,
		ListingPriceMin:          metric.Metric.ListingPriceMin,
		ListingPriceMax:          metric.Metric.ListingPriceMax,
		TransactionPriceMin:      metric.Metric.TransactionPriceMin,
		TransactionPriceMax:      metric.Metric.TransactionPriceMax,
		TransactionMomentum:      metric.Metric.TransactionMomentum,
		TargetLayoutSupply:       metric.Metric.TargetLayoutSupply,
		ListingSampleCount:       metric.Metric.ListingSampleCount,
		TransactionSampleCount:   metric.Metric.TransactionSampleCount,
		Coverage:                 metric.Metric.Coverage,
		Freshness:                metric.Metric.Freshness,
		QualityState:             metric.Metric.QualityState,
		QualityWarnings:          metric.Metric.QualityWarnings,
		Status:                   metric.Signal.Status,
		SupplyPressure:           metric.Signal.SupplyPressure,
		PriceCutShare:            metric.Signal.PriceCutShare,
		PriceGapPct:              metric.Signal.PriceGapPct,
		TargetLayoutScarcity:     metric.Signal.TargetLayoutScarcity,
		Advice:                   metric.Signal.NextAction,
		Reasons:                  metric.Signal.Reasons,
		CalculatedAt:             metric.Metric.CalculatedAt.UTC().Format(time.RFC3339),
	}
}
