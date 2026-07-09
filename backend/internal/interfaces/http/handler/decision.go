package handler

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	appdecision "github.com/sine-io/propulse/backend/internal/application/decision"
	domaindecision "github.com/sine-io/propulse/backend/internal/domain/decision"
)

type DecisionApplication interface {
	GetActionWindow(ctx context.Context, query appdecision.GetActionWindowQuery) (domaindecision.ActionWindowResult, error)
}

type Decision struct {
	app DecisionApplication
}

func NewDecision(app DecisionApplication) Decision {
	return Decision{app: app}
}

func (h Decision) GetActionWindow(c *gin.Context) {
	result, err := h.app.GetActionWindow(c.Request.Context(), appdecision.GetActionWindowQuery{
		NeighborhoodID: c.Query("neighborhoodId"),
	})
	if err != nil {
		if errors.Is(err, appdecision.ErrCapacityRequired) {
			writeError(c, http.StatusBadRequest, "capacity_required", "create a capacity calculation before requesting an action window")
			return
		}
		if errors.Is(err, appdecision.ErrWatchlistRequired) {
			writeError(c, http.StatusBadRequest, "watchlist_required", "add a neighborhood to the watchlist before requesting an action window")
			return
		}
		if errors.Is(err, appdecision.ErrInvalidNeighborhoodID) {
			writeError(c, http.StatusBadRequest, "invalid_neighborhood_id", "neighborhoodId must be a valid UUID")
			return
		}
		if errors.Is(err, appdecision.ErrMetricRequired) {
			writeError(c, http.StatusNotFound, "metric_required", "no neighborhood metric is available")
			return
		}
		writeError(c, http.StatusInternalServerError, "internal_error", "internal server error")
		return
	}

	c.JSON(http.StatusOK, result)
}
