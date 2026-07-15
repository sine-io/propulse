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
	MetricHistory(ctx context.Context, query appneighborhood.MetricHistoryQuery) (appneighborhood.MetricHistoryResult, error)
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
	Name             string   `json:"name"`
	City             string   `json:"city"`
	Area             string   `json:"area"`
	AvailableLayouts []string `json:"availableLayouts"`
}

type neighborhoodResponse struct {
	ID               string   `json:"id"`
	Name             string   `json:"name"`
	City             *string  `json:"city"`
	Area             string   `json:"area"`
	AvailableLayouts []string `json:"availableLayouts"`
	CreatedAt        string   `json:"createdAt,omitempty"`
}

type neighborhoodSearchResponse struct {
	Items    []neighborhoodResponse            `json:"items"`
	Total    int                               `json:"total"`
	Page     int                               `json:"page"`
	PageSize int                               `json:"pageSize"`
	Filters  neighborhoodSearchFiltersResponse `json:"filters"`
}

type neighborhoodSearchFiltersResponse struct {
	Cities []string                         `json:"cities"`
	Areas  []neighborhoodAreaFilterResponse `json:"areas"`
}

type neighborhoodAreaFilterResponse struct {
	City string `json:"city"`
	Area string `json:"area"`
}

type metricResponse struct {
	ID                       string                                 `json:"id"`
	NeighborhoodID           string                                 `json:"neighborhoodId"`
	CollectionRunID          string                                 `json:"collectionRunId"`
	InventoryCollectionRunID *string                                `json:"inventoryCollectionRunId,omitempty"`
	SourceIDs                []string                               `json:"sourceIds"`
	LatestObservedAt         string                                 `json:"latestObservedAt"`
	CollectedAt              string                                 `json:"collectedAt"`
	AlgorithmVersion         string                                 `json:"algorithmVersion"`
	ListedHomes              int                                    `json:"listedHomes"`
	PriceCutHomes            int                                    `json:"priceCutHomes"`
	AvgDaysOnMarket          *float64                               `json:"avgDaysOnMarket"`
	ListingPriceMin          *float64                               `json:"listingPriceMin"`
	ListingPriceMax          *float64                               `json:"listingPriceMax"`
	TransactionPriceMin      *float64                               `json:"transactionPriceMin"`
	TransactionPriceMax      *float64                               `json:"transactionPriceMax"`
	TransactionMomentum      domainneighborhood.TransactionMomentum `json:"transactionMomentum"`
	TransactionEvidence      *transactionMomentumEvidenceResponse   `json:"transactionEvidence"`
	TargetLayout             string                                 `json:"targetLayout"`
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

type transactionMomentumEvidenceResponse struct {
	WindowStart                       string  `json:"windowStart"`
	WindowEnd                         string  `json:"windowEnd"`
	SampleCount                       int     `json:"sampleCount"`
	RecentThirtyDayCount              int     `json:"recent30DayTransactionCount"`
	PrecedingSixtyDayCount            int     `json:"preceding60DayTransactionCount"`
	RecentThirtyDayMonthlyFrequency   float64 `json:"recent30DayMonthlyFrequency"`
	PrecedingSixtyDayMonthlyFrequency float64 `json:"preceding60DayMonthlyFrequency"`
}

type metricHistoryResponse struct {
	Status           appneighborhood.MetricHistoryStatus `json:"status"`
	NeighborhoodID   string                              `json:"neighborhoodId"`
	TargetLayout     string                              `json:"targetLayout"`
	AlgorithmVersion string                              `json:"algorithmVersion"`
	Window           metricHistoryWindowResponse         `json:"window"`
	Items            []metricHistoryPointResponse        `json:"items"`
}

type metricHistoryWindowResponse struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type collectionRunReferenceResponse struct {
	CollectionRunID string                      `json:"collectionRunId"`
	DataSourceID    string                      `json:"dataSourceId"`
	SourceRef       string                      `json:"sourceRef"`
	CollectedAt     string                      `json:"collectedAt"`
	Coverage        domainneighborhood.Coverage `json:"coverage"`
}

type metricChangeValueResponse struct {
	Current          int                                       `json:"current"`
	Baseline         int                                       `json:"baseline"`
	AbsoluteChange   int                                       `json:"absoluteChange"`
	PercentageChange *float64                                  `json:"percentageChange"`
	PercentageStatus domainneighborhood.PercentageChangeStatus `json:"percentageStatus"`
}

type metricComparisonResponse struct {
	Status                      domainneighborhood.MetricComparisonStatus `json:"status"`
	Reason                      domainneighborhood.MetricComparisonReason `json:"reason,omitempty"`
	CurrentBatch                collectionRunReferenceResponse            `json:"currentBatch"`
	BaselineBatch               *collectionRunReferenceResponse           `json:"baselineBatch,omitempty"`
	ListedHomes                 *metricChangeValueResponse                `json:"listedHomes,omitempty"`
	PriceCutHomes               *metricChangeValueResponse                `json:"priceCutHomes,omitempty"`
	RecentThirtyDayTransactions *metricChangeValueResponse                `json:"recent30DayTransactions,omitempty"`
}

type metricHistoryPointResponse struct {
	ID                     string                                 `json:"id"`
	NeighborhoodID         string                                 `json:"neighborhoodId"`
	AlgorithmVersion       string                                 `json:"algorithmVersion"`
	CollectedAt            string                                 `json:"collectedAt"`
	CalculatedAt           string                                 `json:"calculatedAt"`
	LatestObservedAt       string                                 `json:"latestObservedAt"`
	Batch                  collectionRunReferenceResponse         `json:"batch"`
	SourceIDs              []string                               `json:"sourceIds"`
	ListedHomes            int                                    `json:"listedHomes"`
	PriceCutHomes          int                                    `json:"priceCutHomes"`
	TransactionMomentum    domainneighborhood.TransactionMomentum `json:"transactionMomentum"`
	TransactionEvidence    *transactionMomentumEvidenceResponse   `json:"transactionEvidence"`
	TargetLayoutSupply     int                                    `json:"targetLayoutSupply"`
	ListingSampleCount     int                                    `json:"listingSampleCount"`
	TransactionSampleCount int                                    `json:"transactionSampleCount"`
	Coverage               domainneighborhood.Coverage            `json:"coverage"`
	Freshness              domainneighborhood.Freshness           `json:"freshness"`
	QualityState           domainneighborhood.MarketQualityState  `json:"qualityState"`
	QualityWarnings        []domainneighborhood.QualityWarning    `json:"qualityWarnings"`
	WeeklyComparison       metricComparisonResponse               `json:"weeklyComparison"`
	MonthlyComparison      metricComparisonResponse               `json:"monthlyComparison"`
}

func (h Neighborhood) CreateNeighborhood(c *gin.Context) {
	var request createNeighborhoodRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", "request body is invalid")
		return
	}
	request.Name = strings.TrimSpace(request.Name)
	request.City = strings.TrimSpace(request.City)
	request.Area = strings.TrimSpace(request.Area)

	neighborhood, err := h.app.CreateNeighborhood(c.Request.Context(), appneighborhood.CreateNeighborhoodCommand{
		Name:             request.Name,
		City:             request.City,
		Area:             request.Area,
		AvailableLayouts: request.AvailableLayouts,
	})
	if err != nil {
		if errors.Is(err, appneighborhood.ErrInvalidNeighborhood) {
			writeError(c, http.StatusBadRequest, "invalid_request", "name, city, area, and at least one available layout are required")
			return
		}
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
		City:         strings.TrimSpace(c.Query("city")),
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
		Filters:  newNeighborhoodSearchFiltersResponse(result.Filters),
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
	metric, err := h.app.LatestMetric(c.Request.Context(), appneighborhood.LatestMetricQuery{
		NeighborhoodID: c.Param("id"),
		TargetLayout:   strings.TrimSpace(c.Query("targetLayout")),
	})
	if err != nil {
		if errors.Is(err, appneighborhood.ErrInvalidTargetLayout) {
			writeError(c, http.StatusBadRequest, "invalid_target_layout", "targetLayout must belong to the neighborhood layout catalog")
			return
		}
		if errors.Is(err, appneighborhood.ErrMetricNotFound) || errors.Is(err, appneighborhood.ErrNeighborhoodNotFound) {
			writeError(c, http.StatusNotFound, "not_found", "neighborhood metric not found")
			return
		}
		writeError(c, http.StatusInternalServerError, "internal_error", "internal server error")
		return
	}

	c.JSON(http.StatusOK, newMetricResponse(metric))
}

func (h Neighborhood) GetMetricHistory(c *gin.Context) {
	from, err := parseOptionalRFC3339(c.Query("from"))
	if err != nil {
		writeError(c, http.StatusBadRequest, "invalid_time_window", "from must be an RFC3339 timestamp")
		return
	}
	to, err := parseOptionalRFC3339(c.Query("to"))
	if err != nil {
		writeError(c, http.StatusBadRequest, "invalid_time_window", "to must be an RFC3339 timestamp")
		return
	}
	result, err := h.app.MetricHistory(c.Request.Context(), appneighborhood.MetricHistoryQuery{
		NeighborhoodID: c.Param("id"),
		TargetLayout:   strings.TrimSpace(c.Query("targetLayout")),
		From:           from,
		To:             to,
	})
	if err != nil {
		if errors.Is(err, appneighborhood.ErrInvalidTargetLayout) {
			writeError(c, http.StatusBadRequest, "invalid_target_layout", "targetLayout must belong to the neighborhood layout catalog")
			return
		}
		if errors.Is(err, appneighborhood.ErrNeighborhoodNotFound) {
			writeError(c, http.StatusNotFound, "not_found", "neighborhood not found")
			return
		}
		if errors.Is(err, appneighborhood.ErrInvalidMetricHistoryWindow) {
			writeError(c, http.StatusBadRequest, "invalid_time_window", "metric history window must be ordered and no longer than 52 weeks")
			return
		}
		writeError(c, http.StatusInternalServerError, "internal_error", "internal server error")
		return
	}
	c.JSON(http.StatusOK, newMetricHistoryResponse(result))
}

func parseOptionalRFC3339(raw string) (time.Time, error) {
	if strings.TrimSpace(raw) == "" {
		return time.Time{}, nil
	}
	return time.Parse(time.RFC3339, strings.TrimSpace(raw))
}

func newNeighborhoodResponse(neighborhood appneighborhood.Neighborhood) neighborhoodResponse {
	response := neighborhoodResponse{
		ID:               neighborhood.ID,
		Name:             neighborhood.Name,
		City:             neighborhood.City,
		Area:             neighborhood.Area,
		AvailableLayouts: append([]string{}, neighborhood.AvailableLayouts...),
	}
	if !neighborhood.CreatedAt.IsZero() {
		response.CreatedAt = neighborhood.CreatedAt.UTC().Format(time.RFC3339)
	}
	return response
}

func newNeighborhoodSearchFiltersResponse(filters appneighborhood.NeighborhoodSearchFilters) neighborhoodSearchFiltersResponse {
	response := neighborhoodSearchFiltersResponse{
		Cities: append([]string{}, filters.Cities...),
		Areas:  make([]neighborhoodAreaFilterResponse, 0, len(filters.Areas)),
	}
	for _, area := range filters.Areas {
		response.Areas = append(response.Areas, neighborhoodAreaFilterResponse{City: area.City, Area: area.Area})
	}
	return response
}

func newMetricResponse(metric appneighborhood.MetricWithSignal) metricResponse {
	return metricResponse{
		ID:                       metric.Metric.ID,
		NeighborhoodID:           metric.Metric.NeighborhoodID,
		CollectionRunID:          metric.Metric.CollectionRunID,
		InventoryCollectionRunID: metric.Metric.InventoryCollectionRunID,
		SourceIDs:                append([]string{}, metric.Metric.SourceIDs...),
		LatestObservedAt:         metric.Metric.LatestObservedAt.UTC().Format(time.RFC3339),
		CollectedAt:              metric.Metric.CollectedAt.UTC().Format(time.RFC3339),
		AlgorithmVersion:         metric.Metric.AlgorithmVersion,
		ListedHomes:              metric.Metric.ListedHomes,
		PriceCutHomes:            metric.Metric.PriceCutHomes,
		AvgDaysOnMarket:          metric.Metric.AvgDaysOnMarket,
		ListingPriceMin:          metric.Metric.ListingPriceMin,
		ListingPriceMax:          metric.Metric.ListingPriceMax,
		TransactionPriceMin:      metric.Metric.TransactionPriceMin,
		TransactionPriceMax:      metric.Metric.TransactionPriceMax,
		TransactionMomentum:      metric.Metric.TransactionMomentum,
		TransactionEvidence:      newTransactionMomentumEvidenceResponse(metric.Metric.TransactionEvidence),
		TargetLayout:             metric.Metric.TargetLayout,
		TargetLayoutSupply:       metric.Metric.TargetLayoutSupply,
		ListingSampleCount:       metric.Metric.ListingSampleCount,
		TransactionSampleCount:   metric.Metric.TransactionSampleCount,
		Coverage:                 metric.Metric.Coverage,
		Freshness:                metric.Metric.Freshness,
		QualityState:             metric.Metric.QualityState,
		QualityWarnings:          append([]domainneighborhood.QualityWarning{}, metric.Metric.QualityWarnings...),
		Status:                   metric.Signal.Status,
		SupplyPressure:           metric.Signal.SupplyPressure,
		PriceCutShare:            metric.Signal.PriceCutShare,
		PriceGapPct:              metric.Signal.PriceGapPct,
		TargetLayoutScarcity:     metric.Signal.TargetLayoutScarcity,
		Advice:                   metric.Signal.NextAction,
		Reasons:                  append([]string{}, metric.Signal.Reasons...),
		CalculatedAt:             metric.Metric.CalculatedAt.UTC().Format(time.RFC3339),
	}
}

func newTransactionMomentumEvidenceResponse(evidence *domainneighborhood.TransactionMomentumEvidence) *transactionMomentumEvidenceResponse {
	if evidence == nil {
		return nil
	}
	return &transactionMomentumEvidenceResponse{
		WindowStart:                       evidence.WindowStart.Format(time.DateOnly),
		WindowEnd:                         evidence.WindowEnd.Format(time.DateOnly),
		SampleCount:                       evidence.SampleCount,
		RecentThirtyDayCount:              evidence.RecentThirtyDayCount,
		PrecedingSixtyDayCount:            evidence.PrecedingSixtyDayCount,
		RecentThirtyDayMonthlyFrequency:   evidence.RecentThirtyDayMonthlyFrequency,
		PrecedingSixtyDayMonthlyFrequency: evidence.PrecedingSixtyDayMonthlyFrequency,
	}
}

func newMetricHistoryResponse(result appneighborhood.MetricHistoryResult) metricHistoryResponse {
	response := metricHistoryResponse{
		Status:           result.Status,
		NeighborhoodID:   result.NeighborhoodID,
		TargetLayout:     result.TargetLayout,
		AlgorithmVersion: result.AlgorithmVersion,
		Window: metricHistoryWindowResponse{
			From: result.From.UTC().Format(time.RFC3339),
			To:   result.To.UTC().Format(time.RFC3339),
		},
		Items: make([]metricHistoryPointResponse, 0, len(result.Items)),
	}
	for _, point := range result.Items {
		response.Items = append(response.Items, metricHistoryPointResponse{
			ID:                     point.Metric.ID,
			NeighborhoodID:         point.Metric.NeighborhoodID,
			AlgorithmVersion:       point.Metric.AlgorithmVersion,
			CollectedAt:            point.Batch.CollectedAt.UTC().Format(time.RFC3339),
			CalculatedAt:           point.Metric.CalculatedAt.UTC().Format(time.RFC3339),
			LatestObservedAt:       point.Metric.LatestObservedAt.UTC().Format(time.RFC3339),
			Batch:                  newCollectionRunReferenceResponse(point.Batch),
			SourceIDs:              append([]string{}, point.Metric.SourceIDs...),
			ListedHomes:            point.Metric.ListedHomes,
			PriceCutHomes:          point.Metric.PriceCutHomes,
			TransactionMomentum:    point.Metric.TransactionMomentum,
			TransactionEvidence:    newTransactionMomentumEvidenceResponse(point.Metric.TransactionEvidence),
			TargetLayoutSupply:     point.Metric.TargetLayoutSupply,
			ListingSampleCount:     point.Metric.ListingSampleCount,
			TransactionSampleCount: point.Metric.TransactionSampleCount,
			Coverage:               point.Metric.Coverage,
			Freshness:              point.Metric.Freshness,
			QualityState:           point.Metric.QualityState,
			QualityWarnings:        append([]domainneighborhood.QualityWarning{}, point.Metric.QualityWarnings...),
			WeeklyComparison:       newMetricComparisonResponse(point.WeeklyComparison),
			MonthlyComparison:      newMetricComparisonResponse(point.MonthlyComparison),
		})
	}
	return response
}

func newCollectionRunReferenceResponse(reference appneighborhood.CollectionRunReference) collectionRunReferenceResponse {
	return collectionRunReferenceResponse{
		CollectionRunID: reference.CollectionRunID,
		DataSourceID:    reference.DataSourceID,
		SourceRef:       reference.SourceRef,
		CollectedAt:     reference.CollectedAt.UTC().Format(time.RFC3339),
		Coverage:        reference.Coverage,
	}
}

func newMetricComparisonResponse(comparison appneighborhood.MetricComparison) metricComparisonResponse {
	response := metricComparisonResponse{
		Status:       comparison.Status,
		Reason:       comparison.Reason,
		CurrentBatch: newCollectionRunReferenceResponse(comparison.CurrentBatch),
	}
	if comparison.BaselineBatch != nil {
		baseline := newCollectionRunReferenceResponse(*comparison.BaselineBatch)
		response.BaselineBatch = &baseline
	}
	response.ListedHomes = newMetricChangeValueResponse(comparison.ListedHomes)
	response.PriceCutHomes = newMetricChangeValueResponse(comparison.PriceCutHomes)
	response.RecentThirtyDayTransactions = newMetricChangeValueResponse(comparison.RecentThirtyDayTransactions)
	return response
}

func newMetricChangeValueResponse(change *domainneighborhood.MetricChangeValue) *metricChangeValueResponse {
	if change == nil {
		return nil
	}
	return &metricChangeValueResponse{
		Current:          change.Current,
		Baseline:         change.Baseline,
		AbsoluteChange:   change.AbsoluteChange,
		PercentageChange: change.PercentageChange,
		PercentageStatus: change.PercentageStatus,
	}
}
