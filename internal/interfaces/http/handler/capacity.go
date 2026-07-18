package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	appcapacity "github.com/sine-io/propulse/internal/application/capacity"
	domaincapacity "github.com/sine-io/propulse/internal/domain/capacity"
)

type CapacityApplication interface {
	CreateCalculation(ctx context.Context, command appcapacity.CreateCalculationCommand) (appcapacity.CalculationRecord, error)
	GetAssumptions(ctx context.Context, query appcapacity.GetAssumptionsQuery) (appcapacity.AssumptionsView, error)
	GetCalculation(ctx context.Context, query appcapacity.GetCalculationQuery) (appcapacity.CalculationRecord, error)
	ListCalculations(ctx context.Context, query appcapacity.ListCalculationsQuery) (appcapacity.CalculationHistoryPage, error)
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
	CashOnHand                *float64                     `json:"cashOnHand"`
	OldHomeValue              *float64                     `json:"oldHomeValue"`
	OldLoanBalance            *float64                     `json:"oldLoanBalance"`
	MonthlyIncome             *float64                     `json:"monthlyIncome"`
	CurrentMonthlyMortgage    *float64                     `json:"currentMonthlyMortgage"`
	AcceptableMonthlyMortgage *float64                     `json:"acceptableMonthlyMortgage"`
	TargetTotalPrice          *float64                     `json:"targetTotalPrice"`
	RenovationBudget          *float64                     `json:"renovationBudget"`
	TransactionCosts          *float64                     `json:"transactionCosts"`
	TransitionRentCost        *float64                     `json:"transitionRentCost"`
	TransactionScenario       *transactionScenarioRequest  `json:"transactionScenario,omitempty"`
	LoanPlan                  *loanPlanRequest             `json:"loanPlan,omitempty"`
	ManualOverrides           *calculationOverridesRequest `json:"manualOverrides,omitempty"`
	LoanOverride              *loanParamsRequest           `json:"loanOverride,omitempty"`
	CityPolicyOverride        *cityPolicyInputRequest      `json:"cityPolicyOverride,omitempty"`
	OldHomeSelection          *oldHomeSelectionRequest     `json:"oldHomeSelection,omitempty"`
	TargetHomeSelection       *targetHomeSelectionRequest  `json:"targetHomeSelection,omitempty"`
}

type oldHomeSelectionRequest struct {
	Mode                 *appcapacity.OldHomeSelectionMode `json:"mode"`
	AssetID              string                            `json:"assetId,omitempty"`
	ExpectedSalePriceWan *float64                          `json:"expectedSalePriceWan,omitempty"`
	PriceConfirmed       bool                              `json:"priceConfirmed"`
}

type targetHomeSelectionRequest struct {
	NeighborhoodID           *string  `json:"neighborhoodId"`
	RoomID                   *string  `json:"roomId"`
	ExpectedPurchasePriceWan *float64 `json:"expectedPurchasePriceWan"`
	PriceConfirmed           bool     `json:"priceConfirmed"`
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
	ID               string                        `json:"id"`
	Input            housingCapacityInputResponse  `json:"input"`
	Result           housingCapacityResultResponse `json:"result"`
	SelectionContext *appcapacity.SelectionContext `json:"selectionContext,omitempty"`
	CreatedAt        string                        `json:"createdAt"`
}

type calculationSummaryResponse struct {
	ID                     string                       `json:"id"`
	CreatedAt              string                       `json:"createdAt"`
	PressureLevel          domaincapacity.PressureLevel `json:"pressureLevel"`
	TargetTotalPrice       float64                      `json:"targetTotalPrice"`
	TargetNeighborhoodName string                       `json:"targetNeighborhoodName"`
	TargetLayout           string                       `json:"targetLayout"`
	OldHomeName            string                       `json:"oldHomeName"`
}

type calculationHistoryPageResponse struct {
	Items    []calculationSummaryResponse `json:"items"`
	Total    int64                        `json:"total"`
	Page     int                          `json:"page"`
	PageSize int                          `json:"pageSize"`
}

type housingCapacityInputResponse struct {
	CashOnHand                float64                       `json:"cashOnHand"`
	OldHomeValue              float64                       `json:"oldHomeValue"`
	OldLoanBalance            float64                       `json:"oldLoanBalance"`
	MonthlyIncome             float64                       `json:"monthlyIncome"`
	CurrentMonthlyMortgage    float64                       `json:"currentMonthlyMortgage"`
	AcceptableMonthlyMortgage float64                       `json:"acceptableMonthlyMortgage"`
	TargetTotalPrice          float64                       `json:"targetTotalPrice"`
	RenovationBudget          float64                       `json:"renovationBudget"`
	TransactionCosts          float64                       `json:"transactionCosts"`
	TransitionRentCost        float64                       `json:"transitionRentCost"`
	TransactionScenario       *transactionScenarioResponse  `json:"transactionScenario,omitempty"`
	LoanPlan                  *loanPlanResponse             `json:"loanPlan,omitempty"`
	ManualOverrides           *calculationOverridesResponse `json:"manualOverrides,omitempty"`
	LoanOverride              *loanParamsResponse           `json:"loanOverride,omitempty"`
	CityPolicyOverride        *cityPolicyInputResponse      `json:"cityPolicyOverride,omitempty"`
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
	DownPaymentRate   float64                `json:"downPaymentRate"`
	PolicyVersion     *housingPolicyResponse `json:"policyVersion,omitempty"`
	Sources           []policySourceResponse `json:"sources"`
	LoanOptions       []loanOptionResponse   `json:"loanOptions"`
	HomePurchaseOrder string                 `json:"homePurchaseOrder"`
	LoanTermMonths    int                    `json:"loanTermMonths"`
	Disclaimer        string                 `json:"disclaimer"`
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
	RecommendedDownPaymentRate  float64                           `json:"recommendedDownPaymentRate,omitempty"`
	RecommendedDownPayment      float64                           `json:"recommendedDownPayment,omitempty"`
	LoanBreakdown               *loanBreakdownResponse            `json:"loanBreakdown,omitempty"`
	TaxBreakdown                *taxBreakdownResponse             `json:"taxBreakdown,omitempty"`
	PolicyVersion               *policyVersionReferenceResponse   `json:"policyVersion,omitempty"`
	Sources                     []policySourceResponse            `json:"sources,omitempty"`
	ManualOverrides             []appliedManualOverrideResponse   `json:"manualOverrides,omitempty"`
	Disclaimer                  string                            `json:"disclaimer,omitempty"`
}

func (h Capacity) CreateCalculation(c *gin.Context) {
	var request housingCapacityInputRequest
	if err := decodeStrictJSON(c, &request); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", "request body is invalid")
		return
	}
	input, err := request.domainInput()
	if err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", "request body is invalid")
		return
	}

	oldSelection, err := request.oldHomeSelectionInput()
	if err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", "request body is invalid")
		return
	}
	targetSelection, err := request.targetHomeSelectionInput()
	if err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", "request body is invalid")
		return
	}
	record, err := h.app.CreateCalculation(c.Request.Context(), appcapacity.CreateCalculationCommand{
		UserID: h.userID, Input: input, OldHomeSelection: oldSelection, TargetHomeSelection: targetSelection,
	})
	if err != nil {
		if errors.Is(err, domaincapacity.ErrInvalidInput) || errors.Is(err, appcapacity.ErrInvalidSelection) {
			writeError(c, http.StatusBadRequest, "invalid_request", "request body is invalid")
			return
		}
		if errors.Is(err, appcapacity.ErrSelectedAssetNotFound) || errors.Is(err, appcapacity.ErrTargetListingNotFound) {
			writeError(c, http.StatusNotFound, "selection_not_found", "selected asset or target listing was not found")
			return
		}
		if errors.Is(err, appcapacity.ErrTargetListingUnavailable) {
			writeError(c, http.StatusConflict, "listing_unavailable", "selected target listing is no longer active")
			return
		}
		writeError(c, http.StatusInternalServerError, "internal_error", "internal server error")
		return
	}

	c.JSON(http.StatusCreated, newCalculationResponse(record))
}

// GetAssumptions exposes the injected, currently effective rule set used to prefill the calculator.
func (h Capacity) GetAssumptions(c *gin.Context) {
	term := 0
	if raw := c.Query("loanTermMonths"); raw != "" {
		parsed, parseErr := strconv.Atoi(raw)
		if parseErr != nil {
			writeError(c, http.StatusBadRequest, "invalid_request", "query parameters are invalid")
			return
		}
		term = parsed
	}
	assumptions, err := h.app.GetAssumptions(c.Request.Context(), appcapacity.GetAssumptionsQuery{
		City: c.Query("city"), HomePurchaseOrder: domaincapacity.HomePurchaseOrder(c.Query("homePurchaseOrder")), LoanTermMonths: term,
	})
	if err != nil {
		if errors.Is(err, domaincapacity.ErrInvalidInput) {
			writeError(c, http.StatusBadRequest, "invalid_request", "query parameters are invalid")
			return
		}
		writeError(c, http.StatusInternalServerError, "internal_error", "internal server error")
		return
	}
	c.JSON(http.StatusOK, newCapacityAssumptionsResponse(assumptions))
}

func (h Capacity) GetCalculation(c *gin.Context) {
	record, err := h.app.GetCalculation(c.Request.Context(), appcapacity.GetCalculationQuery{UserID: h.userID, ID: c.Param("id")})
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

func (h Capacity) ListCalculations(c *gin.Context) {
	page, err := parseOptionalPositiveInt(c, "page")
	if err != nil {
		writeError(c, http.StatusBadRequest, "invalid_query", "query parameters are invalid")
		return
	}
	pageSize, err := parseOptionalPositiveInt(c, "pageSize")
	if err != nil {
		writeError(c, http.StatusBadRequest, "invalid_query", "query parameters are invalid")
		return
	}
	result, err := h.app.ListCalculations(c.Request.Context(), appcapacity.ListCalculationsQuery{
		UserID: h.userID, Query: c.Query("q"), Page: page, PageSize: pageSize,
	})
	if err != nil {
		if errors.Is(err, appcapacity.ErrInvalidCalculationQuery) {
			writeError(c, http.StatusBadRequest, "invalid_query", "query parameters are invalid")
			return
		}
		writeError(c, http.StatusInternalServerError, "internal_error", "internal server error")
		return
	}
	response := calculationHistoryPageResponse{
		Items: make([]calculationSummaryResponse, 0, len(result.Items)),
		Total: result.Total, Page: result.Page, PageSize: result.PageSize,
	}
	for _, item := range result.Items {
		response.Items = append(response.Items, calculationSummaryResponse{
			ID: item.ID, CreatedAt: item.CreatedAt.UTC().Format(time.RFC3339), PressureLevel: item.PressureLevel,
			TargetTotalPrice: item.TargetTotalPrice, TargetNeighborhoodName: item.TargetNeighborhoodName,
			TargetLayout: item.TargetLayout, OldHomeName: item.OldHomeName,
		})
	}
	c.JSON(http.StatusOK, response)
}

func parseOptionalPositiveInt(c *gin.Context, name string) (int, error) {
	raw, exists := c.GetQuery(name)
	if !exists {
		return 0, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 1 || (name == "pageSize" && value > 100) {
		return 0, appcapacity.ErrInvalidCalculationQuery
	}
	return value, nil
}

func (request housingCapacityInputRequest) domainInput() (domaincapacity.HousingCapacityInput, error) {
	newPolicyInput := request.TransactionScenario != nil || request.LoanPlan != nil || request.ManualOverrides != nil
	if request.CashOnHand == nil || request.OldHomeValue == nil || request.OldLoanBalance == nil ||
		request.MonthlyIncome == nil || request.CurrentMonthlyMortgage == nil || request.AcceptableMonthlyMortgage == nil ||
		request.TargetTotalPrice == nil || request.RenovationBudget == nil || (!newPolicyInput && request.TransactionCosts == nil) ||
		request.TransitionRentCost == nil {
		return domaincapacity.HousingCapacityInput{}, domaincapacity.ErrInvalidInput
	}
	input := domaincapacity.HousingCapacityInput{
		CashOnHand: *request.CashOnHand, OldHomeValue: *request.OldHomeValue, OldLoanBalance: *request.OldLoanBalance,
		MonthlyIncome: *request.MonthlyIncome, CurrentMonthlyMortgage: *request.CurrentMonthlyMortgage,
		AcceptableMonthlyMortgage: *request.AcceptableMonthlyMortgage, TargetTotalPrice: *request.TargetTotalPrice,
		RenovationBudget:   *request.RenovationBudget,
		TransitionRentCost: *request.TransitionRentCost,
	}
	if request.TransactionCosts != nil {
		input.TransactionCosts = *request.TransactionCosts
	}
	if request.TransactionScenario != nil {
		scenario, parseErr := request.TransactionScenario.domain()
		if parseErr != nil {
			return domaincapacity.HousingCapacityInput{}, parseErr
		}
		input.TransactionScenario = &scenario
	}
	if request.LoanPlan != nil {
		plan, parseErr := request.LoanPlan.domain()
		if parseErr != nil {
			return domaincapacity.HousingCapacityInput{}, parseErr
		}
		input.LoanPlan = &plan
	}
	if request.ManualOverrides != nil {
		input.ManualOverrides = request.ManualOverrides.domain()
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
		Result: newHousingCapacityResultResponse(record.Result), SelectionContext: record.SelectionContext,
		CreatedAt: record.CreatedAt.UTC().Format(time.RFC3339),
	}
}

func (request housingCapacityInputRequest) oldHomeSelectionInput() (*appcapacity.OldHomeSelectionInput, error) {
	if request.OldHomeSelection == nil {
		return nil, nil
	}
	if request.OldHomeSelection.Mode == nil {
		return nil, appcapacity.ErrInvalidSelection
	}
	return &appcapacity.OldHomeSelectionInput{
		Mode: *request.OldHomeSelection.Mode, AssetID: request.OldHomeSelection.AssetID,
		ExpectedSalePriceWan: request.OldHomeSelection.ExpectedSalePriceWan,
		PriceConfirmed:       request.OldHomeSelection.PriceConfirmed,
	}, nil
}

func (request housingCapacityInputRequest) targetHomeSelectionInput() (*appcapacity.TargetHomeSelectionInput, error) {
	if request.TargetHomeSelection == nil {
		return nil, nil
	}
	if request.TargetHomeSelection.NeighborhoodID == nil || request.TargetHomeSelection.RoomID == nil ||
		request.TargetHomeSelection.ExpectedPurchasePriceWan == nil {
		return nil, appcapacity.ErrInvalidSelection
	}
	return &appcapacity.TargetHomeSelectionInput{
		NeighborhoodID: *request.TargetHomeSelection.NeighborhoodID, RoomID: *request.TargetHomeSelection.RoomID,
		ExpectedPurchasePriceWan: request.TargetHomeSelection.ExpectedPurchasePriceWan,
		PriceConfirmed:           request.TargetHomeSelection.PriceConfirmed,
	}, nil
}

func newHousingCapacityInputResponse(input domaincapacity.HousingCapacityInput) housingCapacityInputResponse {
	response := housingCapacityInputResponse{
		CashOnHand: input.CashOnHand, OldHomeValue: input.OldHomeValue, OldLoanBalance: input.OldLoanBalance,
		MonthlyIncome: input.MonthlyIncome, CurrentMonthlyMortgage: input.CurrentMonthlyMortgage,
		AcceptableMonthlyMortgage: input.AcceptableMonthlyMortgage, TargetTotalPrice: input.TargetTotalPrice,
		RenovationBudget: input.RenovationBudget, TransactionCosts: input.TransactionCosts,
		TransitionRentCost: input.TransitionRentCost,
	}
	if input.TransactionScenario != nil {
		scenario := newTransactionScenarioResponse(*input.TransactionScenario)
		response.TransactionScenario = &scenario
	}
	if input.LoanPlan != nil {
		plan := newLoanPlanResponse(*input.LoanPlan)
		response.LoanPlan = &plan
	}
	if input.ManualOverrides != nil {
		overrides := newCalculationOverridesResponse(*input.ManualOverrides)
		response.ManualOverrides = &overrides
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
		RecommendedDownPaymentRate: result.RecommendedDownPaymentRate, RecommendedDownPayment: result.RecommendedDownPayment,
		Sources: newPolicySourceResponses(result.Sources), ManualOverrides: newAppliedManualOverrideResponses(result.ManualOverrides),
		Disclaimer: result.Disclaimer,
	}
	if result.LoanBreakdown != nil {
		loan := newLoanBreakdownResponse(*result.LoanBreakdown)
		response.LoanBreakdown = &loan
	}
	if result.TaxBreakdown != nil {
		taxes := newTaxBreakdownResponse(*result.TaxBreakdown)
		response.TaxBreakdown = &taxes
	}
	if result.PolicyVersion != nil {
		policy := newPolicyVersionReferenceResponse(*result.PolicyVersion)
		response.PolicyVersion = &policy
	}
	if result.AppliedAssumptions != nil {
		assumptions := newAppliedAssumptionsResponse(*result.AppliedAssumptions)
		response.AppliedAssumptions = &assumptions
	}
	return response
}

func newCapacityAssumptionsResponse(view appcapacity.AssumptionsView) capacityAssumptionsResponse {
	response := capacityAssumptionsResponse{
		appliedAssumptionsResponse: newAppliedAssumptionsResponse(view.Legacy),
		DownPaymentRate:            view.Legacy.CityPolicy.DownPaymentRate,
		Sources:                    []policySourceResponse{}, LoanOptions: newLoanOptionResponses(view.LoanOptions),
		HomePurchaseOrder: string(view.HomePurchaseOrder), LoanTermMonths: view.LoanTermMonths, Disclaimer: view.Disclaimer,
	}
	if view.Policy != nil {
		policy := newHousingPolicyResponse(*view.Policy)
		response.PolicyVersion = &policy
		response.Sources = policy.Sources
		response.RuleVersion = policy.Version
		response.EffectiveDate = policy.EffectiveFrom
		response.RuleSource = policy.Name
		if len(view.LoanOptions) > 0 {
			response.DownPaymentRate = view.LoanOptions[0].DownPaymentRate
			response.CityPolicy = cityPolicyResponse{
				City: policy.City, PolicyName: policy.Name, DownPaymentRate: response.DownPaymentRate,
				EffectiveDate: policy.EffectiveFrom, Source: policy.Sources[0].URL, Origin: domaincapacity.OriginConfiguredDefault,
			}
		}
	}
	return response
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
