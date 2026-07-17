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

type transactionScenarioRequest struct {
	City                  *string  `json:"city"`
	HomePurchaseOrder     *string  `json:"homePurchaseOrder"`
	TargetHomeType        *string  `json:"targetHomeType"`
	TargetHomeAreaSQM     *float64 `json:"targetHomeAreaSqm"`
	OldHomeHoldingYears   *int     `json:"oldHomeHoldingYears"`
	OldHomeOnlyFamilyHome *bool    `json:"oldHomeOnlyFamilyHome"`
	OldHomeOriginalPrice  *float64 `json:"oldHomeOriginalPrice"`
	TaxBurdenMode         *string  `json:"taxBurdenMode"`
}

func (request transactionScenarioRequest) domain() (domaincapacity.TransactionScenario, error) {
	if request.City == nil || request.HomePurchaseOrder == nil || request.TargetHomeType == nil ||
		request.TargetHomeAreaSQM == nil || request.OldHomeHoldingYears == nil || request.OldHomeOnlyFamilyHome == nil ||
		request.OldHomeOriginalPrice == nil || request.TaxBurdenMode == nil {
		return domaincapacity.TransactionScenario{}, domaincapacity.ErrInvalidInput
	}
	return domaincapacity.TransactionScenario{
		City: *request.City, HomePurchaseOrder: domaincapacity.HomePurchaseOrder(*request.HomePurchaseOrder),
		TargetHomeType: domaincapacity.TargetHomeType(*request.TargetHomeType), TargetHomeAreaSQM: *request.TargetHomeAreaSQM,
		OldHomeHoldingYears: *request.OldHomeHoldingYears, OldHomeOnlyFamilyHome: *request.OldHomeOnlyFamilyHome,
		OldHomeOriginalPrice: *request.OldHomeOriginalPrice, TaxBurdenMode: domaincapacity.TaxBurdenMode(*request.TaxBurdenMode),
	}, nil
}

type loanPlanRequest struct {
	Type                    *string  `json:"type"`
	TotalLoanAmount         *float64 `json:"totalLoanAmount"`
	CommercialLoanAmount    *float64 `json:"commercialLoanAmount,omitempty"`
	ProvidentFundLoanAmount *float64 `json:"providentFundLoanAmount,omitempty"`
	LoanTermMonths          *int     `json:"loanTermMonths"`
	RepaymentMethod         *string  `json:"repaymentMethod"`
}

func (request loanPlanRequest) domain() (domaincapacity.LoanPlan, error) {
	if request.Type == nil || request.TotalLoanAmount == nil || request.LoanTermMonths == nil || request.RepaymentMethod == nil {
		return domaincapacity.LoanPlan{}, domaincapacity.ErrInvalidInput
	}
	plan := domaincapacity.LoanPlan{
		Type: domaincapacity.LoanType(*request.Type), TotalLoanAmount: *request.TotalLoanAmount,
		LoanTermMonths: *request.LoanTermMonths, RepaymentMethod: domaincapacity.RepaymentMethod(*request.RepaymentMethod),
	}
	if request.CommercialLoanAmount != nil {
		plan.CommercialLoanAmount = *request.CommercialLoanAmount
	}
	if request.ProvidentFundLoanAmount != nil {
		plan.ProvidentFundLoanAmount = *request.ProvidentFundLoanAmount
	}
	return plan, nil
}

type calculationOverridesRequest struct {
	CommercialAnnualInterestRate *float64           `json:"commercialAnnualInterestRate,omitempty"`
	ProvidentAnnualInterestRate  *float64           `json:"providentAnnualInterestRate,omitempty"`
	DownPaymentRate              *float64           `json:"downPaymentRate,omitempty"`
	TaxAmounts                   map[string]float64 `json:"taxAmounts,omitempty"`
}

func (request calculationOverridesRequest) domain() *domaincapacity.CalculationOverrides {
	return &domaincapacity.CalculationOverrides{
		CommercialAnnualInterestRate: request.CommercialAnnualInterestRate,
		ProvidentAnnualInterestRate:  request.ProvidentAnnualInterestRate,
		DownPaymentRate:              request.DownPaymentRate, TaxAmounts: copyFloatMap(request.TaxAmounts),
	}
}

type transactionScenarioResponse struct {
	City                  string                           `json:"city"`
	HomePurchaseOrder     domaincapacity.HomePurchaseOrder `json:"homePurchaseOrder"`
	TargetHomeType        domaincapacity.TargetHomeType    `json:"targetHomeType"`
	TargetHomeAreaSQM     float64                          `json:"targetHomeAreaSqm"`
	OldHomeHoldingYears   int                              `json:"oldHomeHoldingYears"`
	OldHomeOnlyFamilyHome bool                             `json:"oldHomeOnlyFamilyHome"`
	OldHomeOriginalPrice  float64                          `json:"oldHomeOriginalPrice"`
	TaxBurdenMode         domaincapacity.TaxBurdenMode     `json:"taxBurdenMode"`
}

func newTransactionScenarioResponse(value domaincapacity.TransactionScenario) transactionScenarioResponse {
	return transactionScenarioResponse{
		City: value.City, HomePurchaseOrder: value.HomePurchaseOrder, TargetHomeType: value.TargetHomeType,
		TargetHomeAreaSQM: value.TargetHomeAreaSQM, OldHomeHoldingYears: value.OldHomeHoldingYears,
		OldHomeOnlyFamilyHome: value.OldHomeOnlyFamilyHome, OldHomeOriginalPrice: value.OldHomeOriginalPrice,
		TaxBurdenMode: value.TaxBurdenMode,
	}
}

type loanPlanResponse struct {
	Type                    domaincapacity.LoanType        `json:"type"`
	TotalLoanAmount         float64                        `json:"totalLoanAmount"`
	CommercialLoanAmount    float64                        `json:"commercialLoanAmount"`
	ProvidentFundLoanAmount float64                        `json:"providentFundLoanAmount"`
	LoanTermMonths          int                            `json:"loanTermMonths"`
	RepaymentMethod         domaincapacity.RepaymentMethod `json:"repaymentMethod"`
}

func newLoanPlanResponse(value domaincapacity.LoanPlan) loanPlanResponse {
	return loanPlanResponse{
		Type: value.Type, TotalLoanAmount: value.TotalLoanAmount, CommercialLoanAmount: value.CommercialLoanAmount,
		ProvidentFundLoanAmount: value.ProvidentFundLoanAmount, LoanTermMonths: value.LoanTermMonths,
		RepaymentMethod: value.RepaymentMethod,
	}
}

type calculationOverridesResponse struct {
	CommercialAnnualInterestRate *float64           `json:"commercialAnnualInterestRate,omitempty"`
	ProvidentAnnualInterestRate  *float64           `json:"providentAnnualInterestRate,omitempty"`
	DownPaymentRate              *float64           `json:"downPaymentRate,omitempty"`
	TaxAmounts                   map[string]float64 `json:"taxAmounts,omitempty"`
}

func newCalculationOverridesResponse(value domaincapacity.CalculationOverrides) calculationOverridesResponse {
	return calculationOverridesResponse{
		CommercialAnnualInterestRate: value.CommercialAnnualInterestRate,
		ProvidentAnnualInterestRate:  value.ProvidentAnnualInterestRate,
		DownPaymentRate:              value.DownPaymentRate, TaxAmounts: copyFloatMap(value.TaxAmounts),
	}
}

type policySourceResponse struct {
	Code          string `json:"code"`
	Title         string `json:"title"`
	Issuer        string `json:"issuer"`
	URL           string `json:"url"`
	EffectiveDate string `json:"effectiveDate"`
}

func newPolicySourceResponses(values []domaincapacity.PolicySource) []policySourceResponse {
	responses := make([]policySourceResponse, 0, len(values))
	for _, value := range values {
		responses = append(responses, policySourceResponse{
			Code: value.Code, Title: value.Title, Issuer: value.Issuer, URL: value.URL, EffectiveDate: value.EffectiveDate,
		})
	}
	return responses
}

type downPaymentRulesResponse struct {
	CommercialFirst  float64 `json:"commercialFirst"`
	CommercialSecond float64 `json:"commercialSecond"`
	ProvidentFirst   float64 `json:"providentFirst"`
	ProvidentSecond  float64 `json:"providentSecond"`
	CombinedFirst    float64 `json:"combinedFirst"`
	CombinedSecond   float64 `json:"combinedSecond"`
}

type interestRateRulesResponse struct {
	CommercialFirst              float64 `json:"commercialFirst"`
	CommercialSecond             float64 `json:"commercialSecond"`
	ProvidentFirstUpToFiveYears  float64 `json:"providentFirstUpToFiveYears"`
	ProvidentFirstOverFiveYears  float64 `json:"providentFirstOverFiveYears"`
	ProvidentSecondUpToFiveYears float64 `json:"providentSecondUpToFiveYears"`
	ProvidentSecondOverFiveYears float64 `json:"providentSecondOverFiveYears"`
}

type taxRulesResponse struct {
	DeedFirstUpToAreaRate       float64 `json:"deedFirstUpToAreaRate"`
	DeedFirstOverAreaRate       float64 `json:"deedFirstOverAreaRate"`
	DeedSecondUpToAreaRate      float64 `json:"deedSecondUpToAreaRate"`
	DeedSecondOverAreaRate      float64 `json:"deedSecondOverAreaRate"`
	DeedAreaThresholdSQM        float64 `json:"deedAreaThresholdSqm"`
	VATRate                     float64 `json:"vatRate"`
	VATExemptHoldingYears       int     `json:"vatExemptHoldingYears"`
	VATSurchargeRate            float64 `json:"vatSurchargeRate"`
	IncomeTaxGainRate           float64 `json:"incomeTaxGainRate"`
	IncomeTaxAssessedRate       float64 `json:"incomeTaxAssessedRate"`
	IncomeTaxExemptHoldingYears int     `json:"incomeTaxExemptHoldingYears"`
}

type housingPolicyRulesResponse struct {
	DownPayment downPaymentRulesResponse  `json:"downPayment"`
	Interest    interestRateRulesResponse `json:"interest"`
	Tax         taxRulesResponse          `json:"tax"`
}

type housingPolicyResponse struct {
	ID            string                     `json:"id"`
	City          string                     `json:"city"`
	Version       string                     `json:"version"`
	Name          string                     `json:"name"`
	EffectiveFrom string                     `json:"effectiveFrom"`
	EffectiveTo   *string                    `json:"effectiveTo"`
	Enabled       bool                       `json:"enabled"`
	Rules         housingPolicyRulesResponse `json:"rules"`
	Sources       []policySourceResponse     `json:"sources"`
	CreatedAt     string                     `json:"createdAt"`
}

func newHousingPolicyResponse(value domaincapacity.HousingPolicyVersion) housingPolicyResponse {
	return housingPolicyResponse{
		ID: value.ID, City: value.City, Version: value.Version, Name: value.Name,
		EffectiveFrom: value.EffectiveFrom, EffectiveTo: value.EffectiveTo, Enabled: value.Enabled,
		Rules: newHousingPolicyRulesResponse(value.Rules), Sources: newPolicySourceResponses(value.Sources),
		CreatedAt: value.CreatedAt.UTC().Format(time.RFC3339),
	}
}

func newHousingPolicyRulesResponse(value domaincapacity.HousingPolicyRules) housingPolicyRulesResponse {
	return housingPolicyRulesResponse{
		DownPayment: downPaymentRulesResponse{
			CommercialFirst: value.DownPayment.CommercialFirst, CommercialSecond: value.DownPayment.CommercialSecond,
			ProvidentFirst: value.DownPayment.ProvidentFirst, ProvidentSecond: value.DownPayment.ProvidentSecond,
			CombinedFirst: value.DownPayment.CombinedFirst, CombinedSecond: value.DownPayment.CombinedSecond,
		},
		Interest: interestRateRulesResponse{
			CommercialFirst: value.Interest.CommercialFirst, CommercialSecond: value.Interest.CommercialSecond,
			ProvidentFirstUpToFiveYears:  value.Interest.ProvidentFirstUpToFiveYears,
			ProvidentFirstOverFiveYears:  value.Interest.ProvidentFirstOverFiveYears,
			ProvidentSecondUpToFiveYears: value.Interest.ProvidentSecondUpToFiveYears,
			ProvidentSecondOverFiveYears: value.Interest.ProvidentSecondOverFiveYears,
		},
		Tax: taxRulesResponse{
			DeedFirstUpToAreaRate: value.Tax.DeedFirstUpToAreaRate, DeedFirstOverAreaRate: value.Tax.DeedFirstOverAreaRate,
			DeedSecondUpToAreaRate: value.Tax.DeedSecondUpToAreaRate, DeedSecondOverAreaRate: value.Tax.DeedSecondOverAreaRate,
			DeedAreaThresholdSQM: value.Tax.DeedAreaThresholdSQM, VATRate: value.Tax.VATRate,
			VATExemptHoldingYears: value.Tax.VATExemptHoldingYears, VATSurchargeRate: value.Tax.VATSurchargeRate,
			IncomeTaxGainRate: value.Tax.IncomeTaxGainRate, IncomeTaxAssessedRate: value.Tax.IncomeTaxAssessedRate,
			IncomeTaxExemptHoldingYears: value.Tax.IncomeTaxExemptHoldingYears,
		},
	}
}

type loanOptionResponse struct {
	Type                         domaincapacity.LoanType `json:"type"`
	DownPaymentRate              float64                 `json:"downPaymentRate"`
	CommercialAnnualInterestRate *float64                `json:"commercialAnnualInterestRate,omitempty"`
	ProvidentAnnualInterestRate  *float64                `json:"providentAnnualInterestRate,omitempty"`
}

func newLoanOptionResponses(values []appcapacity.LoanOption) []loanOptionResponse {
	responses := make([]loanOptionResponse, 0, len(values))
	for _, value := range values {
		responses = append(responses, loanOptionResponse{
			Type: value.Type, DownPaymentRate: value.DownPaymentRate,
			CommercialAnnualInterestRate: value.CommercialAnnualInterestRate,
			ProvidentAnnualInterestRate:  value.ProvidentAnnualInterestRate,
		})
	}
	return responses
}

type loanComponentResponse struct {
	Type               domaincapacity.LoanType `json:"type"`
	Principal          float64                 `json:"principal"`
	AnnualInterestRate float64                 `json:"annualInterestRate"`
	MonthlyPayment     float64                 `json:"monthlyPayment"`
	SourceCode         string                  `json:"sourceCode"`
	ManualOverride     bool                    `json:"manualOverride"`
}

type loanBreakdownResponse struct {
	Type            domaincapacity.LoanType        `json:"type"`
	TotalPrincipal  float64                        `json:"totalPrincipal"`
	LoanTermMonths  int                            `json:"loanTermMonths"`
	RepaymentMethod domaincapacity.RepaymentMethod `json:"repaymentMethod"`
	Components      []loanComponentResponse        `json:"components"`
	MonthlyPayment  float64                        `json:"monthlyPayment"`
}

func newLoanBreakdownResponse(value domaincapacity.LoanBreakdown) loanBreakdownResponse {
	components := make([]loanComponentResponse, 0, len(value.Components))
	for _, component := range value.Components {
		components = append(components, loanComponentResponse{
			Type: component.Type, Principal: component.Principal, AnnualInterestRate: component.AnnualInterestRate,
			MonthlyPayment: component.MonthlyPayment, SourceCode: component.SourceCode, ManualOverride: component.ManualOverride,
		})
	}
	return loanBreakdownResponse{
		Type: value.Type, TotalPrincipal: value.TotalPrincipal, LoanTermMonths: value.LoanTermMonths,
		RepaymentMethod: value.RepaymentMethod, Components: components, MonthlyPayment: value.MonthlyPayment,
	}
}

type taxItemResponse struct {
	Code            string                 `json:"code"`
	Name            string                 `json:"name"`
	StatutorySide   domaincapacity.TaxSide `json:"statutorySide"`
	PaidBy          domaincapacity.TaxSide `json:"paidBy"`
	TaxBase         float64                `json:"taxBase"`
	Rate            float64                `json:"rate"`
	AutomaticAmount float64                `json:"automaticAmount"`
	Amount          float64                `json:"amount"`
	Formula         string                 `json:"formula"`
	Exempt          bool                   `json:"exempt"`
	SourceCode      string                 `json:"sourceCode"`
	ManualOverride  bool                   `json:"manualOverride"`
}

type taxBreakdownResponse struct {
	Items       []taxItemResponse `json:"items"`
	BuyerTotal  float64           `json:"buyerTotal"`
	SellerTotal float64           `json:"sellerTotal"`
	Total       float64           `json:"total"`
}

func newTaxBreakdownResponse(value domaincapacity.TaxBreakdown) taxBreakdownResponse {
	items := make([]taxItemResponse, 0, len(value.Items))
	for _, item := range value.Items {
		items = append(items, taxItemResponse{
			Code: item.Code, Name: item.Name, StatutorySide: item.StatutorySide, PaidBy: item.PaidBy,
			TaxBase: item.TaxBase, Rate: item.Rate, AutomaticAmount: item.AutomaticAmount, Amount: item.Amount,
			Formula: item.Formula, Exempt: item.Exempt, SourceCode: item.SourceCode, ManualOverride: item.ManualOverride,
		})
	}
	return taxBreakdownResponse{Items: items, BuyerTotal: value.BuyerTotal, SellerTotal: value.SellerTotal, Total: value.Total}
}

type policyVersionReferenceResponse struct {
	ID            string  `json:"id"`
	City          string  `json:"city"`
	Version       string  `json:"version"`
	Name          string  `json:"name"`
	EffectiveFrom string  `json:"effectiveFrom"`
	EffectiveTo   *string `json:"effectiveTo"`
}

func newPolicyVersionReferenceResponse(value domaincapacity.PolicyVersionReference) policyVersionReferenceResponse {
	return policyVersionReferenceResponse{
		ID: value.ID, City: value.City, Version: value.Version, Name: value.Name,
		EffectiveFrom: value.EffectiveFrom, EffectiveTo: value.EffectiveTo,
	}
}

type appliedManualOverrideResponse struct {
	Field          string  `json:"field"`
	AutomaticValue float64 `json:"automaticValue"`
	AppliedValue   float64 `json:"appliedValue"`
}

func newAppliedManualOverrideResponses(values []domaincapacity.AppliedManualOverride) []appliedManualOverrideResponse {
	responses := make([]appliedManualOverrideResponse, 0, len(values))
	for _, value := range values {
		responses = append(responses, appliedManualOverrideResponse{
			Field: value.Field, AutomaticValue: value.AutomaticValue, AppliedValue: value.AppliedValue,
		})
	}
	return responses
}

type policySourceRequest struct {
	Code          string `json:"code"`
	Title         string `json:"title"`
	Issuer        string `json:"issuer"`
	URL           string `json:"url"`
	EffectiveDate string `json:"effectiveDate"`
}

type housingPolicyRequest struct {
	City          string                     `json:"city"`
	Version       string                     `json:"version"`
	Name          string                     `json:"name"`
	EffectiveFrom string                     `json:"effectiveFrom"`
	EffectiveTo   *string                    `json:"effectiveTo"`
	Enabled       *bool                      `json:"enabled"`
	Rules         housingPolicyRulesResponse `json:"rules"`
	Sources       []policySourceRequest      `json:"sources"`
}

func (request housingPolicyRequest) domain() domaincapacity.HousingPolicyVersion {
	enabled := true
	if request.Enabled != nil {
		enabled = *request.Enabled
	}
	sources := make([]domaincapacity.PolicySource, 0, len(request.Sources))
	for _, source := range request.Sources {
		sources = append(sources, domaincapacity.PolicySource{
			Code: source.Code, Title: source.Title, Issuer: source.Issuer, URL: source.URL, EffectiveDate: source.EffectiveDate,
		})
	}
	return domaincapacity.HousingPolicyVersion{
		City: request.City, Version: request.Version, Name: request.Name,
		EffectiveFrom: request.EffectiveFrom, EffectiveTo: request.EffectiveTo, Enabled: enabled,
		Rules: domaincapacity.HousingPolicyRules{
			DownPayment: domaincapacity.DownPaymentRules{
				CommercialFirst: request.Rules.DownPayment.CommercialFirst, CommercialSecond: request.Rules.DownPayment.CommercialSecond,
				ProvidentFirst: request.Rules.DownPayment.ProvidentFirst, ProvidentSecond: request.Rules.DownPayment.ProvidentSecond,
				CombinedFirst: request.Rules.DownPayment.CombinedFirst, CombinedSecond: request.Rules.DownPayment.CombinedSecond,
			},
			Interest: domaincapacity.InterestRateRules{
				CommercialFirst: request.Rules.Interest.CommercialFirst, CommercialSecond: request.Rules.Interest.CommercialSecond,
				ProvidentFirstUpToFiveYears:  request.Rules.Interest.ProvidentFirstUpToFiveYears,
				ProvidentFirstOverFiveYears:  request.Rules.Interest.ProvidentFirstOverFiveYears,
				ProvidentSecondUpToFiveYears: request.Rules.Interest.ProvidentSecondUpToFiveYears,
				ProvidentSecondOverFiveYears: request.Rules.Interest.ProvidentSecondOverFiveYears,
			},
			Tax: domaincapacity.TaxRules{
				DeedFirstUpToAreaRate: request.Rules.Tax.DeedFirstUpToAreaRate, DeedFirstOverAreaRate: request.Rules.Tax.DeedFirstOverAreaRate,
				DeedSecondUpToAreaRate: request.Rules.Tax.DeedSecondUpToAreaRate, DeedSecondOverAreaRate: request.Rules.Tax.DeedSecondOverAreaRate,
				DeedAreaThresholdSQM: request.Rules.Tax.DeedAreaThresholdSQM, VATRate: request.Rules.Tax.VATRate,
				VATExemptHoldingYears: request.Rules.Tax.VATExemptHoldingYears, VATSurchargeRate: request.Rules.Tax.VATSurchargeRate,
				IncomeTaxGainRate: request.Rules.Tax.IncomeTaxGainRate, IncomeTaxAssessedRate: request.Rules.Tax.IncomeTaxAssessedRate,
				IncomeTaxExemptHoldingYears: request.Rules.Tax.IncomeTaxExemptHoldingYears,
			},
		},
		Sources: sources,
	}
}

type CapacityPolicyApplication interface {
	ListPolicyVersions(context.Context, appcapacity.ListPolicyVersionsQuery) ([]domaincapacity.HousingPolicyVersion, error)
	CreatePolicyVersion(context.Context, appcapacity.CreatePolicyVersionCommand) (domaincapacity.HousingPolicyVersion, error)
}

type AdminCapacityPolicies struct {
	app CapacityPolicyApplication
}

func NewAdminCapacityPolicies(app CapacityPolicyApplication) AdminCapacityPolicies {
	return AdminCapacityPolicies{app: app}
}

func (h AdminCapacityPolicies) List(c *gin.Context) {
	policies, err := h.app.ListPolicyVersions(c.Request.Context(), appcapacity.ListPolicyVersionsQuery{City: c.Query("city")})
	if err != nil {
		writeError(c, http.StatusInternalServerError, "internal_error", "internal server error")
		return
	}
	items := make([]housingPolicyResponse, 0, len(policies))
	for _, policy := range policies {
		items = append(items, newHousingPolicyResponse(policy))
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h AdminCapacityPolicies) Create(c *gin.Context) {
	var request housingPolicyRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", "request body is invalid")
		return
	}
	policy, err := h.app.CreatePolicyVersion(c.Request.Context(), appcapacity.CreatePolicyVersionCommand{Policy: request.domain()})
	if err != nil {
		switch {
		case errors.Is(err, appcapacity.ErrInvalidPolicy):
			writeError(c, http.StatusBadRequest, "invalid_policy", "policy version is invalid")
		case errors.Is(err, appcapacity.ErrPolicyConflict):
			writeError(c, http.StatusConflict, "policy_conflict", "policy effective range conflicts with an existing version")
		default:
			writeError(c, http.StatusInternalServerError, "internal_error", "internal server error")
		}
		return
	}
	c.JSON(http.StatusCreated, newHousingPolicyResponse(policy))
}

func copyFloatMap(source map[string]float64) map[string]float64 {
	if source == nil {
		return nil
	}
	copy := make(map[string]float64, len(source))
	for key, value := range source {
		copy[key] = value
	}
	return copy
}
