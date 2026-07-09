package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	appcollection "github.com/sine-io/propulse/backend/internal/application/collection"
)

func TestAdminImportsCreatesManualImport(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	service := &stubCollectionApplication{
		result: appcollection.ImportManualListingsResult{
			CollectionRunID:       "collection_run_1",
			ImportedSnapshotCount: 2,
		},
	}
	engine := gin.New()
	engine.POST("/admin/api/imports", NewAdminImports(service).CreateImport)

	req := httptest.NewRequest(http.MethodPost, "/admin/api/imports", bytes.NewBufferString(`{
		"sourceType": "manual_json",
		"sourceRef": "demo-weekly-import",
		"neighborhoodId": "neighborhood_1",
		"records": [
			{"listingPrice": 520, "transactionPrice": 495, "priceCut": true, "daysOnMarket": 78, "layout": "三房"},
			{"listingPrice": 610, "transactionPrice": 0, "priceCut": false, "daysOnMarket": 14, "layout": "三房"}
		]
	}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", rec.Code, rec.Body.String())
	}
	if service.command.SourceType != "manual_json" || service.command.NeighborhoodID != "neighborhood_1" || len(service.command.Records) != 2 {
		t.Fatalf("command = %#v", service.command)
	}

	var response struct {
		CollectionRunID       string `json:"collectionRunId"`
		ImportedSnapshotCount int    `json:"importedSnapshotCount"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if response.CollectionRunID != "collection_run_1" || response.ImportedSnapshotCount != 2 {
		t.Fatalf("response = %#v", response)
	}
}

func TestAdminImportsMapsApplicationErrors(t *testing.T) {
	tests := []struct {
		name       string
		appErr     error
		wantStatus int
		wantCode   string
	}{
		{name: "invalid request", appErr: appcollection.ErrInvalidRequest, wantStatus: http.StatusBadRequest, wantCode: "invalid_request"},
		{name: "neighborhood not found", appErr: appcollection.ErrNeighborhoodNotFound, wantStatus: http.StatusNotFound, wantCode: "neighborhood_not_found"},
		{name: "import failed", appErr: appcollection.ErrImportFailed, wantStatus: http.StatusInternalServerError, wantCode: "import_failed"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gin.SetMode(gin.ReleaseMode)
			engine := gin.New()
			engine.POST("/admin/api/imports", NewAdminImports(&stubCollectionApplication{err: tt.appErr}).CreateImport)

			req := httptest.NewRequest(http.MethodPost, "/admin/api/imports", bytes.NewBufferString(`{
				"sourceType": "manual_json",
				"sourceRef": "demo-weekly-import",
				"neighborhoodId": "neighborhood_1",
				"records": [{"listingPrice": 520, "daysOnMarket": 0}]
			}`))
			req.Header.Set("Content-Type", "application/json")
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

func TestAdminImportsRejectsInvalidJSON(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	service := &stubCollectionApplication{}
	engine := gin.New()
	engine.POST("/admin/api/imports", NewAdminImports(service).CreateImport)

	req := httptest.NewRequest(http.MethodPost, "/admin/api/imports", bytes.NewBufferString(`{`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if service.called {
		t.Fatal("ImportManualListings was called for invalid JSON")
	}
}

type stubCollectionApplication struct {
	command appcollection.ImportManualListingsCommand
	result  appcollection.ImportManualListingsResult
	err     error
	called  bool
}

func (s *stubCollectionApplication) ImportManualListings(_ context.Context, command appcollection.ImportManualListingsCommand) (appcollection.ImportManualListingsResult, error) {
	s.called = true
	s.command = command
	if s.err != nil {
		return appcollection.ImportManualListingsResult{}, s.err
	}
	return s.result, nil
}

var errAdminImportBoom = errors.New("boom")
