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
	appcapacity "github.com/propulse/propulse/backend/internal/application/capacity"
	domaincapacity "github.com/propulse/propulse/backend/internal/domain/capacity"
)

func TestCreateCapacityCalculationReturnsSummary(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	service := &stubCapacityApplication{
		createRecord: appcapacity.CalculationRecord{
			ID: "calc_123",
			Result: domaincapacity.HousingCapacityResult{
				PressureLevel: domaincapacity.PressureStrained,
				Strategy:      "先卖后买或同步推进",
			},
		},
	}

	engine := gin.New()
	engine.POST("/api/v1/capacity/calculations", NewCapacity(service).CreateCalculation)

	body := `{"cashOnHand":150,"oldHomeValue":320,"oldLoanBalance":80,"monthlyIncome":3.5,"currentMonthlyMortgage":0,"acceptableMonthlyMortgage":1.5,"targetTotalPrice":550,"renovationBudget":40,"transactionCosts":18,"transitionRentCost":5}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/capacity/calculations", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201", rec.Code)
	}

	var response struct {
		ID     string `json:"id"`
		Result struct {
			PressureLevel string `json:"pressureLevel"`
			Strategy      string `json:"strategy"`
		} `json:"result"`
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
}

func TestCreateCapacityCalculationRejectsInvalidJSON(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	engine.POST("/api/v1/capacity/calculations", NewCapacity(&stubCapacityApplication{}).CreateCalculation)

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
	service := &stubCapacityApplication{}
	engine := gin.New()
	engine.POST("/api/v1/capacity/calculations", NewCapacity(service).CreateCalculation)

	body := `{"cashOnHand":150,"oldHomeValue":320,"oldLoanBalance":80,"monthlyIncome":0,"currentMonthlyMortgage":0,"acceptableMonthlyMortgage":1.5,"targetTotalPrice":550,"renovationBudget":40,"transactionCosts":18,"transitionRentCost":5}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/capacity/calculations", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", rec.Code, rec.Body.String())
	}
	if service.createCalled {
		t.Fatal("CreateCalculation was called for invalid numeric input")
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
	engine.GET("/api/v1/capacity/calculations/:id", NewCapacity(service).GetCalculation)

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
	engine.GET("/api/v1/capacity/calculations/:id", NewCapacity(service).GetCalculation)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/capacity/calculations/missing", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

type stubCapacityApplication struct {
	createRecord appcapacity.CalculationRecord
	createErr    error
	createCalled bool
	getRecord    appcapacity.CalculationRecord
	getErr       error
}

func (s *stubCapacityApplication) CreateCalculation(_ context.Context, _ appcapacity.CreateCalculationCommand) (appcapacity.CalculationRecord, error) {
	s.createCalled = true
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
	engine.GET("/api/v1/capacity/calculations/:id", NewCapacity(service).GetCalculation)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/capacity/calculations/calc_123", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
}
