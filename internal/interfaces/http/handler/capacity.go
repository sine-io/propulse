package handler

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	appcapacity "github.com/sine-io/propulse/internal/application/capacity"
	domaincapacity "github.com/sine-io/propulse/internal/domain/capacity"
)

type CapacityApplication interface {
	CreateCalculation(ctx context.Context, command appcapacity.CreateCalculationCommand) (appcapacity.CalculationRecord, error)
	GetAssumptions(ctx context.Context, query appcapacity.GetAssumptionsQuery) (domaincapacity.Assumptions, error)
	GetCalculation(ctx context.Context, query appcapacity.GetCalculationQuery) (appcapacity.CalculationRecord, error)
	LatestCalculation(ctx context.Context, query appcapacity.LatestCalculationQuery) (appcapacity.CalculationRecord, error)
}

type Capacity struct {
	app    CapacityApplication
	userID string
}

func NewCapacity(app CapacityApplication, userID string) Capacity {
	return Capacity{app: app, userID: userID}
}

type housingCapacityInputRequest struct {
	CashOnHand                *float64                `json:"cashOnHand"`
	OldHomeValue              *float64                `json:"oldHomeValue"`
	OldLoanBalance            *float64                `json:"oldLoanBalance"`
	MonthlyIncome             *float64                `json:"monthlyIncome"`
	CurrentMonthlyMortgage    *float64                `json:"currentMonthlyMortgage"`
	AcceptableMonthlyMortgage *float64                `json:"acceptableMonthlyMortgage"`
	TargetTotalPrice          *float64                `json:"targetTotalPrice"`
	RenovationBudget          *float64                `json:"renovationBudget"`
	TransactionCosts          *float64                `json:"transactionCosts"`
	TransitionRentCost        *float64                `json:"transitionRentCost"`
	LoanOverride              *loanParamsRequest      `json:"loanOverride,omitempty"`
	CityPolicyOverride        *cityPolicyInputRequest `json:"cityPolicyOverride,omitempty"`
}

type loanParamsRequest struct {
	AnnualInterestRate *float64 `json:"annualInterestRate"`
	LoanTermMonths     *int     `json:"loanTermMonths"`
	RepaymentMethod    *string  `json:"repaymentMethod"`
}

type cityPolicyInputRequest struct {
	City            *string  `json:"city"`
	PolicyName      *string  `json:"policyName"`
	DownPaymentRate *float64 `json:"downPaymentRate"`
	EffectiveDate   *string  `json:"effectiveDate"`
	Source          *string  `json:"source"`
}

type calculationResponse struct {
	ID        string                        `json:"id"`
	Input     housingCapacityInputResponse  `json:"input"`
	Result    housingCapacityResultResponse `json:"result"`
	CreatedAt string                        `json:"createdAt"`
}

type housingCapacityInputResponse struct {
	CashOnHand                float64                  `json:"cashOnHand"`
	OldHomeValue              float64                  `json:"oldHomeValue"`
	OldLoanBalance            float64                  `json:"oldLoanBalance"`
	MonthlyIncome             float64                  `json:"monthlyIncome"`
	CurrentMonthlyMortgage    float64                  `json:"currentMonthlyMortgage"`
	AcceptableMonthlyMortgage float64                  `json:"acceptableMonthlyMortgage"`
	TargetTotalPrice          float64                  `json:"targetTotalPrice"`
	RenovationBudget          float64                  `json:"renovationBudget"`
	TransactionCosts          float64                  `json:"transactionCosts"`
	TransitionRentCost        float64                  `json:"transitionRentCost"`
	LoanOverride              *loanParamsResponse      `json:"loanOverride,omitempty"`
	CityPolicyOverride        *cityPolicyInputResponse `json:"cityPolicyOverride,omitempty"`
}

type loanParamsResponse struct {
	AnnualInterestRate float64 `json:"annualInterestRate"`
	LoanTermMonths     int     `json:"loanTermMonths"`
	RepaymentMethod    string  `json:"repaymentMethod"`
}

type cityPolicyInputResponse struct {
	City            string  `json:"city"`
	PolicyName      string  `json:"policyName"`
	DownPaymentRate float64 `json:"downPaymentRate"`
	EffectiveDate   string  `json:"effectiveDate"`
	Source          string  `json:"source"`
}

type cityPolicyResponse struct {
	City            string                          `json:"city"`
	PolicyName      string                          `json:"policyName"`
	DownPaymentRate float64                         `json:"downPaymentRate"`
	EffectiveDate   string                          `json:"effectiveDate"`
	Source          string                          `json:"source"`
	Origin          domaincapacity.AssumptionOrigin `json:"origin"`
}

type pressureThresholdsResponse struct {
	SafeRatio        float64 `json:"safeRatio"`
	StrainedRatio    float64 `json:"strainedRatio"`
	DangerRatio      float64 `json:"dangerRatio"`
	DangerMultiplier float64 `json:"dangerMultiplier"`
}

type capacityAssumptionsResponse struct {
	appliedAssumptionsResponse
	DownPaymentRate float64 `json:"downPaymentRate"`
}

type appliedAssumptionsResponse struct {
	RuleVersion           string                          `json:"ruleVersion"`
	EffectiveDate         string                          `json:"effectiveDate"`
	RuleSource            string                          `json:"ruleSource"`
	Loan                  loanParamsResponse              `json:"loan"`
	LoanSource            string                          `json:"loanSource"`
	LoanOrigin            domaincapacity.AssumptionOrigin `json:"loanOrigin"`
	CityPolicy            cityPolicyResponse              `json:"cityPolicy"`
	ReserveMonths         float64                         `json:"reserveMonths"`
	PressureThresholds    pressureThresholdsResponse      `json:"pressureThresholds"`
	OldHomeShareThreshold float64                         `json:"oldHomeShareThreshold"`
}

type housingCapacityResultResponse struct {
	NetOldHomeProceeds          float64                           `json:"netOldHomeProceeds"`
	DeployableCash              float64                           `json:"deployableCash"`
	SafeTotalPrice              float64                           `json:"safeTotalPrice"`
	StrainedTotalPrice          float64                           `json:"strainedTotalPrice"`
	DangerTotalPrice            float64                           `json:"dangerTotalPrice"`
	DownPaymentGap              float64                           `json:"downPaymentGap"`
	MonthlyPayment              float64                           `json:"monthlyPayment"`
	MonthlyPaymentRatio         float64                           `json:"monthlyPaymentRatio"`
	PressureLevel               domaincapacity.PressureLevel      `json:"pressureLevel"`
	MinimumSafeOldHomeSalePrice float64                           `json:"minimumSafeOldHomeSalePrice"`
	Strategy                    string                            `json:"strategy"`
	Reasons                     []string                          `json:"reasons"`
	RuleVersion                 string                            `json:"ruleVersion"`
	EffectiveDate               string                            `json:"effectiveDate"`
	TraceabilityStatus          domaincapacity.TraceabilityStatus `json:"traceabilityStatus"`
	AppliedAssumptions          *appliedAssumptionsResponse       `json:"appliedAssumptions"`
}

func (h Capacity) CreateCalculation(c *gin.Context) {
	var request housingCapacityInputRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", "request body is invalid")
		return
	}
	input, err := request.domainInput()
	if err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", "request body is invalid")
		return
	}

	record, err := h.app.CreateCalculation(c.Request.Context(), appcapacity.CreateCalculationCommand{UserID: h.userID, Input: input})
	if err != nil {
		if errors.Is(err, domaincapacity.ErrInvalidInput) {
			writeError(c, http.StatusBadRequest, "invalid_request", "request body is invalid")
			return
		}
		writeError(c, http.StatusInternalServerError, "internal_error", "internal server error")
		return
	}

	c.JSON(http.StatusCreated, newCalculationResponse(record))
}

// GetAssumptions exposes the injected, currently effective rule set used to prefill the calculator.
func (h Capacity) GetAssumptions(c *gin.Context) {
	assumptions, err := h.app.GetAssumptions(c.Request.Context(), appcapacity.GetAssumptionsQuery{})
	if err != nil {
		writeError(c, http.StatusInternalServerError, "internal_error", "internal server error")
		return
	}
	c.JSON(http.StatusOK, newCapacityAssumptionsResponse(assumptions))
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

	c.JSON(http.StatusOK, newCalculationResponse(record))
}

func (request housingCapacityInputRequest) domainInput() (domaincapacity.HousingCapacityInput, error) {
	if request.CashOnHand == nil || request.OldHomeValue == nil || request.OldLoanBalance == nil ||
		request.MonthlyIncome == nil || request.CurrentMonthlyMortgage == nil || request.AcceptableMonthlyMortgage == nil ||
		request.TargetTotalPrice == nil || request.RenovationBudget == nil || request.TransactionCosts == nil ||
		request.TransitionRentCost == nil {
		return domaincapacity.HousingCapacityInput{}, domaincapacity.ErrInvalidInput
	}
	input := domaincapacity.HousingCapacityInput{
		CashOnHand: *request.CashOnHand, OldHomeValue: *request.OldHomeValue, OldLoanBalance: *request.OldLoanBalance,
		MonthlyIncome: *request.MonthlyIncome, CurrentMonthlyMortgage: *request.CurrentMonthlyMortgage,
		AcceptableMonthlyMortgage: *request.AcceptableMonthlyMortgage, TargetTotalPrice: *request.TargetTotalPrice,
		RenovationBudget: *request.RenovationBudget, TransactionCosts: *request.TransactionCosts,
		TransitionRentCost: *request.TransitionRentCost,
	}
	if request.LoanOverride != nil {
		loan := request.LoanOverride
		if loan.AnnualInterestRate == nil || loan.LoanTermMonths == nil || loan.RepaymentMethod == nil {
			return domaincapacity.HousingCapacityInput{}, domaincapacity.ErrInvalidInput
		}
		input.LoanOverride = &domaincapacity.LoanParams{
			AnnualInterestRate: *loan.AnnualInterestRate,
			LoanTermMonths:     *loan.LoanTermMonths,
			RepaymentMethod:    domaincapacity.RepaymentMethod(*loan.RepaymentMethod),
		}
	}
	if request.CityPolicyOverride != nil {
		policy := request.CityPolicyOverride
		if policy.City == nil || policy.PolicyName == nil || policy.DownPaymentRate == nil ||
			policy.EffectiveDate == nil || policy.Source == nil {
			return domaincapacity.HousingCapacityInput{}, domaincapacity.ErrInvalidInput
		}
		input.CityPolicyOverride = &domaincapacity.CityPolicy{
			City: *policy.City, PolicyName: *policy.PolicyName, DownPaymentRate: *policy.DownPaymentRate,
			EffectiveDate: *policy.EffectiveDate, Source: *policy.Source,
		}
	}
	return input, nil
}

func newCalculationResponse(record appcapacity.CalculationRecord) calculationResponse {
	return calculationResponse{
		ID: record.ID, Input: newHousingCapacityInputResponse(record.Input),
		Result: newHousingCapacityResultResponse(record.Result), CreatedAt: record.CreatedAt.UTC().Format(time.RFC3339),
	}
}

func newHousingCapacityInputResponse(input domaincapacity.HousingCapacityInput) housingCapacityInputResponse {
	response := housingCapacityInputResponse{
		CashOnHand: input.CashOnHand, OldHomeValue: input.OldHomeValue, OldLoanBalance: input.OldLoanBalance,
		MonthlyIncome: input.MonthlyIncome, CurrentMonthlyMortgage: input.CurrentMonthlyMortgage,
		AcceptableMonthlyMortgage: input.AcceptableMonthlyMortgage, TargetTotalPrice: input.TargetTotalPrice,
		RenovationBudget: input.RenovationBudget, TransactionCosts: input.TransactionCosts,
		TransitionRentCost: input.TransitionRentCost,
	}
	if input.LoanOverride != nil {
		loan := newLoanParamsResponse(*input.LoanOverride)
		response.LoanOverride = &loan
	}
	if input.CityPolicyOverride != nil {
		policy := newCityPolicyInputResponse(*input.CityPolicyOverride)
		response.CityPolicyOverride = &policy
	}
	return response
}

func newHousingCapacityResultResponse(result domaincapacity.HousingCapacityResult) housingCapacityResultResponse {
	response := housingCapacityResultResponse{
		NetOldHomeProceeds: result.NetOldHomeProceeds, DeployableCash: result.DeployableCash,
		SafeTotalPrice: result.SafeTotalPrice, StrainedTotalPrice: result.StrainedTotalPrice,
		DangerTotalPrice: result.DangerTotalPrice, DownPaymentGap: result.DownPaymentGap,
		MonthlyPayment: result.MonthlyPayment, MonthlyPaymentRatio: result.MonthlyPaymentRatio,
		PressureLevel: result.PressureLevel, MinimumSafeOldHomeSalePrice: result.MinimumSafeOldHomeSalePrice,
		Strategy: result.Strategy, Reasons: result.Reasons, RuleVersion: result.RuleVersion,
		EffectiveDate: result.EffectiveDate, TraceabilityStatus: result.TraceabilityStatus,
	}
	if result.AppliedAssumptions != nil {
		assumptions := newAppliedAssumptionsResponse(*result.AppliedAssumptions)
		response.AppliedAssumptions = &assumptions
	}
	return response
}

func newCapacityAssumptionsResponse(assumptions domaincapacity.Assumptions) capacityAssumptionsResponse {
	return capacityAssumptionsResponse{
		appliedAssumptionsResponse: newAppliedAssumptionsResponse(assumptions),
		DownPaymentRate:            assumptions.CityPolicy.DownPaymentRate,
	}
}

func newAppliedAssumptionsResponse(assumptions domaincapacity.Assumptions) appliedAssumptionsResponse {
	return appliedAssumptionsResponse{
		RuleVersion: assumptions.RuleVersion, EffectiveDate: assumptions.EffectiveDate, RuleSource: assumptions.RuleSource,
		Loan:       newLoanParamsResponse(assumptions.Loan),
		LoanSource: assumptions.LoanSource, LoanOrigin: assumptions.LoanOrigin,
		CityPolicy: newCityPolicyResponse(assumptions.CityPolicy), ReserveMonths: assumptions.ReserveMonths,
		PressureThresholds: pressureThresholdsResponse{
			SafeRatio: assumptions.PressureThresholds.SafeRatio, StrainedRatio: assumptions.PressureThresholds.StrainedRatio,
			DangerRatio: assumptions.PressureThresholds.DangerRatio, DangerMultiplier: assumptions.PressureThresholds.DangerMultiplier,
		},
		OldHomeShareThreshold: assumptions.OldHomeShareThreshold,
	}
}

func newLoanParamsResponse(loan domaincapacity.LoanParams) loanParamsResponse {
	return loanParamsResponse{
		AnnualInterestRate: loan.AnnualInterestRate,
		LoanTermMonths:     loan.LoanTermMonths,
		RepaymentMethod:    string(loan.RepaymentMethod),
	}
}

func newCityPolicyInputResponse(policy domaincapacity.CityPolicy) cityPolicyInputResponse {
	return cityPolicyInputResponse{
		City: policy.City, PolicyName: policy.PolicyName, DownPaymentRate: policy.DownPaymentRate,
		EffectiveDate: policy.EffectiveDate, Source: policy.Source,
	}
}

func newCityPolicyResponse(policy domaincapacity.CityPolicy) cityPolicyResponse {
	return cityPolicyResponse{
		City: policy.City, PolicyName: policy.PolicyName, DownPaymentRate: policy.DownPaymentRate,
		EffectiveDate: policy.EffectiveDate, Source: policy.Source, Origin: policy.Origin,
	}
}
