package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	appdecision "github.com/propulse/propulse/backend/internal/application/decision"
	domaindecision "github.com/propulse/propulse/backend/internal/domain/decision"
)

func TestGetActionWindowReturnsRecommendation(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	service := &stubDecisionApplication{
		result: domaindecision.ActionWindowResult{
			Action:     domaindecision.ActionBargain,
			Confidence: domaindecision.ConfidenceHigh,
			Summary:    "预算仍可服务，且目标小区供应与降价信号支持买方试探底价。",
			Checklist:  []string{"约看 3 套成交区间附近、挂牌超过 60 天的目标户型。"},
			Risks:      []string{"预算不是完全宽松，砍价失败时不要上调总价硬追。"},
		},
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
		Action     string   `json:"action"`
		Confidence string   `json:"confidence"`
		Summary    string   `json:"summary"`
		Checklist  []string `json:"checklist"`
		Risks      []string `json:"risks"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if response.Action != "砍价" || response.Confidence != "高" || response.Summary != "预算仍可服务，且目标小区供应与降价信号支持买方试探底价。" {
		t.Fatalf("response = %#v", response)
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
	result domaindecision.ActionWindowResult
	err    error
}

func (s *stubDecisionApplication) GetActionWindow(_ context.Context, query appdecision.GetActionWindowQuery) (domaindecision.ActionWindowResult, error) {
	s.query = query
	if s.err != nil {
		return domaindecision.ActionWindowResult{}, s.err
	}
	return s.result, nil
}
