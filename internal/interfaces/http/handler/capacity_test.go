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

type stubCapacityApplication struct {
	createRecord   appcapacity.CalculationRecord
	createErr      error
	createCalled   bool
	createCommand  appcapacity.CreateCalculationCommand
	getRecord      appcapacity.CalculationRecord
	getErr         error
	assumptions    domaincapacity.Assumptions
	assumptionsErr error
}

func (s *stubCapacityApplication) GetAssumptions(_ context.Context, _ appcapacity.GetAssumptionsQuery) (domaincapacity.Assumptions, error) {
	return s.assumptions, s.assumptionsErr
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
