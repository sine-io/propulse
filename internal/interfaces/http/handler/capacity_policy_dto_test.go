package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	appcapacity "github.com/sine-io/propulse/internal/application/capacity"
	domaincapacity "github.com/sine-io/propulse/internal/domain/capacity"
)

func TestAdminCapacityPoliciesListsVersionsByCity(t *testing.T) {
	policy := handlerTestPolicy()
	app := &stubPolicyApplication{items: []domaincapacity.HousingPolicyVersion{policy}}
	engine := gin.New()
	engine.GET("/policies", NewAdminCapacityPolicies(app).List)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/policies?city=%E5%A4%A9%E6%B4%A5", nil))

	if recorder.Code != http.StatusOK || app.listQuery.City != "天津" {
		t.Fatalf("status/query = %d/%#v; body=%s", recorder.Code, app.listQuery, recorder.Body.String())
	}
	var response struct {
		Items []housingPolicyResponse `json:"items"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Items) != 1 || response.Items[0].Version != policy.Version || response.Items[0].Sources[0].Code != "commercial_rate" {
		t.Fatalf("response = %#v", response)
	}
}

func TestAdminCapacityPoliciesCreatesAppendOnlyVersion(t *testing.T) {
	created := handlerTestPolicy()
	app := &stubPolicyApplication{created: created}
	engine := gin.New()
	engine.POST("/policies", NewAdminCapacityPolicies(app).Create)
	body := `{
      "city":"天津","version":"tianjin-future","name":"天津未来政策",
      "effectiveFrom":"2027-01-01","effectiveTo":null,"enabled":true,
      "rules":{
        "downPayment":{"commercialFirst":0.15,"commercialSecond":0.15,"providentFirst":0.2,"providentSecond":0.2,"combinedFirst":0.2,"combinedSecond":0.2},
        "interest":{"commercialFirst":0.031,"commercialSecond":0.031,"providentFirstUpToFiveYears":0.021,"providentFirstOverFiveYears":0.026,"providentSecondUpToFiveYears":0.02525,"providentSecondOverFiveYears":0.03075},
        "tax":{"deedFirstUpToAreaRate":0.01,"deedFirstOverAreaRate":0.015,"deedSecondUpToAreaRate":0.01,"deedSecondOverAreaRate":0.02,"deedAreaThresholdSqm":140,"vatRate":0.03,"vatExemptHoldingYears":2,"vatSurchargeRate":0.06,"incomeTaxGainRate":0.2,"incomeTaxAssessedRate":0.01,"incomeTaxExemptHoldingYears":5}
      },
      "sources":[{"code":"official","title":"官方来源","issuer":"主管部门","url":"https://example.com/policy","effectiveDate":"2027-01-01"}]
    }`
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/policies", bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d; body=%s", recorder.Code, recorder.Body.String())
	}
	policy := app.createCommand.Policy
	if policy.City != "天津" || policy.Version != "tianjin-future" || policy.EffectiveFrom != "2027-01-01" || !policy.Enabled {
		t.Fatalf("policy command = %#v", policy)
	}
	if policy.Rules.Tax.DeedAreaThresholdSQM != 140 || len(policy.Sources) != 1 || policy.Sources[0].URL != "https://example.com/policy" {
		t.Fatalf("policy rules/sources = %#v", policy)
	}
}

func TestAdminCapacityPoliciesReturnsConflict(t *testing.T) {
	app := &stubPolicyApplication{err: appcapacity.ErrPolicyConflict}
	engine := gin.New()
	engine.POST("/policies", NewAdminCapacityPolicies(app).Create)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/policies", bytes.NewBufferString(`{"city":"天津"}`))
	request.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusConflict || !bytes.Contains(recorder.Body.Bytes(), []byte(`"policy_conflict"`)) {
		t.Fatalf("status/body = %d/%s", recorder.Code, recorder.Body.String())
	}
}

type stubPolicyApplication struct {
	items         []domaincapacity.HousingPolicyVersion
	created       domaincapacity.HousingPolicyVersion
	err           error
	listQuery     appcapacity.ListPolicyVersionsQuery
	createCommand appcapacity.CreatePolicyVersionCommand
}

func (s *stubPolicyApplication) ListPolicyVersions(_ context.Context, query appcapacity.ListPolicyVersionsQuery) ([]domaincapacity.HousingPolicyVersion, error) {
	s.listQuery = query
	return s.items, s.err
}

func (s *stubPolicyApplication) CreatePolicyVersion(_ context.Context, command appcapacity.CreatePolicyVersionCommand) (domaincapacity.HousingPolicyVersion, error) {
	s.createCommand = command
	return s.created, s.err
}
