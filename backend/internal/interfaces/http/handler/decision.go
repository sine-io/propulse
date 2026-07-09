package handler

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	appdecision "github.com/propulse/propulse/backend/internal/application/decision"
	domaindecision "github.com/propulse/propulse/backend/internal/domain/decision"
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
		writeError(c, http.StatusInternalServerError, "internal_error", "internal server error")
		return
	}

	c.JSON(http.StatusOK, result)
}
