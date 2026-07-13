package handler

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	appcapacity "github.com/sine-io/propulse/internal/application/capacity"
	"github.com/sine-io/propulse/internal/application/user"
	domaincapacity "github.com/sine-io/propulse/internal/domain/capacity"
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
	RuleVersion   string                       `json:"ruleVersion"`
	EffectiveDate string                       `json:"effectiveDate"`
}

type calculationResponse struct {
	ID        string                        `json:"id"`
	Input     housingCapacityInputResponse  `json:"input"`
	Result    housingCapacityResultResponse `json:"result"`
	CreatedAt string                        `json:"createdAt"`
}

type housingCapacityInputResponse struct {
	CashOnHand                float64 `json:"cashOnHand"`
	OldHomeValue              float64 `json:"oldHomeValue"`
	OldLoanBalance            float64 `json:"oldLoanBalance"`
	MonthlyIncome             float64 `json:"monthlyIncome"`
	CurrentMonthlyMortgage    float64 `json:"currentMonthlyMortgage"`
	AcceptableMonthlyMortgage float64 `json:"acceptableMonthlyMortgage"`
	TargetTotalPrice          float64 `json:"targetTotalPrice"`
	RenovationBudget          float64 `json:"renovationBudget"`
	TransactionCosts          float64 `json:"transactionCosts"`
	TransitionRentCost        float64 `json:"transitionRentCost"`
}

type housingCapacityResultResponse struct {
	NetOldHomeProceeds          float64                      `json:"netOldHomeProceeds"`
	DeployableCash              float64                      `json:"deployableCash"`
	SafeTotalPrice              float64                      `json:"safeTotalPrice"`
	StrainedTotalPrice          float64                      `json:"strainedTotalPrice"`
	DangerTotalPrice            float64                      `json:"dangerTotalPrice"`
	DownPaymentGap              float64                      `json:"downPaymentGap"`
	MonthlyPayment              float64                      `json:"monthlyPayment"`
	MonthlyPaymentRatio         float64                      `json:"monthlyPaymentRatio"`
	PressureLevel               domaincapacity.PressureLevel `json:"pressureLevel"`
	MinimumSafeOldHomeSalePrice float64                      `json:"minimumSafeOldHomeSalePrice"`
	Strategy                    string                       `json:"strategy"`
	Reasons                     []string                     `json:"reasons"`
	RuleVersion                 string                       `json:"ruleVersion"`
	EffectiveDate               string                       `json:"effectiveDate"`
}

func (h Capacity) CreateCalculation(c *gin.Context) {
	var request housingCapacityInputResponse
	if err := c.ShouldBindJSON(&request); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", "request body is invalid")
		return
	}
	input := request.domainInput()
	if err := input.Validate(); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", "request body is invalid")
		return
	}

	record, err := h.app.CreateCalculation(c.Request.Context(), appcapacity.CreateCalculationCommand{UserID: user.SingleUserID, Input: input})
	if err != nil {
		if errors.Is(err, domaincapacity.ErrInvalidInput) {
			writeError(c, http.StatusBadRequest, "invalid_request", "request body is invalid")
			return
		}
		writeError(c, http.StatusInternalServerError, "internal_error", "internal server error")
		return
	}

	c.JSON(http.StatusCreated, createCalculationResponse{
		ID: record.ID,
		Result: createCalculationResult{
			PressureLevel: record.Result.PressureLevel,
			Strategy:      record.Result.Strategy,
			RuleVersion:   record.Result.RuleVersion,
			EffectiveDate: record.Result.EffectiveDate,
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
		Input:     newHousingCapacityInputResponse(record.Input),
		Result:    newHousingCapacityResultResponse(record.Result),
		CreatedAt: record.CreatedAt.UTC().Format(time.RFC3339),
	})
}

func (response housingCapacityInputResponse) domainInput() domaincapacity.HousingCapacityInput {
	return domaincapacity.HousingCapacityInput{
		CashOnHand: response.CashOnHand, OldHomeValue: response.OldHomeValue, OldLoanBalance: response.OldLoanBalance,
		MonthlyIncome: response.MonthlyIncome, CurrentMonthlyMortgage: response.CurrentMonthlyMortgage,
		AcceptableMonthlyMortgage: response.AcceptableMonthlyMortgage, TargetTotalPrice: response.TargetTotalPrice,
		RenovationBudget: response.RenovationBudget, TransactionCosts: response.TransactionCosts,
		TransitionRentCost: response.TransitionRentCost,
	}
}

func newHousingCapacityInputResponse(input domaincapacity.HousingCapacityInput) housingCapacityInputResponse {
	return housingCapacityInputResponse{
		CashOnHand: input.CashOnHand, OldHomeValue: input.OldHomeValue, OldLoanBalance: input.OldLoanBalance,
		MonthlyIncome: input.MonthlyIncome, CurrentMonthlyMortgage: input.CurrentMonthlyMortgage,
		AcceptableMonthlyMortgage: input.AcceptableMonthlyMortgage, TargetTotalPrice: input.TargetTotalPrice,
		RenovationBudget: input.RenovationBudget, TransactionCosts: input.TransactionCosts,
		TransitionRentCost: input.TransitionRentCost,
	}
}

func newHousingCapacityResultResponse(result domaincapacity.HousingCapacityResult) housingCapacityResultResponse {
	return housingCapacityResultResponse{
		NetOldHomeProceeds: result.NetOldHomeProceeds, DeployableCash: result.DeployableCash,
		SafeTotalPrice: result.SafeTotalPrice, StrainedTotalPrice: result.StrainedTotalPrice,
		DangerTotalPrice: result.DangerTotalPrice, DownPaymentGap: result.DownPaymentGap,
		MonthlyPayment: result.MonthlyPayment, MonthlyPaymentRatio: result.MonthlyPaymentRatio,
		PressureLevel: result.PressureLevel, MinimumSafeOldHomeSalePrice: result.MinimumSafeOldHomeSalePrice,
		Strategy: result.Strategy, Reasons: result.Reasons,
		RuleVersion: result.RuleVersion, EffectiveDate: result.EffectiveDate,
	}
}
