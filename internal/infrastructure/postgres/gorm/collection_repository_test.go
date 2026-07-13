package gormrepo

import (
	"bytes"
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	appcollection "github.com/sine-io/propulse/internal/application/collection"
	appneighborhood "github.com/sine-io/propulse/internal/application/neighborhood"
	domainneighborhood "github.com/sine-io/propulse/internal/domain/neighborhood"
	migraterunner "github.com/sine-io/propulse/internal/infrastructure/migrate"
	"gorm.io/gorm"
)

var _ appcollection.TrustedRepository = (*CollectionRepository)(nil)

func TestCollectionRepositorySaveCollectionRunPersistsRunAndBothObservationTypes(t *testing.T) {
	ctx, db, repo := openCollectionRepositoryTest(t)
	source, neighborhood := createCollectionRepositoryFixtures(t, ctx, repo, db)
	batch := collectionRepositoryBatch(source.ID, neighborhood.ID)

	result, err := repo.SaveCollectionRun(ctx, batch)
	if err != nil {
		t.Fatalf("SaveCollectionRun() error = %v", err)
	}
	if !result.Created {
		t.Fatal("SaveCollectionRun() Created = false, want true")
	}
	if result.Run.ID != batch.Run.ID {
		t.Fatalf("SaveCollectionRun() Run.ID = %q, want %q", result.Run.ID, batch.Run.ID)
	}

	var run CollectionRunModel
	if err := db.WithContext(ctx).First(&run, "id = ?", batch.Run.ID).Error; err != nil {
		t.Fatalf("Find(collection run) error = %v", err)
	}
	if run.DataSourceID != source.ID || run.NeighborhoodID != neighborhood.ID || run.SourceRef != batch.Run.SourceRef || run.RawContentType != "application/vnd.propulse.test+json" {
		t.Fatalf("persisted run identity = %#v", run)
	}
	if !bytes.Equal(run.RawPayload, batch.Run.RawPayload) {
		t.Fatalf("raw payload = %q, want %q", run.RawPayload, batch.Run.RawPayload)
	}
	if run.ContentChecksum != batch.Run.ContentChecksum {
		t.Fatalf("content checksum = %q, want %q", run.ContentChecksum, batch.Run.ContentChecksum)
	}
	if !bytes.Contains(run.ValidationSummary, []byte(`"issues":[]`)) {
		t.Fatalf("validation summary = %s, want empty issues array", run.ValidationSummary)
	}

	var storedSource DataSourceModel
	if err := db.WithContext(ctx).First(&storedSource, "id = ?", source.ID).Error; err != nil {
		t.Fatalf("Find(data source) error = %v", err)
	}
	if storedSource.Name != source.Name || storedSource.SourceType != source.SourceType || storedSource.City != source.City || storedSource.Notes != source.Notes {
		t.Fatalf("stored source = %#v, want %#v", storedSource, source)
	}

	var listings []ListingObservationModel
	if err := db.WithContext(ctx).Order("source_row ASC").Find(&listings, "collection_run_id = ?", batch.Run.ID).Error; err != nil {
		t.Fatalf("Find(listing observations) error = %v", err)
	}
	if len(listings) != 1 {
		t.Fatalf("listing observations = %d, want 1", len(listings))
	}
	if listings[0].CollectionRunID != batch.Run.ID || listings[0].NeighborhoodID != neighborhood.ID || listings[0].SourceListingID != "listing-source-1" || listings[0].SourceRow != 1 || listings[0].Layout != "三房" || listings[0].AreaSQM != 89.5 || listings[0].ListingPrice != 520 || listings[0].DaysOnMarket != 78 || listings[0].Status != string(appcollection.ListingStatusActive) {
		t.Fatalf("listing observation = %#v", listings[0])
	}
	if !bytes.Contains(listings[0].Attributes, []byte(`"orientation":"south"`)) {
		t.Fatalf("listing attributes = %s", listings[0].Attributes)
	}

	var transactions []TransactionObservationModel
	if err := db.WithContext(ctx).Order("source_row ASC").Find(&transactions, "collection_run_id = ?", batch.Run.ID).Error; err != nil {
		t.Fatalf("Find(transaction observations) error = %v", err)
	}
	if len(transactions) != 1 {
		t.Fatalf("transaction observations = %d, want 1", len(transactions))
	}
	if transactions[0].CollectionRunID != batch.Run.ID || transactions[0].NeighborhoodID != neighborhood.ID || transactions[0].SourceRecordID != "transaction-source-1" || transactions[0].SourceRow != 2 || transactions[0].Layout != "三房" || transactions[0].AreaSQM != 88.2 || transactions[0].TransactionPrice != 495 || transactions[0].OriginalListingRef == nil || *transactions[0].OriginalListingRef != "listing-source-1" {
		t.Fatalf("transaction observation = %#v", transactions[0])
	}
}

func TestCollectionRepositorySaveCollectionRunReturnsExistingRunForDuplicateIdentity(t *testing.T) {
	ctx, db, repo := openCollectionRepositoryTest(t)
	source, neighborhood := createCollectionRepositoryFixtures(t, ctx, repo, db)
	first := collectionRepositoryBatch(source.ID, neighborhood.ID)
	firstResult, err := repo.SaveCollectionRun(ctx, first)
	if err != nil {
		t.Fatalf("SaveCollectionRun(first) error = %v", err)
	}
	if !firstResult.Created {
		t.Fatal("SaveCollectionRun(first) Created = false, want true")
	}

	replay := collectionRepositoryBatch(source.ID, neighborhood.ID)
	replay.Run.ID = uuid.NewString()
	replay.Listings[0].ID = uuid.NewString()
	replay.Transactions[0].ID = uuid.NewString()
	replayResult, err := repo.SaveCollectionRun(ctx, replay)
	if err != nil {
		t.Fatalf("SaveCollectionRun(replay) error = %v", err)
	}
	if replayResult.Created {
		t.Fatal("SaveCollectionRun(replay) Created = true, want false")
	}
	if replayResult.Run.ID != first.Run.ID {
		t.Fatalf("SaveCollectionRun(replay) Run.ID = %q, want existing %q", replayResult.Run.ID, first.Run.ID)
	}

	assertCollectionRepositoryRowCount(t, ctx, db, &CollectionRunModel{}, "data_source_id = ? AND source_ref = ? AND content_checksum = ?", 1, source.ID, first.Run.SourceRef, first.Run.ContentChecksum)
	assertCollectionRepositoryRowCount(t, ctx, db, &ListingObservationModel{}, "collection_run_id = ?", 1, first.Run.ID)
	assertCollectionRepositoryRowCount(t, ctx, db, &TransactionObservationModel{}, "collection_run_id = ?", 1, first.Run.ID)
}

func TestCollectionRepositorySaveCollectionRunRollsBackWhenChildInsertFails(t *testing.T) {
	ctx, db, repo := openCollectionRepositoryTest(t)
	source, neighborhood := createCollectionRepositoryFixtures(t, ctx, repo, db)
	batch := collectionRepositoryBatch(source.ID, neighborhood.ID)
	batch.Transactions[0].SourceRecordID = ""

	if _, err := repo.SaveCollectionRun(ctx, batch); err == nil {
		t.Fatal("SaveCollectionRun() error = nil, want child insert failure")
	}

	assertCollectionRepositoryRowCount(t, ctx, db, &CollectionRunModel{}, "id = ?", 0, batch.Run.ID)
	assertCollectionRepositoryRowCount(t, ctx, db, &ListingObservationModel{}, "collection_run_id = ?", 0, batch.Run.ID)
	assertCollectionRepositoryRowCount(t, ctx, db, &TransactionObservationModel{}, "collection_run_id = ?", 0, batch.Run.ID)
}

func TestCollectionRepositorySaveCollectionRunAllowsTransactionOnlyBatch(t *testing.T) {
	ctx, db, repo := openCollectionRepositoryTest(t)
	source, neighborhood := createCollectionRepositoryFixtures(t, ctx, repo, db)
	batch := collectionRepositoryBatch(source.ID, neighborhood.ID)
	batch.Listings = nil

	result, err := repo.SaveCollectionRun(ctx, batch)
	if err != nil {
		t.Fatalf("SaveCollectionRun() error = %v", err)
	}
	if !result.Created {
		t.Fatal("SaveCollectionRun() Created = false, want true")
	}

	assertCollectionRepositoryRowCount(t, ctx, db, &CollectionRunModel{}, "id = ?", 1, batch.Run.ID)
	assertCollectionRepositoryRowCount(t, ctx, db, &ListingObservationModel{}, "collection_run_id = ?", 0, batch.Run.ID)
	assertCollectionRepositoryRowCount(t, ctx, db, &TransactionObservationModel{}, "collection_run_id = ?", 1, batch.Run.ID)
}

func TestCollectionRepositoryGetCollectionRunReturnsTraceability(t *testing.T) {
	ctx, db, repo := openCollectionRepositoryTest(t)
	source, neighborhood := createCollectionRepositoryFixtures(t, ctx, repo, db)
	batch := collectionRepositoryBatch(source.ID, neighborhood.ID)
	batch.Listings = append(batch.Listings, appcollection.ListingObservation{
		ID:              uuid.NewString(),
		CollectionRunID: "caller-supplied-wrong-run",
		NeighborhoodID:  "caller-supplied-wrong-neighborhood",
		SourceListingID: "listing-source-0",
		SourceRow:       3,
		Layout:          "两房",
		AreaSQM:         72,
		ListingPrice:    410,
		DaysOnMarket:    12,
		Status:          appcollection.ListingStatusPending,
		CapturedAt:      batch.Run.CollectedAt,
	})
	if _, err := repo.SaveCollectionRun(ctx, batch); err != nil {
		t.Fatalf("SaveCollectionRun() error = %v", err)
	}
	if err := repo.UpdateMetricStatus(ctx, batch.Run.ID, appcollection.MetricStatusCompleted); err != nil {
		t.Fatalf("UpdateMetricStatus() error = %v", err)
	}

	detail, err := repo.GetCollectionRun(ctx, batch.Run.ID)
	if err != nil {
		t.Fatalf("GetCollectionRun() error = %v", err)
	}
	if detail.Source.ID != source.ID || detail.Source.Name != source.Name || detail.Source.City != source.City {
		t.Fatalf("detail source = %#v, want %#v", detail.Source, source)
	}
	if detail.Run.ID != batch.Run.ID || detail.Run.MetricStatus != appcollection.MetricStatusCompleted || !bytes.Equal(detail.Run.RawPayload, batch.Run.RawPayload) {
		t.Fatalf("detail run = %#v", detail.Run)
	}
	if len(detail.Listings) != 2 || detail.Listings[0].SourceRow != 1 || detail.Listings[1].SourceRow != 3 {
		t.Fatalf("detail listings order = %#v", detail.Listings)
	}
	if detail.Listings[0].CollectionRunID != batch.Run.ID || detail.Listings[0].NeighborhoodID != neighborhood.ID {
		t.Fatalf("detail listing should use batch run/neighborhood ids, got %#v", detail.Listings[0])
	}
	if len(detail.Transactions) != 1 || detail.Transactions[0].SourceRow != 2 {
		t.Fatalf("detail transactions = %#v", detail.Transactions)
	}

	_, err = repo.GetCollectionRun(ctx, uuid.NewString())
	if !errors.Is(err, appcollection.ErrCollectionRunNotFound) {
		t.Fatalf("GetCollectionRun(missing) error = %v, want %v", err, appcollection.ErrCollectionRunNotFound)
	}
	if err := repo.UpdateMetricStatus(ctx, uuid.NewString(), appcollection.MetricStatusFailed); !errors.Is(err, appcollection.ErrCollectionRunNotFound) {
		t.Fatalf("UpdateMetricStatus(missing) error = %v, want %v", err, appcollection.ErrCollectionRunNotFound)
	}
}

func TestCollectionRepositoryConcurrentDuplicateImportsCreateOneRun(t *testing.T) {
	ctx, db, repo := openCollectionRepositoryTest(t)
	source, neighborhood := createCollectionRepositoryFixtures(t, ctx, repo, db)
	batch := collectionRepositoryBatch(source.ID, neighborhood.ID)

	var wg sync.WaitGroup
	results := make([]appcollection.SaveImportResult, 2)
	errs := make([]error, 2)
	for i := range results {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			duplicate := collectionRepositoryBatch(source.ID, neighborhood.ID)
			duplicate.Run.ID = uuid.NewString()
			duplicate.Listings[0].ID = uuid.NewString()
			duplicate.Transactions[0].ID = uuid.NewString()
			if index == 0 {
				duplicate = batch
			}
			results[index], errs[index] = repo.SaveCollectionRun(ctx, duplicate)
		}(i)
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("SaveCollectionRun(%d) error = %v", i, err)
		}
	}

	created := 0
	replayed := 0
	runIDs := map[string]bool{}
	for _, result := range results {
		if result.Created {
			created++
		} else {
			replayed++
		}
		runIDs[result.Run.ID] = true
	}
	if created != 1 || replayed != 1 || len(runIDs) != 1 {
		t.Fatalf("results = %#v, want one created and one replay of same run", results)
	}

	assertCollectionRepositoryRowCount(t, ctx, db, &CollectionRunModel{}, "data_source_id = ? AND source_ref = ? AND content_checksum = ?", 1, source.ID, batch.Run.SourceRef, batch.Run.ContentChecksum)
}

func openCollectionRepositoryTest(t *testing.T) (context.Context, *gorm.DB, *CollectionRepository) {
	t.Helper()
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
	t.Cleanup(func() { _ = sqlDB.Close() })

	return ctx, db, NewCollectionRepository(db)
}

func createCollectionRepositoryFixtures(t *testing.T, ctx context.Context, repo *CollectionRepository, db *gorm.DB) (appcollection.DataSource, appneighborhood.Neighborhood) {
	t.Helper()
	source := appcollection.DataSource{
		ID:         uuid.NewString(),
		Name:       "链家手工源 " + uuid.NewString(),
		SourceType: "manual_json",
		City:       "杭州",
		Notes:      "trusted weekly import",
	}
	createdSource, err := repo.CreateDataSource(ctx, source)
	if err != nil {
		t.Fatalf("CreateDataSource() error = %v", err)
	}
	duplicateSource, err := repo.CreateDataSource(ctx, appcollection.DataSource{
		ID:         uuid.NewString(),
		Name:       source.Name,
		SourceType: "manual_json",
		City:       source.City,
		Notes:      "ignored duplicate notes",
	})
	if err != nil {
		t.Fatalf("CreateDataSource(duplicate) error = %v", err)
	}
	if duplicateSource.ID != createdSource.ID {
		t.Fatalf("duplicate source ID = %q, want %q", duplicateSource.ID, createdSource.ID)
	}
	exists, err := repo.DataSourceExists(ctx, createdSource.ID)
	if err != nil {
		t.Fatalf("DataSourceExists() error = %v", err)
	}
	if !exists {
		t.Fatal("DataSourceExists() = false, want true")
	}
	sources, err := repo.ListDataSources(ctx)
	if err != nil {
		t.Fatalf("ListDataSources() error = %v", err)
	}
	if len(sources) == 0 {
		t.Fatal("ListDataSources() returned no sources")
	}

	neighborhoodRepo := NewNeighborhoodRepository(db)
	neighborhood, err := neighborhoodRepo.CreateNeighborhood(ctx, appneighborhood.CreateNeighborhoodInput{
		ID:           uuid.NewString(),
		Name:         "导入测试小区 " + uuid.NewString(),
		Area:         "测试板块",
		TargetLayout: "三房",
	})
	if err != nil {
		t.Fatalf("CreateNeighborhood() error = %v", err)
	}
	exists, err = repo.NeighborhoodExists(ctx, neighborhood.ID)
	if err != nil {
		t.Fatalf("NeighborhoodExists() error = %v", err)
	}
	if !exists {
		t.Fatal("NeighborhoodExists() = false, want true")
	}
	return createdSource, neighborhood
}

func collectionRepositoryBatch(dataSourceID string, neighborhoodID string) appcollection.ImportBatch {
	collectedAt := time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC)
	transactionDate := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	originalListingRef := "listing-source-1"
	runID := uuid.NewString()
	return appcollection.ImportBatch{
		Run: appcollection.CollectionRun{
			ID:              runID,
			DataSourceID:    dataSourceID,
			NeighborhoodID:  neighborhoodID,
			SourceRef:       "weekly-2026-07-09",
			CollectedAt:     collectedAt,
			Coverage:        domainneighborhood.CoverageFull,
			Format:          appcollection.ImportFormatJSON,
			ContentChecksum: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
			RawPayload:      []byte(`{"source":"manual","emoji":"小区"}`),
			RawContentType:  "application/vnd.propulse.test+json",
			ValidationSummary: appcollection.ValidationSummary{
				RecordCount:      2,
				ListingCount:     1,
				TransactionCount: 1,
				Issues:           []appcollection.ValidationIssue{},
			},
			Status:       appcollection.CollectionRunStatusCompleted,
			MetricStatus: appcollection.MetricStatusPending,
			CreatedAt:    collectedAt,
			UpdatedAt:    collectedAt,
		},
		Listings: []appcollection.ListingObservation{
			{
				ID:              uuid.NewString(),
				CollectionRunID: "caller-supplied-wrong-run",
				NeighborhoodID:  "caller-supplied-wrong-neighborhood",
				SourceListingID: "listing-source-1",
				SourceRow:       1,
				Layout:          "三房",
				AreaSQM:         89.5,
				ListingPrice:    520,
				DaysOnMarket:    78,
				Status:          appcollection.ListingStatusActive,
				CapturedAt:      time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
				Attributes:      map[string]string{"orientation": "south"},
			},
		},
		Transactions: []appcollection.TransactionObservation{
			{
				ID:                 uuid.NewString(),
				CollectionRunID:    "caller-supplied-wrong-run",
				NeighborhoodID:     "caller-supplied-wrong-neighborhood",
				SourceRecordID:     "transaction-source-1",
				SourceRow:          2,
				Layout:             "三房",
				AreaSQM:            88.2,
				TransactionPrice:   495,
				TransactionDate:    transactionDate,
				OriginalListingRef: &originalListingRef,
				CapturedAt:         time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
			},
		},
	}
}

func assertCollectionRepositoryRowCount(t *testing.T, ctx context.Context, db *gorm.DB, model any, where string, want int64, args ...any) {
	t.Helper()
	var got int64
	if err := db.WithContext(ctx).Model(model).Where(where, args...).Count(&got).Error; err != nil {
		t.Fatalf("Count(%T) error = %v", model, err)
	}
	if got != want {
		t.Fatalf("Count(%T) = %d, want %d", model, got, want)
	}
}
