package handler

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	appcollection "github.com/sine-io/propulse/internal/application/collection"
)

func TestAdminImportsJSONPreservesRawPayloadAndNormalizesRecords(t *testing.T) {
	result := trustedImportResult(false)
	result.TransactionCount = 1
	app := &stubTrustedCollectionApplication{result: result}
	engine := gin.New()
	engine.POST("/imports/json", NewAdminImports(app).CreateJSON)
	body := `{"dataSourceId":"11111111-1111-1111-1111-111111111111","neighborhoodId":"22222222-2222-2222-2222-222222222222","sourceRef":"weekly-1","collectedAt":"2026-07-13T10:00:00Z","coverage":"full","records":[{"recordType":"listing","sourceRecordId":"listing-1","layout":"三房","areaSqm":89.5,"listingPrice":520.25,"daysOnMarket":12,"status":"active"},{"recordType":"transaction","sourceRecordId":"tx-1","layout":"三房","areaSqm":89.5,"transactionPrice":505.5,"transactionDate":"2026-07-01"}]}`

	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/imports/json", bytes.NewBufferString(body)))

	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", recorder.Code, recorder.Body.String())
	}
	if string(app.command.RawPayload) != body || len(app.command.Records) != 2 {
		t.Fatalf("command raw/records = %q / %#v", app.command.RawPayload, app.command.Records)
	}
	if app.command.Records[1].TransactionDate == nil || app.command.Records[1].TransactionDate.Format(time.DateOnly) != "2026-07-01" {
		t.Fatalf("transaction date = %#v", app.command.Records[1].TransactionDate)
	}
	var response importCollectionRunResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.CollectionRunID != app.result.Run.ID || response.AcceptedRecordCount != 2 || response.RejectedRecordCount != 0 {
		t.Fatalf("response counts/id = %#v", response)
	}
}

func TestAdminImportsJSONReturnsOKForReplay(t *testing.T) {
	app := &stubTrustedCollectionApplication{result: trustedImportResult(true)}
	engine := gin.New()
	engine.POST("/imports/json", NewAdminImports(app).CreateJSON)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/imports/json", bytes.NewBufferString(validTrustedImportBody())))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestAdminImportsJSONReturnsValidationDetails(t *testing.T) {
	row := 1
	app := &stubTrustedCollectionApplication{err: &appcollection.ValidationError{Issues: []appcollection.ValidationIssue{{Row: &row, Field: "listingPrice", Code: "required", Message: "listingPrice is required"}}}}
	engine := gin.New()
	engine.POST("/imports/json", NewAdminImports(app).CreateJSON)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/imports/json", bytes.NewBufferString(validTrustedImportBody())))
	if recorder.Code != http.StatusUnprocessableEntity || !bytes.Contains(recorder.Body.Bytes(), []byte(`"details"`)) {
		t.Fatalf("status/body = %d / %s", recorder.Code, recorder.Body.String())
	}
	var response importValidationErrorResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.AcceptedRecordCount != 0 || response.RejectedRecordCount != 1 {
		t.Fatalf("validation counts = %d/%d", response.AcceptedRecordCount, response.RejectedRecordCount)
	}
}

func TestAdminImportsRejectsOversizedBodyBeforeApplication(t *testing.T) {
	app := &stubTrustedCollectionApplication{}
	engine := gin.New()
	engine.POST("/imports/json", NewAdminImports(app).CreateJSON)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/imports/json", bytes.NewReader(make([]byte, maxImportBytes+1))))
	if recorder.Code != http.StatusRequestEntityTooLarge || app.importCalls != 0 {
		t.Fatalf("status/calls = %d/%d", recorder.Code, app.importCalls)
	}
}

func TestAdminImportsGetDetailReturnsBase64Traceability(t *testing.T) {
	runID := "33333333-3333-3333-3333-333333333333"
	app := &stubTrustedCollectionApplication{detail: appcollection.CollectionRunDetail{
		Run: appcollection.CollectionRun{ID: runID, RawPayload: []byte(`{"source":"raw"}`), ValidationSummary: appcollection.ValidationSummary{}},
	}}
	engine := gin.New()
	engine.GET("/imports/:id", NewAdminImports(app).GetDetail)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/imports/"+runID, nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d; body=%s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		RawPayloadBase64 string `json:"rawPayloadBase64"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.RawPayloadBase64 != base64.StdEncoding.EncodeToString(app.detail.Run.RawPayload) {
		t.Fatalf("rawPayloadBase64 = %q", response.RawPayloadBase64)
	}
}

func TestAdminImportsListMapsFiltersPaginationAndSummaries(t *testing.T) {
	from := "2026-07-01T00:00:00Z"
	to := "2026-07-17T23:59:59Z"
	now := time.Date(2026, 7, 17, 8, 0, 0, 0, time.UTC)
	run := appcollection.CollectionRun{
		ID: "33333333-3333-3333-8333-333333333333", DataSourceID: "11111111-1111-1111-1111-111111111111",
		NeighborhoodID: "22222222-2222-2222-2222-222222222222", SourceRef: "weekly-1",
		CollectedAt: now, Coverage: "full", Format: appcollection.ImportFormatJSON,
		Status: appcollection.CollectionRunStatusCompleted, MetricStatus: appcollection.MetricStatusCompleted,
		ValidationSummary: appcollection.ValidationSummary{RecordCount: 3, ListingCount: 2, TransactionCount: 1},
		CreatedAt:         now, UpdatedAt: now,
	}
	app := &stubTrustedCollectionApplication{listPage: appcollection.CollectionRunsPage{
		Items: []appcollection.CollectionRunSummary{{
			Run: run, Source: appcollection.DataSource{ID: run.DataSourceID, Name: "公开来源"},
			NeighborhoodName: "鸣泉花园",
		}},
		Total: 21, Page: 2, PageSize: 10,
	}}
	engine := gin.New()
	engine.GET("/imports", NewAdminImports(app).List)
	path := "/imports?dataSourceId=" + run.DataSourceID + "&neighborhoodId=" + run.NeighborhoodID +
		"&status=completed&metricStatus=completed&from=" + from + "&to=" + to + "&page=2&pageSize=10"
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d; body=%s", recorder.Code, recorder.Body.String())
	}
	if app.listQuery.DataSourceID != run.DataSourceID || app.listQuery.NeighborhoodID != run.NeighborhoodID ||
		app.listQuery.Status != appcollection.CollectionRunStatusCompleted || app.listQuery.MetricStatus != appcollection.MetricStatusCompleted ||
		app.listQuery.Page != 2 || app.listQuery.PageSize != 10 || app.listQuery.From == nil || app.listQuery.To == nil {
		t.Fatalf("list query = %#v", app.listQuery)
	}
	var response collectionRunsPageResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Total != 21 || len(response.Items) != 1 || response.Items[0].NeighborhoodName != "鸣泉花园" ||
		response.Items[0].DetailHref != "/data/imports/"+run.ID {
		t.Fatalf("response = %#v", response)
	}
}

func TestAdminImportsListRejectsInvalidPaginationBeforeApplication(t *testing.T) {
	app := &stubTrustedCollectionApplication{}
	engine := gin.New()
	engine.GET("/imports", NewAdminImports(app).List)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/imports?page=0&pageSize=20", nil))
	if recorder.Code != http.StatusBadRequest || app.listCalls != 0 {
		t.Fatalf("status/calls = %d/%d", recorder.Code, app.listCalls)
	}
}

func validTrustedImportBody() string {
	return `{"dataSourceId":"11111111-1111-1111-1111-111111111111","neighborhoodId":"22222222-2222-2222-2222-222222222222","sourceRef":"weekly-1","collectedAt":"2026-07-13T10:00:00Z","coverage":"full","records":[{"recordType":"listing","sourceRecordId":"listing-1","layout":"三房","areaSqm":89.5,"listingPrice":520,"daysOnMarket":12,"status":"active"}]}`
}

func trustedImportResult(replay bool) appcollection.ImportCollectionRunResult {
	now := time.Date(2026, 7, 13, 10, 0, 0, 0, time.UTC)
	return appcollection.ImportCollectionRunResult{
		Run:          appcollection.CollectionRun{ID: "33333333-3333-3333-3333-333333333333", CollectedAt: now, CreatedAt: now, UpdatedAt: now, MetricStatus: appcollection.MetricStatusCompleted},
		ListingCount: 1, IdempotentReplay: replay, MetricRefreshStatus: appcollection.MetricStatusCompleted,
	}
}

type stubTrustedCollectionApplication struct {
	command     appcollection.ImportCollectionRunCommand
	result      appcollection.ImportCollectionRunResult
	detail      appcollection.CollectionRunDetail
	listPage    appcollection.CollectionRunsPage
	listQuery   appcollection.ListCollectionRunsQuery
	err         error
	importCalls int
	listCalls   int
}

func (s *stubTrustedCollectionApplication) ImportCollectionRun(_ context.Context, command appcollection.ImportCollectionRunCommand) (appcollection.ImportCollectionRunResult, error) {
	s.importCalls++
	s.command = command
	return s.result, s.err
}

func (s *stubTrustedCollectionApplication) GetCollectionRun(context.Context, appcollection.GetCollectionRunQuery) (appcollection.CollectionRunDetail, error) {
	if s.err != nil && !errors.As(s.err, new(*appcollection.ValidationError)) {
		return appcollection.CollectionRunDetail{}, s.err
	}
	return s.detail, nil
}

func (s *stubTrustedCollectionApplication) ListCollectionRuns(_ context.Context, query appcollection.ListCollectionRunsQuery) (appcollection.CollectionRunsPage, error) {
	s.listCalls++
	s.listQuery = query
	if s.listPage.Items == nil {
		s.listPage = appcollection.CollectionRunsPage{Items: []appcollection.CollectionRunSummary{}, Page: 1, PageSize: 20}
	}
	return s.listPage, s.err
}
