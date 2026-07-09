package handler

import (
	"context"
	"errors"
	"math"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	appcapacity "github.com/sine-io/propulse/backend/internal/application/capacity"
	domaincapacity "github.com/sine-io/propulse/backend/internal/domain/capacity"
)

type CapacityApplication interface {
	CreateCalculation(ctx context.Context, command appcapacity.CreateCalculationCommand) (appcapacity.CalculationRecord, error)
	GetCalculation(ctx context.Context, query appcapacity.GetCalculationQuery) (appcapacity.CalculationRecord, error)
	LatestCalculation(ctx context.Context, query appcapacity.LatestCalculationQuery) (appcapacity.CalculationRecord, error)
}

type Capacity struct {
	app CapacityApplication
}

func NewCapacity(app CapacityApplication) Capacity {
	return Capacity{app: app}
}

type createCalculationResponse struct {
	ID     string                  `json:"id"`
	Result createCalculationResult `json:"result"`
}

type createCalculationResult struct {
	PressureLevel domaincapacity.PressureLevel `json:"pressureLevel"`
	Strategy      string                       `json:"strategy"`
}

type calculationResponse struct {
	ID        string                               `json:"id"`
	Input     domaincapacity.HousingCapacityInput  `json:"input"`
	Result    domaincapacity.HousingCapacityResult `json:"result"`
	CreatedAt string                               `json:"createdAt"`
}

type errorResponse struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func (h Capacity) CreateCalculation(c *gin.Context) {
	var input domaincapacity.HousingCapacityInput
	if err := c.ShouldBindJSON(&input); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", "request body is invalid")
		return
	}
	if !validCapacityInput(input) {
		writeError(c, http.StatusBadRequest, "invalid_request", "request body is invalid")
		return
	}

	record, err := h.app.CreateCalculation(c.Request.Context(), appcapacity.CreateCalculationCommand{UserID: demoUserID, Input: input})
	if err != nil {
		writeError(c, http.StatusInternalServerError, "internal_error", "internal server error")
		return
	}

	c.JSON(http.StatusCreated, createCalculationResponse{
		ID: record.ID,
		Result: createCalculationResult{
			PressureLevel: record.Result.PressureLevel,
			Strategy:      record.Result.Strategy,
		},
	})
}

func (h Capacity) GetCalculation(c *gin.Context) {
	record, err := h.app.GetCalculation(c.Request.Context(), appcapacity.GetCalculationQuery{ID: c.Param("id")})
	if err != nil {
		if errors.Is(err, appcapacity.ErrCalculationNotFound) {
			writeError(c, http.StatusNotFound, "not_found", "calculation not found")
			return
		}
		writeError(c, http.StatusInternalServerError, "internal_error", "internal server error")
		return
	}

	c.JSON(http.StatusOK, calculationResponse{
		ID:        record.ID,
		Input:     record.Input,
		Result:    record.Result,
		CreatedAt: record.CreatedAt.UTC().Format(time.RFC3339),
	})
}

func validCapacityInput(input domaincapacity.HousingCapacityInput) bool {
	values := []float64{
		input.CashOnHand,
		input.OldHomeValue,
		input.OldLoanBalance,
		input.MonthlyIncome,
		input.CurrentMonthlyMortgage,
		input.AcceptableMonthlyMortgage,
		input.TargetTotalPrice,
		input.RenovationBudget,
		input.TransactionCosts,
		input.TransitionRentCost,
	}
	for _, value := range values {
		if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 {
			return false
		}
	}
	return input.MonthlyIncome > 0 && input.TargetTotalPrice > 0
}

func writeError(c *gin.Context, status int, code, message string) {
	var response errorResponse
	response.Error.Code = code
	response.Error.Message = message
	c.JSON(status, response)
}
