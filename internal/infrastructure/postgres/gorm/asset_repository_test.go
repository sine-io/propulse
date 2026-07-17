package gormrepo

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
	appasset "github.com/sine-io/propulse/internal/application/asset"
	domainasset "github.com/sine-io/propulse/internal/domain/asset"
	migraterunner "github.com/sine-io/propulse/internal/infrastructure/migrate"
)

func TestPropertyAssetPersistenceRoundTripsFactsAndSourceSnapshot(t *testing.T) {
	price := 500.0
	createdAt := time.Date(2026, 7, 17, 8, 0, 0, 0, time.UTC)
	asset := domainasset.Asset{
		ID: "11111111-1111-4111-8111-111111111111", UserID: "user-a", Name: "海河花园 3室2厅",
		Property: domainasset.PropertySnapshot{
			NeighborhoodID: "22222222-2222-4222-8222-222222222222", NeighborhoodName: "海河花园",
			City: "天津", District: "河西区", Layout: "3室2厅", AreaSQM: 118,
			FloorBand: "中楼层", FloorDescription: "中楼层/20层", Orientation: "南北",
			CurrentListingPriceWan: &price,
		},
		OriginalPurchasePriceWan: 260, PurchasedOn: time.Date(2020, 8, 20, 0, 0, 0, 0, time.UTC),
		CurrentLoanBalanceWan: 80, SourceKind: domainasset.SourceMarketListing,
		ListingSource: &domainasset.ListingSourceSnapshot{
			SourceListingID: "room-1", DataSourceID: "33333333-3333-4333-8333-333333333333",
			DataSourceName: "房鉴", DataSourceType: "fangjian", SourceRef: "batch-1",
			CollectionRunID: "44444444-4444-4444-8444-444444444444", SnapshotID: "55555555-5555-4555-8555-555555555555",
			CollectedAt: createdAt.Add(-time.Hour), ListedAt: createdAt.Add(-20 * 24 * time.Hour), QualityStatus: "complete",
		},
		CreatedAt: createdAt, UpdatedAt: createdAt,
	}
	model, err := propertyAssetModel(asset)
	if err != nil {
		t.Fatal(err)
	}
	if !json.Valid(model.SourceSnapshot) || string(model.SourceSnapshot) == "{}" {
		t.Fatalf("SourceSnapshot = %s", model.SourceSnapshot)
	}
	got, err := propertyAssetFromModel(model)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, asset) {
		t.Fatalf("round trip = %#v, want %#v", got, asset)
	}
}

func TestManualPropertyAssetPersistenceUsesEmptySourceSnapshot(t *testing.T) {
	asset := domainasset.Asset{SourceKind: domainasset.SourceManual}
	model, err := propertyAssetModel(asset)
	if err != nil {
		t.Fatal(err)
	}
	if string(model.SourceSnapshot) != "{}" {
		t.Fatalf("manual SourceSnapshot = %s, want {}", model.SourceSnapshot)
	}
}

func TestAssetRepositoryCRUDIsUserScopedAndSoftDeletes(t *testing.T) {
	databaseURL := os.Getenv("PROPULSE_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("PROPULSE_TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	if err := migraterunner.Run(ctx, databaseURL, "up"); err != nil {
		t.Fatalf("Run(up) error = %v", err)
	}
	db, sqlDB, err := Open(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	city := "天津"
	neighborhoodID := uuid.NewString()
	if err := db.Create(&NeighborhoodModel{ID: neighborhoodID, Name: "仓储测试小区", City: &city, Area: "河西区"}).Error; err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	asset, err := domainasset.New(domainasset.Draft{
		ID: uuid.NewString(), UserID: "asset-repository-user", Name: "仓储测试资产",
		Property: domainasset.PropertySnapshot{
			NeighborhoodID: neighborhoodID, NeighborhoodName: "仓储测试小区", City: city,
			District: "河西区", Layout: "2室1厅", AreaSQM: 82,
		},
		OriginalPurchasePriceWan: 180, PurchasedOn: time.Date(2020, 8, 20, 0, 0, 0, 0, time.UTC),
		CurrentLoanBalanceWan: 60, SourceKind: domainasset.SourceManual,
	}, now)
	if err != nil {
		t.Fatal(err)
	}
	repo := NewAssetRepository(db)
	created, err := repo.Create(ctx, asset)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Find(ctx, "another-user", created.ID); !errors.Is(err, appasset.ErrAssetNotFound) {
		t.Fatalf("cross-user Find() error = %v", err)
	}
	items, total, err := repo.List(ctx, created.UserID, 20, 0)
	if err != nil || total != 1 || len(items) != 1 || items[0].ID != created.ID {
		t.Fatalf("List() = %#v/%d, %v", items, total, err)
	}
	created.Name = "仓储测试资产（更新）"
	created.CurrentLoanBalanceWan = 55
	created.UpdatedAt = now.Add(time.Hour)
	updated, err := repo.Update(ctx, created)
	if err != nil || updated.Name != created.Name || updated.CurrentLoanBalanceWan != 55 {
		t.Fatalf("Update() = %#v, %v", updated, err)
	}
	if err := repo.SoftDelete(ctx, "another-user", created.ID); !errors.Is(err, appasset.ErrAssetNotFound) {
		t.Fatalf("cross-user SoftDelete() error = %v", err)
	}
	if err := repo.SoftDelete(ctx, created.UserID, created.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Find(ctx, created.UserID, created.ID); !errors.Is(err, appasset.ErrAssetNotFound) {
		t.Fatalf("deleted Find() error = %v", err)
	}
	var persisted UserPropertyAssetModel
	if err := db.Where("id = ?", created.ID).First(&persisted).Error; err != nil || persisted.DeletedAt == nil {
		t.Fatalf("persisted deleted asset = %#v, %v", persisted, err)
	}
}
