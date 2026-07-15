package handler

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	appneighborhood "github.com/sine-io/propulse/internal/application/neighborhood"
	domainneighborhood "github.com/sine-io/propulse/internal/domain/neighborhood"
)

type Watchlist struct {
	app    NeighborhoodApplication
	userID string
}

func NewWatchlist(app NeighborhoodApplication, userID string) Watchlist {
	return Watchlist{app: app, userID: userID}
}

type addWatchlistItemRequest struct {
	NeighborhoodID string `json:"neighborhoodId"`
	TargetLayout   string `json:"targetLayout"`
}

type addWatchlistItemResponse struct {
	ID             string `json:"id"`
	NeighborhoodID string `json:"neighborhoodId"`
	TargetLayout   string `json:"targetLayout"`
	UserID         string `json:"userId"`
	CreatedAt      string `json:"createdAt"`
}

type watchlistResponse struct {
	Items []watchlistItemResponse `json:"items"`
}

type watchlistItemResponse struct {
	ID                     string                                  `json:"id"`
	NeighborhoodID         string                                  `json:"neighborhoodId"`
	Name                   string                                  `json:"name"`
	City                   *string                                 `json:"city"`
	Area                   string                                  `json:"area"`
	TargetLayout           string                                  `json:"targetLayout"`
	Status                 domainneighborhood.NeighborhoodStatus   `json:"status"`
	ListedHomes            *int                                    `json:"listedHomes"`
	PriceCutHomes          *int                                    `json:"priceCutHomes"`
	TransactionMomentum    *domainneighborhood.TransactionMomentum `json:"transactionMomentum"`
	TargetLayoutSupply     *int                                    `json:"targetLayoutSupply"`
	TargetLayoutScarcity   *domainneighborhood.Scarcity            `json:"targetLayoutScarcity"`
	Advice                 string                                  `json:"advice"`
	HasMetric              bool                                    `json:"hasMetric"`
	CollectionRunID        string                                  `json:"collectionRunId,omitempty"`
	AlgorithmVersion       string                                  `json:"algorithmVersion,omitempty"`
	SourceIDs              []string                                `json:"sourceIds"`
	CollectedAt            *string                                 `json:"collectedAt"`
	TransactionSampleCount *int                                    `json:"transactionSampleCount"`
	Coverage               domainneighborhood.Coverage             `json:"coverage"`
	Freshness              domainneighborhood.Freshness            `json:"freshness"`
	QualityState           domainneighborhood.MarketQualityState   `json:"qualityState"`
	QualityWarnings        []domainneighborhood.QualityWarning     `json:"qualityWarnings"`
	WeeklyComparison       *metricComparisonResponse               `json:"weeklyComparison"`
}

func (h Watchlist) AddItem(c *gin.Context) {
	var request addWatchlistItemRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", "request body is invalid")
		return
	}
	request.NeighborhoodID = strings.TrimSpace(request.NeighborhoodID)
	request.TargetLayout = strings.TrimSpace(request.TargetLayout)

	item, err := h.app.AddWatchlistItem(c.Request.Context(), appneighborhood.AddWatchlistItemCommand{
		UserID:         h.userID,
		NeighborhoodID: request.NeighborhoodID,
		TargetLayout:   request.TargetLayout,
	})
	if err != nil {
		if errors.Is(err, appneighborhood.ErrInvalidNeighborhoodID) {
			writeError(c, http.StatusBadRequest, "invalid_neighborhood_id", "neighborhoodId must be a valid UUID")
			return
		}
		if errors.Is(err, appneighborhood.ErrInvalidTargetLayout) {
			writeError(c, http.StatusBadRequest, "invalid_target_layout", "targetLayout must belong to the neighborhood layout catalog")
			return
		}
		if errors.Is(err, appneighborhood.ErrNeighborhoodNotFound) {
			writeError(c, http.StatusNotFound, "not_found", "neighborhood not found")
			return
		}
		if errors.Is(err, appneighborhood.ErrWatchlistItemExists) {
			writeError(c, http.StatusConflict, "watchlist_item_exists", "the neighborhood is already on the watchlist")
			return
		}
		writeError(c, http.StatusInternalServerError, "internal_error", "internal server error")
		return
	}

	c.JSON(http.StatusCreated, addWatchlistItemResponse{
		ID:             item.ID,
		NeighborhoodID: item.NeighborhoodID,
		TargetLayout:   item.TargetLayout,
		UserID:         item.UserID,
		CreatedAt:      item.CreatedAt.UTC().Format(time.RFC3339),
	})
}

func (h Watchlist) List(c *gin.Context) {
	items, err := h.app.ListWatchlist(c.Request.Context(), appneighborhood.ListWatchlistQuery{UserID: h.userID})
	if err != nil {
		writeError(c, http.StatusInternalServerError, "internal_error", "internal server error")
		return
	}

	response := watchlistResponse{Items: make([]watchlistItemResponse, 0, len(items))}
	for _, item := range items {
		var collectedAt *string
		var listedHomes, priceCutHomes, transactionSampleCount *int
		var transactionMomentum *domainneighborhood.TransactionMomentum
		var targetLayoutSupply *int
		var targetLayoutScarcity *domainneighborhood.Scarcity
		var weeklyComparison *metricComparisonResponse
		if item.CollectedAt != nil {
			formatted := item.CollectedAt.UTC().Format(time.RFC3339)
			collectedAt = &formatted
		}
		if item.HasMetric {
			listedHomes = handlerIntPtr(item.ListedHomes)
			priceCutHomes = handlerIntPtr(item.PriceCutHomes)
			transactionMomentum = &item.TransactionMomentum
			targetLayoutSupply = handlerIntPtr(item.TargetLayoutSupply)
			targetLayoutScarcity = &item.TargetLayoutScarcity
			transactionSampleCount = handlerIntPtr(item.TransactionSampleCount)
			if item.WeeklyComparison != nil {
				comparison := newMetricComparisonResponse(*item.WeeklyComparison)
				weeklyComparison = &comparison
			}
		}
		response.Items = append(response.Items, watchlistItemResponse{
			ID:                     item.ID,
			NeighborhoodID:         item.NeighborhoodID,
			Name:                   item.Name,
			City:                   item.City,
			Area:                   item.Area,
			TargetLayout:           item.TargetLayout,
			Status:                 item.Status,
			ListedHomes:            listedHomes,
			PriceCutHomes:          priceCutHomes,
			TransactionMomentum:    transactionMomentum,
			TargetLayoutSupply:     targetLayoutSupply,
			TargetLayoutScarcity:   targetLayoutScarcity,
			Advice:                 item.Advice,
			HasMetric:              item.HasMetric,
			CollectionRunID:        item.CollectionRunID,
			AlgorithmVersion:       item.AlgorithmVersion,
			SourceIDs:              append([]string{}, item.SourceIDs...),
			CollectedAt:            collectedAt,
			TransactionSampleCount: transactionSampleCount,
			Coverage:               item.Coverage,
			Freshness:              item.Freshness,
			QualityState:           item.QualityState,
			QualityWarnings:        append([]domainneighborhood.QualityWarning{}, item.QualityWarnings...),
			WeeklyComparison:       weeklyComparison,
		})
	}

	c.JSON(http.StatusOK, response)
}

func handlerIntPtr(value int) *int { return &value }
