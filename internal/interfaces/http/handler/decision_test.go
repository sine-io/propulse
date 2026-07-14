package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	appdecision "github.com/sine-io/propulse/internal/application/decision"
	domaincapacity "github.com/sine-io/propulse/internal/domain/capacity"
	domaindecision "github.com/sine-io/propulse/internal/domain/decision"
	domainneighborhood "github.com/sine-io/propulse/internal/domain/neighborhood"
)

func TestGetActionWindowReturnsRecommendation(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	service := &stubDecisionApplication{
		result: decisionResultFixture(),
	}
	engine := gin.New()
	engine.GET("/api/v1/decision/action-window", NewDecision(service).GetActionWindow)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/decision/action-window?neighborhoodId=neighborhood_2", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if service.query.NeighborhoodID != "neighborhood_2" {
		t.Fatalf("NeighborhoodID = %q, want neighborhood_2", service.query.NeighborhoodID)
	}

	var response struct {
		Action            string   `json:"action"`
		Confidence        string   `json:"confidence"`
		ConfidenceReasons []string `json:"confidenceReasons"`
		Summary           string   `json:"summary"`
		Target            struct {
			NeighborhoodID string `json:"neighborhoodId"`
			Name           string `json:"name"`
		} `json:"target"`
		CapacityCalculation struct {
			ID        string `json:"id"`
			CreatedAt string `json:"createdAt"`
		} `json:"capacityCalculation"`
		Metric struct {
			ID              string `json:"id"`
			CollectionRunID string `json:"collectionRunId"`
			CollectedAt     string `json:"collectedAt"`
		} `json:"metric"`
		Factors []struct {
			Key    string `json:"key"`
			Status string `json:"status"`
			Source *struct {
				Type       string `json:"type"`
				ID         string `json:"id"`
				ObservedAt string `json:"observedAt"`
			} `json:"source"`
			Evidence []struct {
				Key         string   `json:"key"`
				ValueType   string   `json:"valueType"`
				NumberValue *float64 `json:"numberValue"`
			} `json:"evidence"`
		} `json:"factors"`
		Checklist []string `json:"checklist"`
		Risks     []string `json:"risks"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if response.Action != "砍价" || response.Confidence != "高" || response.Summary != "预算仍可服务，且目标小区供应与降价信号支持买方试探底价。" {
		t.Fatalf("response = %#v", response)
	}
	if len(response.ConfidenceReasons) != 1 || response.Target.NeighborhoodID != "11111111-1111-1111-1111-111111111111" || response.Target.Name != "青枫花园" {
		t.Fatalf("traceable response = %#v", response)
	}
	if response.CapacityCalculation.ID != "22222222-2222-2222-2222-222222222222" || response.CapacityCalculation.CreatedAt != "2026-07-14T07:30:00Z" || response.Metric.ID != "33333333-3333-3333-3333-333333333333" || response.Metric.CollectionRunID != "44444444-4444-4444-4444-444444444444" || response.Metric.CollectedAt != "2026-07-14T08:00:00Z" {
		t.Fatalf("source references = %#v", response)
	}
	if len(response.Factors) != 6 || response.Factors[0].Key != "budget_pressure" || response.Factors[0].Source == nil || response.Factors[0].Source.ID != "22222222-2222-2222-2222-222222222222" || response.Factors[0].Source.ObservedAt != "2026-07-14T07:30:00Z" || response.Factors[0].Evidence[0].NumberValue == nil || *response.Factors[0].Evidence[0].NumberValue != 32 {
		t.Fatalf("budget factor = %#v", response.Factors)
	}
	if response.Factors[5].Key != "alternatives" || response.Factors[5].Status != "unknown" || response.Factors[5].Source != nil || len(response.Factors[5].Evidence) != 0 {
		t.Fatalf("alternatives factor = %#v", response.Factors[5])
	}
	if len(response.Checklist) != 1 || response.Checklist[0] != "约看 3 套成交区间附近、挂牌超过 60 天的目标户型。" {
		t.Fatalf("checklist = %#v", response.Checklist)
	}
	if len(response.Risks) != 1 || response.Risks[0] != "预算不是完全宽松，砍价失败时不要上调总价硬追。" {
		t.Fatalf("risks = %#v", response.Risks)
	}
}

func TestGetActionWindowReturnsCapacityRequired(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	engine.GET("/api/v1/decision/action-window", NewDecision(&stubDecisionApplication{err: appdecision.ErrCapacityRequired}).GetActionWindow)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/decision/action-window", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", rec.Code, rec.Body.String())
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
	if response.Error.Code != "capacity_required" {
		t.Fatalf("code = %q, want capacity_required", response.Error.Code)
	}
	if response.Error.Message != "create a capacity calculation before requesting an action window" {
		t.Fatalf("message = %q", response.Error.Message)
	}
}

func TestGetActionWindowMapsExpectedApplicationErrors(t *testing.T) {
	tests := []struct {
		name       string
		appErr     error
		wantStatus int
		wantCode   string
	}{
		{name: "watchlist required", appErr: appdecision.ErrWatchlistRequired, wantStatus: http.StatusBadRequest, wantCode: "watchlist_required"},
		{name: "invalid neighborhood ID", appErr: appdecision.ErrInvalidNeighborhoodID, wantStatus: http.StatusBadRequest, wantCode: "invalid_neighborhood_id"},
		{name: "metric required", appErr: appdecision.ErrMetricRequired, wantStatus: http.StatusNotFound, wantCode: "metric_required"},
		{name: "metric stale", appErr: appdecision.ErrMetricStale, wantStatus: http.StatusConflict, wantCode: "metric_stale"},
		{name: "metric insufficient", appErr: appdecision.ErrMetricInsufficient, wantStatus: http.StatusConflict, wantCode: "metric_insufficient"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gin.SetMode(gin.ReleaseMode)
			engine := gin.New()
			engine.GET("/api/v1/decision/action-window", NewDecision(&stubDecisionApplication{err: tt.appErr}).GetActionWindow)

			req := httptest.NewRequest(http.MethodGet, "/api/v1/decision/action-window", nil)
			rec := httptest.NewRecorder()
			engine.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; body=%s", rec.Code, tt.wantStatus, rec.Body.String())
			}
			var response struct {
				Error struct {
					Code string `json:"code"`
				} `json:"error"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
				t.Fatalf("json.Unmarshal() error = %v", err)
			}
			if response.Error.Code != tt.wantCode {
				t.Fatalf("error code = %q, want %q", response.Error.Code, tt.wantCode)
			}
		})
	}
}

func TestGetActionWindowReturnsServerError(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	engine.GET("/api/v1/decision/action-window", NewDecision(&stubDecisionApplication{err: errors.New("boom")}).GetActionWindow)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/decision/action-window", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
}

type stubDecisionApplication struct {
	query  appdecision.GetActionWindowQuery
	result appdecision.ActionWindowResult
	err    error
}

func (s *stubDecisionApplication) GetActionWindow(_ context.Context, query appdecision.GetActionWindowQuery) (appdecision.ActionWindowResult, error) {
	s.query = query
	if s.err != nil {
		return appdecision.ActionWindowResult{}, s.err
	}
	return s.result, nil
}

func decisionResultFixture() appdecision.ActionWindowResult {
	calculationTime := time.Date(2026, 7, 14, 7, 30, 0, 0, time.UTC)
	collectedAt := time.Date(2026, 7, 14, 8, 0, 0, 0, time.UTC)
	calculatedAt := time.Date(2026, 7, 14, 8, 5, 0, 0, time.UTC)
	value := 32.0
	capacitySource := &appdecision.DecisionFactorSource{
		Type: appdecision.FactorSourceCapacityCalculation, ID: "22222222-2222-2222-2222-222222222222", ObservedAt: calculationTime,
	}
	metricSource := &appdecision.DecisionFactorSource{
		Type: appdecision.FactorSourceNeighborhoodMetric, ID: "33333333-3333-3333-3333-333333333333", ObservedAt: collectedAt,
	}
	return appdecision.ActionWindowResult{
		Action:            domaindecision.ActionBargain,
		Confidence:        domaindecision.ConfidenceHigh,
		ConfidenceReasons: []string{"真实证据支持该置信度。"},
		Summary:           "预算仍可服务，且目标小区供应与降价信号支持买方试探底价。",
		Target: appdecision.ActionWindowTarget{
			NeighborhoodID: "11111111-1111-1111-1111-111111111111", Name: "青枫花园", Area: "滨江核心", TargetLayout: "三房",
		},
		CapacityCalculation: appdecision.CapacityCalculationReference{
			ID: "22222222-2222-2222-2222-222222222222", CreatedAt: calculationTime,
			RuleVersion: "capacity/2026.07.14.1", TraceabilityStatus: domaincapacity.TraceabilityComplete,
		},
		Metric: appdecision.DecisionMetricReference{
			ID: "33333333-3333-3333-3333-333333333333", CollectionRunID: "44444444-4444-4444-4444-444444444444",
			AlgorithmVersion: "market-metrics/2026.07.14.1", CollectedAt: collectedAt, CalculatedAt: calculatedAt,
			SourceIDs: []string{"55555555-5555-5555-5555-555555555555"}, ListingSampleCount: 42, TransactionSampleCount: 5,
			Coverage: domainneighborhood.CoverageFull, Freshness: domainneighborhood.FreshnessCurrent,
			QualityState: domainneighborhood.MarketQualitySufficient, QualityWarnings: []domainneighborhood.QualityWarning{},
		},
		Factors: []appdecision.DecisionFactor{
			{Key: appdecision.FactorBudgetPressure, Status: appdecision.FactorStatusCaution, Summary: "资金接近承压区。", Source: capacitySource, Evidence: []appdecision.DecisionFactorEvidence{{Key: "monthly_payment_ratio", Label: "月供收入比", ValueType: appdecision.EvidenceValueNumber, NumberValue: &value, Unit: "%"}}},
			{Key: appdecision.FactorDownPaymentGap, Status: appdecision.FactorStatusPositive, Summary: "没有首付缺口。", Source: capacitySource, Evidence: []appdecision.DecisionFactorEvidence{}},
			{Key: appdecision.FactorMarketSignal, Status: appdecision.FactorStatusPositive, Summary: "小区支持议价。", Source: metricSource, Evidence: []appdecision.DecisionFactorEvidence{}},
			{Key: appdecision.FactorTransactionMomentum, Status: appdecision.FactorStatusPositive, Summary: "成交偏弱。", Source: metricSource, Evidence: []appdecision.DecisionFactorEvidence{}},
			{Key: appdecision.FactorTargetLayoutSupply, Status: appdecision.FactorStatusNeutral, Summary: "户型供给中等。", Source: metricSource, Evidence: []appdecision.DecisionFactorEvidence{}},
			{Key: appdecision.FactorAlternatives, Status: appdecision.FactorStatusUnknown, Summary: "尚未比较备选。", Source: nil, Evidence: []appdecision.DecisionFactorEvidence{}},
		},
		Checklist: []string{"约看 3 套成交区间附近、挂牌超过 60 天的目标户型。"},
		Risks:     []string{"预算不是完全宽松，砍价失败时不要上调总价硬追。"},
	}
}
