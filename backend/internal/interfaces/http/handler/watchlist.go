package handler

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	appneighborhood "github.com/propulse/propulse/backend/internal/application/neighborhood"
	domainneighborhood "github.com/propulse/propulse/backend/internal/domain/neighborhood"
)

const demoUserID = "demo-user"

type Watchlist struct {
	app NeighborhoodApplication
}

func NewWatchlist(app NeighborhoodApplication) Watchlist {
	return Watchlist{app: app}
}

type addWatchlistItemRequest struct {
	NeighborhoodID string `json:"neighborhoodId"`
}

type addWatchlistItemResponse struct {
	ID             string `json:"id"`
	NeighborhoodID string `json:"neighborhoodId"`
	UserID         string `json:"userId"`
	CreatedAt      string `json:"createdAt"`
}

type watchlistResponse struct {
	Items []watchlistItemResponse `json:"items"`
}

type watchlistItemResponse struct {
	ID                  string                                 `json:"id"`
	NeighborhoodID      string                                 `json:"neighborhoodId"`
	Name                string                                 `json:"name"`
	Area                string                                 `json:"area"`
	TargetLayout        string                                 `json:"targetLayout"`
	Status              domainneighborhood.NeighborhoodStatus  `json:"status"`
	ListedHomes         int                                    `json:"listedHomes"`
	PriceCutHomes       int                                    `json:"priceCutHomes"`
	TransactionMomentum domainneighborhood.TransactionMomentum `json:"transactionMomentum"`
	Advice              string                                 `json:"advice"`
}

func (h Watchlist) AddItem(c *gin.Context) {
	var request addWatchlistItemRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", "request body is invalid")
		return
	}

	item, err := h.app.AddWatchlistItem(c.Request.Context(), appneighborhood.AddWatchlistItemCommand{
		UserID:         demoUserID,
		NeighborhoodID: request.NeighborhoodID,
	})
	if err != nil {
		if errors.Is(err, appneighborhood.ErrNeighborhoodNotFound) {
			writeError(c, http.StatusNotFound, "not_found", "neighborhood not found")
			return
		}
		writeError(c, http.StatusInternalServerError, "internal_error", "internal server error")
		return
	}

	c.JSON(http.StatusCreated, addWatchlistItemResponse{
		ID:             item.ID,
		NeighborhoodID: item.NeighborhoodID,
		UserID:         item.UserID,
		CreatedAt:      item.CreatedAt.UTC().Format(time.RFC3339),
	})
}

func (h Watchlist) List(c *gin.Context) {
	items, err := h.app.ListWatchlist(c.Request.Context(), appneighborhood.ListWatchlistQuery{UserID: demoUserID})
	if err != nil {
		writeError(c, http.StatusInternalServerError, "internal_error", "internal server error")
		return
	}

	response := watchlistResponse{Items: make([]watchlistItemResponse, 0, len(items))}
	for _, item := range items {
		response.Items = append(response.Items, watchlistItemResponse{
			ID:                  item.ID,
			NeighborhoodID:      item.NeighborhoodID,
			Name:                item.Name,
			Area:                item.Area,
			TargetLayout:        item.TargetLayout,
			Status:              item.Status,
			ListedHomes:         item.ListedHomes,
			PriceCutHomes:       item.PriceCutHomes,
			TransactionMomentum: item.TransactionMomentum,
			Advice:              item.Advice,
		})
	}

	c.JSON(http.StatusOK, response)
}
