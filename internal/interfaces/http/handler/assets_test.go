package handler

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	appasset "github.com/sine-io/propulse/internal/application/asset"
	domainasset "github.com/sine-io/propulse/internal/domain/asset"
)

func TestCreateAssetMapsStrictRequestAndConfiguredUser(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	app := &stubAssetApplication{asset: handlerTestAsset()}
	engine := gin.New()
	engine.POST("/assets", NewAssets(app, "configured-user").Create)
	body := `{"neighborhoodId":"22222222-2222-4222-8222-222222222222","propertySelection":{"mode":"market_listing","roomId":"room-1"},"originalPurchasePriceWan":180,"purchasedOn":"2020-08-20","currentLoanBalanceWan":60}`
	request := httptest.NewRequest(http.MethodPost, "/assets", bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("status/body = %d/%s", recorder.Code, recorder.Body.String())
	}
	if app.create.UserID != "configured-user" || app.create.PropertySelection.RoomID != "room-1" || app.create.PropertySelection.Mode != appasset.PropertySelectionMarketListing {
		t.Fatalf("CreateAsset command = %#v", app.create)
	}
	if !bytes.Contains(recorder.Body.Bytes(), []byte(`"currentListingPriceWan":320`)) || bytes.Contains(recorder.Body.Bytes(), []byte("configured-user")) {
		t.Fatalf("response body = %s", recorder.Body.String())
	}
}

func TestCreateAssetRejectsUnknownFields(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	app := &stubAssetApplication{}
	engine := gin.New()
	engine.POST("/assets", NewAssets(app, "user").Create)
	body := `{"neighborhoodId":"22222222-2222-4222-8222-222222222222","propertySelection":{"mode":"manual","layout":"2室1厅","areaSqm":82},"originalPurchasePriceWan":180,"purchasedOn":"2020-08-20","currentLoanBalanceWan":60,"ownerName":"not-accepted"}`
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/assets", bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest || app.createCalls != 0 {
		t.Fatalf("status/createCalls/body = %d/%d/%s", recorder.Code, app.createCalls, recorder.Body.String())
	}
}

func TestAssetHandlersMapNotFoundAndUnavailable(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	tests := []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{name: "not found", err: appasset.ErrAssetNotFound, status: http.StatusNotFound, code: "not_found"},
		{name: "listing unavailable", err: appasset.ErrListingUnavailable, status: http.StatusConflict, code: "listing_unavailable"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			app := &stubAssetApplication{updateErr: test.err}
			engine := gin.New()
			engine.PATCH("/assets/:id", NewAssets(app, "user").Update)
			request := httptest.NewRequest(http.MethodPatch, "/assets/11111111-1111-4111-8111-111111111111", bytes.NewBufferString(`{"name":"新名称"}`))
			request.Header.Set("Content-Type", "application/json")
			recorder := httptest.NewRecorder()
			engine.ServeHTTP(recorder, request)
			if recorder.Code != test.status || !bytes.Contains(recorder.Body.Bytes(), []byte(test.code)) {
				t.Fatalf("status/body = %d/%s", recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestDeleteAssetReturnsNoContent(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	app := &stubAssetApplication{}
	engine := gin.New()
	engine.DELETE("/assets/:id", NewAssets(app, "user-a").Delete)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodDelete, "/assets/11111111-1111-4111-8111-111111111111", nil))
	if recorder.Code != http.StatusNoContent || app.delete.UserID != "user-a" {
		t.Fatalf("status/command = %d/%#v", recorder.Code, app.delete)
	}
}

type stubAssetApplication struct {
	asset       domainasset.Asset
	create      appasset.CreateAssetCommand
	createCalls int
	createErr   error
	update      appasset.UpdateAssetCommand
	updateErr   error
	delete      appasset.DeleteAssetCommand
	deleteErr   error
}

func (app *stubAssetApplication) CreateAsset(_ context.Context, command appasset.CreateAssetCommand) (domainasset.Asset, error) {
	app.createCalls++
	app.create = command
	return app.asset, app.createErr
}

func (app *stubAssetApplication) UpdateAsset(_ context.Context, command appasset.UpdateAssetCommand) (domainasset.Asset, error) {
	app.update = command
	return app.asset, app.updateErr
}

func (app *stubAssetApplication) DeleteAsset(_ context.Context, command appasset.DeleteAssetCommand) error {
	app.delete = command
	return app.deleteErr
}

func (app *stubAssetApplication) GetAsset(context.Context, appasset.GetAssetQuery) (domainasset.Asset, error) {
	if app.asset.ID == "" {
		return domainasset.Asset{}, appasset.ErrAssetNotFound
	}
	return app.asset, nil
}

func (app *stubAssetApplication) ListAssets(context.Context, appasset.ListAssetsQuery) (appasset.Page, error) {
	items := []domainasset.Asset{}
	if app.asset.ID != "" {
		items = append(items, app.asset)
	}
	return appasset.Page{Items: items, Total: len(items), Page: 1, PageSize: 20}, nil
}

func handlerTestAsset() domainasset.Asset {
	price := 320.0
	return domainasset.Asset{
		ID: "11111111-1111-4111-8111-111111111111", UserID: "configured-user", Name: "海河花园 2室1厅",
		Property: domainasset.PropertySnapshot{
			NeighborhoodID: "22222222-2222-4222-8222-222222222222", NeighborhoodName: "海河花园",
			City: "天津", District: "河西区", Layout: "2室1厅", AreaSQM: 82,
			CurrentListingPriceWan: &price,
		},
		OriginalPurchasePriceWan: 180, PurchasedOn: time.Date(2020, 8, 20, 0, 0, 0, 0, time.UTC),
		CurrentLoanBalanceWan: 60, SourceKind: domainasset.SourceManual,
		CreatedAt: time.Date(2026, 7, 17, 8, 0, 0, 0, time.UTC), UpdatedAt: time.Date(2026, 7, 17, 8, 0, 0, 0, time.UTC),
	}
}
