package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	appcapacity "github.com/sine-io/propulse/internal/application/capacity"
	"github.com/sine-io/propulse/internal/application/user"
	domaincapacity "github.com/sine-io/propulse/internal/domain/capacity"
)

func handlerTestAssumptions() domaincapacity.Assumptions {
	return domaincapacity.Assumptions{
		RuleVersion: "2026.07.14", EffectiveDate: "2026-07-14", RuleSource: "test rule source",
		Loan: domaincapacity.LoanParams{
			AnnualInterestRate: 0.039, LoanTermMonths: 360, RepaymentMethod: domaincapacity.RepaymentEqualInstallment,
		},
		LoanSource: "test loan source", LoanOrigin: domaincapacity.OriginConfiguredDefault,
		CityPolicy: domaincapacity.CityPolicy{
			City: "测试市", PolicyName: "测试政策", DownPaymentRate: 0.35,
			EffectiveDate: "2026-07-14", Source: "测试政策来源", Origin: domaincapacity.OriginConfiguredDefault,
		},
		ReserveMonths: 6,
		PressureThresholds: domaincapacity.PressureThresholds{
			SafeRatio: 0.35, StrainedRatio: 0.45, DangerRatio: 0.55, DangerMultiplier: 1.15,
		},
		OldHomeShareThreshold: 0.5,
	}
}

func TestCreateCapacityCalculationReturnsCompleteRecord(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	assumptions := handlerTestAssumptions()
	service := &stubCapacityApplication{
		createRecord: appcapacity.CalculationRecord{
			ID: "calc_123",
			Input: domaincapacity.HousingCapacityInput{
				CashOnHand: 150, OldHomeValue: 320, OldLoanBalance: 80, MonthlyIncome: 3.5,
				CurrentMonthlyMortgage: 0, AcceptableMonthlyMortgage: 1.5, TargetTotalPrice: 550,
				RenovationBudget: 40, TransactionCosts: 18, TransitionRentCost: 5,
			},
			Result: domaincapacity.HousingCapacityResult{
				PressureLevel: domaincapacity.PressureStrained, Strategy: "先卖后买或同步推进",
				RuleVersion: "2026.07.14", EffectiveDate: "2026-07-14",
				TraceabilityStatus: domaincapacity.TraceabilityComplete, AppliedAssumptions: &assumptions,
			},
			CreatedAt: time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC),
		},
	}

	engine := gin.New()
	engine.POST("/api/v1/capacity/calculations", NewCapacity(service, user.SingleUserID).CreateCalculation)

	body := `{"cashOnHand":150,"oldHomeValue":320,"oldLoanBalance":80,"monthlyIncome":3.5,"currentMonthlyMortgage":0,"acceptableMonthlyMortgage":1.5,"targetTotalPrice":550,"renovationBudget":40,"transactionCosts":18,"transitionRentCost":5}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/capacity/calculations", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201", rec.Code)
	}

	var response struct {
		ID    string `json:"id"`
		Input struct {
			TargetTotalPrice float64 `json:"targetTotalPrice"`
		} `json:"input"`
		Result struct {
			PressureLevel      string                       `json:"pressureLevel"`
			Strategy           string                       `json:"strategy"`
			TraceabilityStatus string                       `json:"traceabilityStatus"`
			AppliedAssumptions *capacityAssumptionsResponse `json:"appliedAssumptions"`
		} `json:"result"`
		CreatedAt string `json:"createdAt"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if response.ID != "calc_123" {
		t.Fatalf("response.ID = %q, want calc_123", response.ID)
	}
	if response.Result.PressureLevel != "strained" {
		t.Fatalf("response.Result.PressureLevel = %q, want strained", response.Result.PressureLevel)
	}
	if response.Result.Strategy != "先卖后买或同步推进" {
		t.Fatalf("response.Result.Strategy = %q", response.Result.Strategy)
	}
	if response.Input.TargetTotalPrice != 550 || response.CreatedAt != "2026-07-14T12:00:00Z" {
		t.Fatalf("full response = %#v", response)
	}
	if response.Result.TraceabilityStatus != "complete" || response.Result.AppliedAssumptions == nil ||
		response.Result.AppliedAssumptions.CityPolicy.City != "测试市" {
		t.Fatalf("traceable result = %#v", response.Result)
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatalf("json.Unmarshal(raw) error = %v", err)
	}
	result := raw["result"].(map[string]interface{})
	applied := result["appliedAssumptions"].(map[string]interface{})
	if _, exists := applied["downPaymentRate"]; exists {
		t.Fatal("appliedAssumptions must not contain the assumptions API compatibility alias downPaymentRate")
	}
	if service.createCommand.UserID != user.SingleUserID {
		t.Fatalf("CreateCalculation command userID = %q, want %q", service.createCommand.UserID, user.SingleUserID)
	}
}

func TestCreateCapacityCalculationMapsLoanOverride(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	service := &stubCapacityApplication{
		createRecord: appcapacity.CalculationRecord{ID: "calc_1"},
	}
	engine := gin.New()
	engine.POST("/api/v1/capacity/calculations", NewCapacity(service, user.SingleUserID).CreateCalculation)

	body := `{"cashOnHand":150,"oldHomeValue":320,"oldLoanBalance":80,"monthlyIncome":3.5,"currentMonthlyMortgage":0,"acceptableMonthlyMortgage":1.5,"targetTotalPrice":500,"renovationBudget":40,"transactionCosts":18,"transitionRentCost":5,"loanOverride":{"annualInterestRate":0.045,"loanTermMonths":240,"repaymentMethod":"equal_installment"}}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/capacity/calculations", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", rec.Code, rec.Body.String())
	}
	override := service.createCommand.Input.LoanOverride
	if override == nil {
		t.Fatal("LoanOverride = nil, want mapped loan params")
	}
	if override.AnnualInterestRate != 0.045 || override.LoanTermMonths != 240 ||
		override.RepaymentMethod != domaincapacity.RepaymentEqualInstallment {
		t.Fatalf("LoanOverride = %#v, want 0.045/240/equal_installment", *override)
	}
}

func TestCreateCapacityCalculationMapsCityPolicyOverride(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	service := &stubCapacityApplication{createRecord: appcapacity.CalculationRecord{ID: "calc_1"}}
	engine := gin.New()
	engine.POST("/api/v1/capacity/calculations", NewCapacity(service, user.SingleUserID).CreateCalculation)

	body := `{"cashOnHand":150,"oldHomeValue":320,"oldLoanBalance":80,"monthlyIncome":3.5,"currentMonthlyMortgage":0,"acceptableMonthlyMortgage":1.5,"targetTotalPrice":500,"renovationBudget":40,"transactionCosts":18,"transitionRentCost":5,"cityPolicyOverride":{"city":"覆盖市","policyName":"覆盖政策","downPaymentRate":0.4,"effectiveDate":"2026-07-01","source":"用户政策来源"}}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/capacity/calculations", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", rec.Code, rec.Body.String())
	}
	policy := service.createCommand.Input.CityPolicyOverride
	if policy == nil || policy.City != "覆盖市" || policy.PolicyName != "覆盖政策" ||
		policy.DownPaymentRate != 0.4 || policy.EffectiveDate != "2026-07-01" || policy.Source != "用户政策来源" {
		t.Fatalf("CityPolicyOverride = %#v", policy)
	}
}

func TestCreateCapacityCalculationMapsPolicyScenarioLoanAndOverrides(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	service := &stubCapacityApplication{createRecord: appcapacity.CalculationRecord{ID: "calc_policy"}}
	engine := gin.New()
	engine.POST("/api/v1/capacity/calculations", NewCapacity(service, user.SingleUserID).CreateCalculation)

	body := `{"cashOnHand":150,"oldHomeValue":320,"oldLoanBalance":80,"monthlyIncome":3.5,"currentMonthlyMortgage":0,"acceptableMonthlyMortgage":1.5,"targetTotalPrice":500,"renovationBudget":40,"transitionRentCost":5,"transactionScenario":{"city":"天津","homePurchaseOrder":"second","targetHomeType":"resale","targetHomeAreaSqm":141,"oldHomeHoldingYears":2,"oldHomeOnlyFamilyHome":false,"oldHomeOriginalPrice":200,"taxBurdenMode":"buyer_all"},"loanPlan":{"type":"combined","totalLoanAmount":350,"commercialLoanAmount":200,"providentFundLoanAmount":150,"loanTermMonths":300,"repaymentMethod":"equal_principal"},"manualOverrides":{"commercialAnnualInterestRate":0.049,"downPaymentRate":0.3,"taxAmounts":{"deed_tax":6}}}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/capacity/calculations", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", rec.Code, rec.Body.String())
	}
	input := service.createCommand.Input
	if input.TransactionScenario == nil || input.TransactionScenario.City != "天津" ||
		input.TransactionScenario.HomePurchaseOrder != domaincapacity.HomeSecond || input.TransactionScenario.TaxBurdenMode != domaincapacity.TaxBurdenBuyerAll {
		t.Fatalf("TransactionScenario = %#v", input.TransactionScenario)
	}
	if input.LoanPlan == nil || input.LoanPlan.Type != domaincapacity.LoanCombined || input.LoanPlan.CommercialLoanAmount != 200 ||
		input.LoanPlan.ProvidentFundLoanAmount != 150 || input.LoanPlan.RepaymentMethod != domaincapacity.RepaymentEqualPrincipal {
		t.Fatalf("LoanPlan = %#v", input.LoanPlan)
	}
	if input.ManualOverrides == nil || input.ManualOverrides.CommercialAnnualInterestRate == nil ||
		*input.ManualOverrides.CommercialAnnualInterestRate != 0.049 || input.ManualOverrides.TaxAmounts["deed_tax"] != 6 {
		t.Fatalf("ManualOverrides = %#v", input.ManualOverrides)
	}
	if input.TransactionCosts != 0 {
		t.Fatalf("TransactionCosts = %v, want optional zero", input.TransactionCosts)
	}
}

func TestCreateCapacityCalculationMapsPropertySelectionsAndReturnsFrozenContext(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	confirmedAt := time.Date(2026, 7, 17, 8, 0, 0, 0, time.UTC)
	service := &stubCapacityApplication{createRecord: appcapacity.CalculationRecord{
		ID: "calc-selection",
		SelectionContext: &appcapacity.SelectionContext{
			OldHome: &appcapacity.OldHomeSelectionSnapshot{Mode: appcapacity.OldHomeNone, ConfirmedAt: confirmedAt},
		},
	}}
	engine := gin.New()
	engine.POST("/api/v1/capacity/calculations", NewCapacity(service, user.SingleUserID).CreateCalculation)
	body := `{"cashOnHand":150,"oldHomeValue":0,"oldLoanBalance":0,"monthlyIncome":3.5,"currentMonthlyMortgage":0,"acceptableMonthlyMortgage":1.5,"targetTotalPrice":480,"renovationBudget":30,"transitionRentCost":5,"transactionScenario":{"city":"天津","homePurchaseOrder":"first","targetHomeType":"resale","targetHomeAreaSqm":118,"oldHomeHoldingYears":0,"oldHomeOnlyFamilyHome":false,"oldHomeOriginalPrice":0,"taxBurdenMode":"statutory"},"loanPlan":{"type":"commercial","totalLoanAmount":400,"loanTermMonths":360,"repaymentMethod":"equal_installment"},"oldHomeSelection":{"mode":"none","priceConfirmed":true},"targetHomeSelection":{"neighborhoodId":"22222222-2222-4222-8222-222222222222","roomId":"room-1","expectedPurchasePriceWan":480,"priceConfirmed":true}}`
	request := httptest.NewRequest(http.MethodPost, "/api/v1/capacity/calculations", bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("status/body = %d/%s", recorder.Code, recorder.Body.String())
	}
	if service.createCommand.OldHomeSelection == nil || service.createCommand.OldHomeSelection.Mode != appcapacity.OldHomeNone ||
		service.createCommand.TargetHomeSelection == nil || service.createCommand.TargetHomeSelection.RoomID != "room-1" {
		t.Fatalf("selection command = %#v", service.createCommand)
	}
	if !bytes.Contains(recorder.Body.Bytes(), []byte(`"selectionContext":{"oldHome":{"mode":"none"`)) {
		t.Fatalf("response body = %s", recorder.Body.String())
	}
}

func TestCreateCapacityCalculationRejectsUnknownFields(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	service := &stubCapacityApplication{}
	engine := gin.New()
	engine.POST("/api/v1/capacity/calculations", NewCapacity(service, user.SingleUserID).CreateCalculation)
	body := `{"cashOnHand":150,"oldHomeValue":0,"oldLoanBalance":0,"monthlyIncome":3.5,"currentMonthlyMortgage":0,"acceptableMonthlyMortgage":1.5,"targetTotalPrice":480,"renovationBudget":30,"transactionCosts":10,"transitionRentCost":5,"ownerName":"not-accepted"}`
	request := httptest.NewRequest(http.MethodPost, "/api/v1/capacity/calculations", bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest || service.createCommand.Input.TargetTotalPrice != 0 {
		t.Fatalf("status/command/body = %d/%#v/%s", recorder.Code, service.createCommand, recorder.Body.String())
	}
}

func TestCreateCapacityCalculationRequiresEveryFamilyFieldButAcceptsExplicitZero(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	tests := map[string]struct {
		body       string
		wantStatus int
		wantCalled bool
	}{
		"missing cash on hand": {
			body:       `{"oldHomeValue":320,"oldLoanBalance":80,"monthlyIncome":3.5,"currentMonthlyMortgage":0,"acceptableMonthlyMortgage":1.5,"targetTotalPrice":500,"renovationBudget":40,"transactionCosts":18,"transitionRentCost":5}`,
			wantStatus: http.StatusBadRequest,
		},
		"explicit zero cash on hand": {
			body:       `{"cashOnHand":0,"oldHomeValue":320,"oldLoanBalance":80,"monthlyIncome":3.5,"currentMonthlyMortgage":0,"acceptableMonthlyMortgage":1.5,"targetTotalPrice":500,"renovationBudget":40,"transactionCosts":18,"transitionRentCost":5}`,
			wantStatus: http.StatusCreated,
			wantCalled: true,
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			service := &stubCapacityApplication{createRecord: appcapacity.CalculationRecord{ID: "calc_1"}}
			engine := gin.New()
			engine.POST("/api/v1/capacity/calculations", NewCapacity(service, user.SingleUserID).CreateCalculation)
			req := httptest.NewRequest(http.MethodPost, "/api/v1/capacity/calculations", bytes.NewBufferString(test.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			engine.ServeHTTP(rec, req)

			if rec.Code != test.wantStatus || service.createCalled != test.wantCalled {
				t.Fatalf("status/called = %d/%v, want %d/%v; body=%s", rec.Code, service.createCalled, test.wantStatus, test.wantCalled, rec.Body.String())
			}
		})
	}
}

func TestGetAssumptionsReturnsInjectedCompleteRuleSet(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	assumptions := handlerTestAssumptions()
	engine := gin.New()
	engine.GET("/api/v1/capacity/assumptions", NewCapacity(&stubCapacityApplication{assumptions: assumptions}, user.SingleUserID).GetAssumptions)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/capacity/assumptions", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		RuleVersion     string  `json:"ruleVersion"`
		RuleSource      string  `json:"ruleSource"`
		DownPaymentRate float64 `json:"downPaymentRate"`
		Loan            struct {
			AnnualInterestRate float64 `json:"annualInterestRate"`
			LoanTermMonths     int     `json:"loanTermMonths"`
			RepaymentMethod    string  `json:"repaymentMethod"`
		} `json:"loan"`
		CityPolicy         cityPolicyResponse         `json:"cityPolicy"`
		PressureThresholds pressureThresholdsResponse `json:"pressureThresholds"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body.RuleVersion != assumptions.RuleVersion || body.RuleSource != assumptions.RuleSource {
		t.Fatalf("rule metadata = %q/%q", body.RuleVersion, body.RuleSource)
	}
	if body.Loan.AnnualInterestRate != assumptions.Loan.AnnualInterestRate ||
		body.Loan.LoanTermMonths != assumptions.Loan.LoanTermMonths ||
		body.Loan.RepaymentMethod != string(assumptions.Loan.RepaymentMethod) ||
		body.DownPaymentRate != assumptions.CityPolicy.DownPaymentRate {
		t.Fatalf("loan defaults = %#v, want injected assumptions", body.Loan)
	}
	if body.CityPolicy.City != "测试市" || body.CityPolicy.Source != "测试政策来源" ||
		body.PressureThresholds.DangerRatio != 0.55 {
		t.Fatalf("complete assumptions = %#v", body)
	}
}

func TestGetAssumptionsMapsPolicyQueryAndOptions(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	commercialRate := 0.031
	policy := handlerTestPolicy()
	view := appcapacity.AssumptionsView{
		Legacy: handlerTestAssumptions(), Policy: &policy, HomePurchaseOrder: domaincapacity.HomeSecond,
		LoanTermMonths: 60, Disclaimer: domaincapacity.BudgetEstimateDisclaimer,
		LoanOptions: []appcapacity.LoanOption{{
			Type: domaincapacity.LoanCommercial, DownPaymentRate: 0.25,
			CommercialAnnualInterestRate: &commercialRate,
		}},
	}
	service := &stubCapacityApplication{assumptionsView: &view}
	engine := gin.New()
	engine.GET("/api/v1/capacity/assumptions", NewCapacity(service, user.SingleUserID).GetAssumptions)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/capacity/assumptions?city=%E5%A4%A9%E6%B4%A5&homePurchaseOrder=second&loanTermMonths=60", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if service.assumptionsQuery.City != "天津" || service.assumptionsQuery.HomePurchaseOrder != domaincapacity.HomeSecond || service.assumptionsQuery.LoanTermMonths != 60 {
		t.Fatalf("query = %#v", service.assumptionsQuery)
	}
	var body struct {
		PolicyVersion struct {
			Version string `json:"version"`
		} `json:"policyVersion"`
		LoanOptions []struct {
			Type            string  `json:"type"`
			DownPaymentRate float64 `json:"downPaymentRate"`
		} `json:"loanOptions"`
		Disclaimer string `json:"disclaimer"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.PolicyVersion.Version != policy.Version || len(body.LoanOptions) != 1 || body.LoanOptions[0].DownPaymentRate != 0.25 || body.Disclaimer == "" {
		t.Fatalf("response = %#v", body)
	}
}

func TestCreateCapacityCalculationRejectsInvalidJSON(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	engine.POST("/api/v1/capacity/calculations", NewCapacity(&stubCapacityApplication{}, user.SingleUserID).CreateCalculation)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/capacity/calculations", bytes.NewBufferString(`{"cashOnHand":`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}

	var response struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if response.Error.Code != "invalid_request" {
		t.Fatalf("response.Error.Code = %q, want invalid_request", response.Error.Code)
	}
	if response.Error.Message != "request body is invalid" {
		t.Fatalf("response.Error.Message = %q", response.Error.Message)
	}
}

func TestCreateCapacityCalculationRejectsInvalidNumbers(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	service := &stubCapacityApplication{createErr: domaincapacity.ErrInvalidInput}
	engine := gin.New()
	engine.POST("/api/v1/capacity/calculations", NewCapacity(service, user.SingleUserID).CreateCalculation)

	body := `{"cashOnHand":150,"oldHomeValue":320,"oldLoanBalance":80,"monthlyIncome":0,"currentMonthlyMortgage":0,"acceptableMonthlyMortgage":1.5,"targetTotalPrice":550,"renovationBudget":40,"transactionCosts":18,"transitionRentCost":5}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/capacity/calculations", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", rec.Code, rec.Body.String())
	}
	if !service.createCalled {
		t.Fatal("CreateCalculation was not called for a structurally complete request")
	}

	var response struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if response.Error.Code != "invalid_request" {
		t.Fatalf("response.Error.Code = %q, want invalid_request", response.Error.Code)
	}
}

func TestGetCapacityCalculationReturnsStoredRecord(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	service := &stubCapacityApplication{
		getRecord: appcapacity.CalculationRecord{
			ID: "calc_123",
			Input: domaincapacity.HousingCapacityInput{
				CashOnHand: 150,
			},
			Result: domaincapacity.HousingCapacityResult{
				PressureLevel: domaincapacity.PressureStrained,
				Strategy:      "先卖后买或同步推进",
			},
			CreatedAt: time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC),
		},
	}

	engine := gin.New()
	engine.GET("/api/v1/capacity/calculations/:id", NewCapacity(service, user.SingleUserID).GetCalculation)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/capacity/calculations/calc_123", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var response struct {
		ID     string                              `json:"id"`
		Input  domaincapacity.HousingCapacityInput `json:"input"`
		Result struct {
			PressureLevel string `json:"pressureLevel"`
			Strategy      string `json:"strategy"`
		} `json:"result"`
		CreatedAt string `json:"createdAt"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if response.ID != "calc_123" {
		t.Fatalf("response.ID = %q, want calc_123", response.ID)
	}
	if response.Result.PressureLevel != "strained" {
		t.Fatalf("response.Result.PressureLevel = %q", response.Result.PressureLevel)
	}
	if response.CreatedAt != "2026-07-09T12:00:00Z" {
		t.Fatalf("response.CreatedAt = %q", response.CreatedAt)
	}
}

func TestGetCapacityCalculationReturnsNotFound(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	service := &stubCapacityApplication{getErr: appcapacity.ErrCalculationNotFound}
	engine := gin.New()
	engine.GET("/api/v1/capacity/calculations/:id", NewCapacity(service, user.SingleUserID).GetCalculation)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/capacity/calculations/missing", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestGetCapacityCalculationReturnsLegacyTraceabilityWithoutAssumptions(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	service := &stubCapacityApplication{getRecord: appcapacity.CalculationRecord{
		ID: "calc_legacy",
		Result: domaincapacity.HousingCapacityResult{
			PressureLevel: domaincapacity.PressureSafe, TraceabilityStatus: domaincapacity.TraceabilityLegacyUnversioned,
		},
		CreatedAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
	}}
	engine := gin.New()
	engine.GET("/api/v1/capacity/calculations/:id", NewCapacity(service, user.SingleUserID).GetCalculation)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/capacity/calculations/calc_legacy", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var response struct {
		Result struct {
			TraceabilityStatus string          `json:"traceabilityStatus"`
			AppliedAssumptions json.RawMessage `json:"appliedAssumptions"`
		} `json:"result"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if response.Result.TraceabilityStatus != "legacy_unversioned" || string(response.Result.AppliedAssumptions) != "null" {
		t.Fatalf("legacy result = %#v", response.Result)
	}
}

func TestListCapacityCalculationsReturnsSummaryPage(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	createdAt := time.Date(2026, 7, 17, 8, 30, 0, 0, time.UTC)
	service := &stubCapacityApplication{listResult: appcapacity.CalculationHistoryPage{
		Items: []appcapacity.CalculationSummary{{
			ID: "calc-1", CreatedAt: createdAt, PressureLevel: domaincapacity.PressureStrained,
			TargetTotalPrice: 480, TargetNeighborhoodName: "海河花园", TargetLayout: "3室2厅", OldHomeName: "现住房",
		}},
		Total: 21, Page: 2, PageSize: 20,
	}}
	engine := gin.New()
	engine.GET("/api/v1/capacity/calculations", NewCapacity(service, user.SingleUserID).ListCalculations)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/capacity/calculations?q=%E6%B5%B7%E6%B2%B3&page=2&pageSize=20", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if service.listQuery.UserID != user.SingleUserID || service.listQuery.Query != "海河" || service.listQuery.Page != 2 || service.listQuery.PageSize != 20 {
		t.Fatalf("list query = %#v", service.listQuery)
	}
	var response calculationHistoryPageResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if response.Total != 21 || response.Page != 2 || response.PageSize != 20 || len(response.Items) != 1 {
		t.Fatalf("response = %#v", response)
	}
	item := response.Items[0]
	if item.ID != "calc-1" || item.CreatedAt != "2026-07-17T08:30:00Z" || item.PressureLevel != domaincapacity.PressureStrained ||
		item.TargetTotalPrice != 480 || item.TargetNeighborhoodName != "海河花园" || item.TargetLayout != "3室2厅" || item.OldHomeName != "现住房" {
		t.Fatalf("item = %#v", item)
	}
}

func TestListCapacityCalculationsValidatesPagination(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	for _, query := range []string{"page=0", "page=invalid", "pageSize=0", "pageSize=101"} {
		t.Run(query, func(t *testing.T) {
			service := &stubCapacityApplication{}
			engine := gin.New()
			engine.GET("/api/v1/capacity/calculations", NewCapacity(service, user.SingleUserID).ListCalculations)
			req := httptest.NewRequest(http.MethodGet, "/api/v1/capacity/calculations?"+query, nil)
			rec := httptest.NewRecorder()
			engine.ServeHTTP(rec, req)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", rec.Code)
			}
		})
	}
}

func TestListCapacityCalculationsMapsApplicationErrors(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	for _, tc := range []struct {
		name string
		err  error
		want int
	}{
		{name: "invalid query", err: appcapacity.ErrInvalidCalculationQuery, want: http.StatusBadRequest},
		{name: "repository", err: errors.New("boom"), want: http.StatusInternalServerError},
	} {
		t.Run(tc.name, func(t *testing.T) {
			service := &stubCapacityApplication{listErr: tc.err}
			engine := gin.New()
			engine.GET("/api/v1/capacity/calculations", NewCapacity(service, user.SingleUserID).ListCalculations)
			req := httptest.NewRequest(http.MethodGet, "/api/v1/capacity/calculations", nil)
			rec := httptest.NewRecorder()
			engine.ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d", rec.Code, tc.want)
			}
		})
	}
}

type stubCapacityApplication struct {
	createRecord     appcapacity.CalculationRecord
	createErr        error
	createCalled     bool
	createCommand    appcapacity.CreateCalculationCommand
	getRecord        appcapacity.CalculationRecord
	getErr           error
	assumptions      domaincapacity.Assumptions
	assumptionsView  *appcapacity.AssumptionsView
	assumptionsQuery appcapacity.GetAssumptionsQuery
	assumptionsErr   error
	listResult       appcapacity.CalculationHistoryPage
	listErr          error
	listQuery        appcapacity.ListCalculationsQuery
}

func (s *stubCapacityApplication) GetAssumptions(_ context.Context, query appcapacity.GetAssumptionsQuery) (appcapacity.AssumptionsView, error) {
	s.assumptionsQuery = query
	if s.assumptionsView != nil {
		return *s.assumptionsView, s.assumptionsErr
	}
	return appcapacity.AssumptionsView{Legacy: s.assumptions}, s.assumptionsErr
}

func handlerTestPolicy() domaincapacity.HousingPolicyVersion {
	return domaincapacity.HousingPolicyVersion{
		ID: "66666666-6666-4666-8666-666666666666", City: "天津", Version: "tianjin-test",
		Name: "天津测试政策", EffectiveFrom: "2026-01-01", Enabled: true,
		Rules: domaincapacity.HousingPolicyRules{
			DownPayment: domaincapacity.DownPaymentRules{CommercialFirst: 0.15, CommercialSecond: 0.25},
			Interest:    domaincapacity.InterestRateRules{CommercialFirst: 0.031, CommercialSecond: 0.041},
		},
		Sources: []domaincapacity.PolicySource{{
			Code: "commercial_rate", Title: "商业贷款参考利率", Issuer: "测试机构",
			URL: "https://example.com/policy", EffectiveDate: "2026-01-01",
		}},
		CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}
}

func (s *stubCapacityApplication) CreateCalculation(_ context.Context, command appcapacity.CreateCalculationCommand) (appcapacity.CalculationRecord, error) {
	s.createCalled = true
	s.createCommand = command
	return s.createRecord, s.createErr
}

func (s *stubCapacityApplication) GetCalculation(_ context.Context, _ appcapacity.GetCalculationQuery) (appcapacity.CalculationRecord, error) {
	if s.getErr != nil {
		return appcapacity.CalculationRecord{}, s.getErr
	}
	return s.getRecord, nil
}

func (s *stubCapacityApplication) ListCalculations(_ context.Context, query appcapacity.ListCalculationsQuery) (appcapacity.CalculationHistoryPage, error) {
	s.listQuery = query
	return s.listResult, s.listErr
}

func (s *stubCapacityApplication) LatestCalculation(_ context.Context, _ appcapacity.LatestCalculationQuery) (appcapacity.CalculationRecord, error) {
	if s.getErr != nil {
		return appcapacity.CalculationRecord{}, s.getErr
	}
	return s.getRecord, nil
}

func TestGetCapacityCalculationReturnsServerError(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	service := &stubCapacityApplication{getErr: errors.New("boom")}
	engine := gin.New()
	engine.GET("/api/v1/capacity/calculations/:id", NewCapacity(service, user.SingleUserID).GetCalculation)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/capacity/calculations/calc_123", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
}
