package gormrepo

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	appcollection "github.com/propulse/propulse/backend/internal/application/collection"
	appneighborhood "github.com/propulse/propulse/backend/internal/application/neighborhood"
	migraterunner "github.com/propulse/propulse/backend/internal/infrastructure/migrate"
)

func TestCollectionRepositoryStoresRawRecordAndSnapshots(t *testing.T) {
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
		t.Fatalf("Open() error = %v", err)
	}
	defer sqlDB.Close()

	neighborhoodRepo := NewNeighborhoodRepository(db)
	neighborhood, err := neighborhoodRepo.CreateNeighborhood(ctx, appneighborhood.CreateNeighborhoodInput{
		ID:           uuid.NewString(),
		Name:         "导入测试小区",
		Area:         "测试板块",
		TargetLayout: "三房",
	})
	if err != nil {
		t.Fatalf("CreateNeighborhood() error = %v", err)
	}

	repo := NewCollectionRepository(db)
	exists, err := repo.NeighborhoodExists(ctx, neighborhood.ID)
	if err != nil {
		t.Fatalf("NeighborhoodExists() error = %v", err)
	}
	if !exists {
		t.Fatal("NeighborhoodExists() = false, want true")
	}

	rawID := uuid.NewString()
	snapshotID := uuid.NewString()
	transactionPrice := 495.0
	capturedAt := time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC)
	err = repo.SaveImport(ctx, appcollection.RawCollectionRecord{
		ID:          rawID,
		SourceType:  "manual_json",
		SourceRef:   "demo-weekly-import",
		Payload:     []byte(`{"sourceType":"manual_json"}`),
		CollectedAt: capturedAt,
	}, []appcollection.ListingSnapshot{
		{
			ID:               snapshotID,
			NeighborhoodID:   neighborhood.ID,
			ListingPrice:     520,
			TransactionPrice: &transactionPrice,
			PriceCut:         true,
			DaysOnMarket:     78,
			Layout:           "三房",
			CapturedAt:       capturedAt,
		},
	})
	if err != nil {
		t.Fatalf("SaveImport() error = %v", err)
	}

	var rawCount int64
	if err := db.WithContext(ctx).Model(&RawCollectionRecordModel{}).Where("id = ?", rawID).Count(&rawCount).Error; err != nil {
		t.Fatalf("Count(raw) error = %v", err)
	}
	if rawCount != 1 {
		t.Fatalf("rawCount = %d, want 1", rawCount)
	}

	var snapshot ListingSnapshotModel
	if err := db.WithContext(ctx).First(&snapshot, "id = ?", snapshotID).Error; err != nil {
		t.Fatalf("Find(snapshot) error = %v", err)
	}
	if snapshot.NeighborhoodID != neighborhood.ID || snapshot.ListingPrice != 520 || snapshot.TransactionPrice == nil || *snapshot.TransactionPrice != 495 || !snapshot.PriceCut || snapshot.DaysOnMarket != 78 || snapshot.Layout != "三房" {
		t.Fatalf("snapshot = %#v", snapshot)
	}
}
