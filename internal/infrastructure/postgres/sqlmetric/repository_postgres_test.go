package sqlmetric

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	appmetric "github.com/sine-io/propulse/internal/application/metric"
	domainneighborhood "github.com/sine-io/propulse/internal/domain/neighborhood"
	migraterunner "github.com/sine-io/propulse/internal/infrastructure/migrate"
)

func TestRepositoryAggregateCollectionRunUsesOnlyItsFullInventory(t *testing.T) {
	ctx, db, repo := openSQLMetricPostgresTest(t)
	neighborhoodID, sourceID := createMetricFixtures(t, ctx, db)
	oldFull := insertMetricRun(t, ctx, db, sourceID, neighborhoodID, time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC), "full")
	newPartial := insertMetricRun(t, ctx, db, sourceID, neighborhoodID, time.Date(2026, 7, 2, 9, 0, 0, 0, time.UTC), "partial")
	insertMetricListing(t, ctx, db, oldFull, neighborhoodID, "a", "三房", 520, "active", oldFull.collectedAt)
	insertMetricListing(t, ctx, db, oldFull, neighborhoodID, "b", "两房", 610, "active", oldFull.collectedAt)
	insertMetricListing(t, ctx, db, newPartial, neighborhoodID, "c", "三房", 700, "active", newPartial.collectedAt)

	got, err := repo.AggregateMarketObservations(ctx, appmetric.AggregateMarketParams{NeighborhoodID: neighborhoodID, TriggerRunID: newPartial.id, TargetLayout: "三房"})
	if err != nil {
		t.Fatalf("AggregateMarketObservations() error = %v", err)
	}
	if got.InventoryCollectionRunID == nil || *got.InventoryCollectionRunID != oldFull.id {
		t.Fatalf("InventoryCollectionRunID = %#v, want %s", got.InventoryCollectionRunID, oldFull.id)
	}
	if got.ListedHomes != 2 {
		t.Fatalf("ListedHomes = %d, want 2", got.ListedHomes)
	}
}

func TestRepositoryAggregateCollectionRunDerivesPriceCutsFromPriorObservations(t *testing.T) {
	ctx, db, repo := openSQLMetricPostgresTest(t)
	neighborhoodID, sourceID := createMetricFixtures(t, ctx, db)
	oldRun := insertMetricRun(t, ctx, db, sourceID, neighborhoodID, time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC), "full")
	newRun := insertMetricRun(t, ctx, db, sourceID, neighborhoodID, time.Date(2026, 7, 2, 9, 0, 0, 0, time.UTC), "full")
	insertMetricListing(t, ctx, db, oldRun, neighborhoodID, "same-listing", "三房", 620, "active", oldRun.collectedAt)
	insertMetricListing(t, ctx, db, newRun, neighborhoodID, "same-listing", "三房", 590, "active", newRun.collectedAt)

	got, err := repo.AggregateMarketObservations(ctx, appmetric.AggregateMarketParams{NeighborhoodID: neighborhoodID, TriggerRunID: newRun.id, TargetLayout: "三房"})
	if err != nil {
		t.Fatalf("AggregateMarketObservations() error = %v", err)
	}
	if got.PriceCutHomes != 1 {
		t.Fatalf("PriceCutHomes = %d, want 1", got.PriceCutHomes)
	}
}

func TestRepositoryAggregateCollectionRunDoesNotCompareIDsAcrossSources(t *testing.T) {
	ctx, db, repo := openSQLMetricPostgresTest(t)
	neighborhoodID, sourceA := createMetricFixtures(t, ctx, db)
	sourceB := insertMetricSource(t, ctx, db)
	oldRun := insertMetricRun(t, ctx, db, sourceA, neighborhoodID, time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC), "full")
	newRun := insertMetricRun(t, ctx, db, sourceB, neighborhoodID, time.Date(2026, 7, 2, 9, 0, 0, 0, time.UTC), "full")
	insertMetricListing(t, ctx, db, oldRun, neighborhoodID, "same-external-id", "三房", 620, "active", oldRun.collectedAt)
	insertMetricListing(t, ctx, db, newRun, neighborhoodID, "same-external-id", "三房", 590, "active", newRun.collectedAt)

	got, err := repo.AggregateMarketObservations(ctx, appmetric.AggregateMarketParams{NeighborhoodID: neighborhoodID, TriggerRunID: newRun.id, TargetLayout: "三房"})
	if err != nil {
		t.Fatalf("AggregateMarketObservations() error = %v", err)
	}
	if got.PriceCutHomes != 0 {
		t.Fatalf("PriceCutHomes = %d, want 0", got.PriceCutHomes)
	}
}

func TestRepositoryAggregateCollectionRunUsesTransactionsWithinNinetyDays(t *testing.T) {
	ctx, db, repo := openSQLMetricPostgresTest(t)
	neighborhoodID, sourceID := createMetricFixtures(t, ctx, db)
	triggerAt := time.Date(2026, 7, 10, 9, 0, 0, 0, time.UTC)
	run := insertMetricRun(t, ctx, db, sourceID, neighborhoodID, triggerAt, "full")
	insertMetricTransaction(t, ctx, db, run, neighborhoodID, "tx-10", 500, triggerAt.AddDate(0, 0, -10))
	insertMetricTransaction(t, ctx, db, run, neighborhoodID, "tx-45", 520, triggerAt.AddDate(0, 0, -45))
	insertMetricTransaction(t, ctx, db, run, neighborhoodID, "tx-91", 540, triggerAt.AddDate(0, 0, -91))

	got, err := repo.AggregateMarketObservations(ctx, appmetric.AggregateMarketParams{NeighborhoodID: neighborhoodID, TriggerRunID: run.id, TargetLayout: "三房"})
	if err != nil {
		t.Fatalf("AggregateMarketObservations() error = %v", err)
	}
	if got.TransactionSampleCount != 2 || got.LastThirtyDayTransactionCount != 1 || got.PrecedingSixtyDayTransactionCount != 1 {
		t.Fatalf("transaction windows = sample %d last30 %d prev60 %d, want 2/1/1", got.TransactionSampleCount, got.LastThirtyDayTransactionCount, got.PrecedingSixtyDayTransactionCount)
	}
}

func TestRepositoryUpsertNeighborhoodMetricPersistsProvenance(t *testing.T) {
	ctx, db, repo := openSQLMetricPostgresTest(t)
	neighborhoodID, sourceID := createMetricFixtures(t, ctx, db)
	run := insertMetricRun(t, ctx, db, sourceID, neighborhoodID, time.Date(2026, 7, 10, 9, 0, 0, 0, time.UTC), "full")
	avg := 22.0
	min := 500.0
	max := 650.0
	inventoryAt := run.collectedAt

	got, err := repo.UpsertNeighborhoodMetric(ctx, appmetric.MetricSnapshot{
		NeighborhoodID:           neighborhoodID,
		CollectionRunID:          run.id,
		InventoryCollectionRunID: &run.id,
		SourceIDs:                []string{sourceID},
		LatestObservedAt:         run.collectedAt,
		ListedHomes:              6,
		PriceCutHomes:            1,
		AvgDaysOnMarket:          &avg,
		ListingPriceMin:          &min,
		ListingPriceMax:          &max,
		TransactionPriceMin:      &min,
		TransactionPriceMax:      &max,
		TransactionMomentum:      domainneighborhood.TransactionMomentumStable,
		TargetLayoutSupply:       2,
		ListingSampleCount:       6,
		TransactionSampleCount:   3,
		Coverage:                 domainneighborhood.CoverageFull,
		Freshness:                domainneighborhood.FreshnessCurrent,
		InventoryCollectedAt:     &inventoryAt,
		QualityState:             domainneighborhood.MarketQualitySufficient,
	})
	if err != nil {
		t.Fatalf("UpsertNeighborhoodMetric() error = %v", err)
	}
	if got.CollectionRunID != run.id || got.InventoryCollectionRunID == nil || *got.InventoryCollectionRunID != run.id || got.QualityState != domainneighborhood.MarketQualitySufficient {
		t.Fatalf("persisted provenance = %#v", got)
	}
}

func TestRepositoryLatestMetricMapsProvenance(t *testing.T) {
	ctx, db, repo := openSQLMetricPostgresTest(t)
	neighborhoodID, sourceID := createMetricFixtures(t, ctx, db)
	oldRun := insertMetricRun(t, ctx, db, sourceID, neighborhoodID, time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC), "full")
	newRun := insertMetricRun(t, ctx, db, sourceID, neighborhoodID, time.Date(2026, 7, 2, 9, 0, 0, 0, time.UTC), "full")
	upsertMinimalMetric(t, ctx, repo, neighborhoodID, oldRun.id, sourceID, 1)
	upsertMinimalMetric(t, ctx, repo, neighborhoodID, newRun.id, sourceID, 2)

	got, err := repo.LatestMetric(ctx, neighborhoodID)
	if err != nil {
		t.Fatalf("LatestMetric() error = %v", err)
	}
	if got.CollectionRunID != newRun.id || got.ListedHomes != 2 || got.Coverage != domainneighborhood.CoverageFull {
		t.Fatalf("LatestMetric() = %#v, want newest trigger run", got)
	}
}

func TestRepositoryMetricHistoryOrdersSnapshotsChronologically(t *testing.T) {
	ctx, db, repo := openSQLMetricPostgresTest(t)
	neighborhoodID, sourceID := createMetricFixtures(t, ctx, db)
	oldRun := insertMetricRun(t, ctx, db, sourceID, neighborhoodID, time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC), "full")
	newRun := insertMetricRun(t, ctx, db, sourceID, neighborhoodID, time.Date(2026, 7, 2, 9, 0, 0, 0, time.UTC), "full")
	upsertMinimalMetric(t, ctx, repo, neighborhoodID, newRun.id, sourceID, 2)
	upsertMinimalMetric(t, ctx, repo, neighborhoodID, oldRun.id, sourceID, 1)

	got, err := repo.ListMetricHistory(ctx, neighborhoodID, time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("ListMetricHistory() error = %v", err)
	}
	if len(got) != 2 || got[0].CollectionRunID != oldRun.id || got[1].CollectionRunID != newRun.id {
		t.Fatalf("history order = %#v", got)
	}
}

type metricRunFixture struct {
	id          string
	sourceID    string
	collectedAt time.Time
}

func openSQLMetricPostgresTest(t *testing.T) (context.Context, *pgxpool.Pool, *Repository) {
	t.Helper()
	databaseURL := os.Getenv("PROPULSE_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("PROPULSE_TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	if err := migraterunner.Run(ctx, databaseURL, "up"); err != nil {
		t.Fatalf("migrate up error = %v", err)
	}
	db, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("pgxpool.New() error = %v", err)
	}
	t.Cleanup(db.Close)
	return ctx, db, NewRepository(db)
}

func createMetricFixtures(t *testing.T, ctx context.Context, db *pgxpool.Pool) (string, string) {
	t.Helper()
	neighborhoodID := uuid.NewString()
	sourceID := insertMetricSource(t, ctx, db)
	if _, err := db.Exec(ctx, `INSERT INTO neighborhoods (id, name, area, target_layout) VALUES ($1, $2, $3, $4)`, neighborhoodID, "metric-neighborhood-"+neighborhoodID, "area", "三房"); err != nil {
		t.Fatalf("insert neighborhood error = %v", err)
	}
	return neighborhoodID, sourceID
}

func insertMetricSource(t *testing.T, ctx context.Context, db *pgxpool.Pool) string {
	t.Helper()
	sourceID := uuid.NewString()
	if _, err := db.Exec(ctx, `INSERT INTO data_sources (id, name, source_type, city) VALUES ($1, $2, $3, $4)`, sourceID, "source-"+sourceID, "manual", "杭州"); err != nil {
		t.Fatalf("insert data source error = %v", err)
	}
	return sourceID
}

func insertMetricRun(t *testing.T, ctx context.Context, db *pgxpool.Pool, sourceID, neighborhoodID string, collectedAt time.Time, coverage string) metricRunFixture {
	t.Helper()
	run := metricRunFixture{id: uuid.NewString(), sourceID: sourceID, collectedAt: collectedAt}
	checksum := strings.Repeat("a", 64)
	if _, err := db.Exec(ctx, `
INSERT INTO collection_runs (id, data_source_id, neighborhood_id, source_ref, collected_at, coverage, import_format, content_checksum, raw_payload, raw_content_type, validation_summary, status, metric_status)
VALUES ($1,$2,$3,$4,$5,$6,'json',$7,$8,'application/json','{}'::jsonb,'completed','pending')`,
		run.id, sourceID, neighborhoodID, "ref-"+run.id, collectedAt, coverage, checksum, []byte("{}")); err != nil {
		t.Fatalf("insert collection run error = %v", err)
	}
	return run
}

func insertMetricListing(t *testing.T, ctx context.Context, db *pgxpool.Pool, run metricRunFixture, neighborhoodID, sourceListingID, layout string, price float64, status string, capturedAt time.Time) {
	t.Helper()
	if _, err := db.Exec(ctx, `
INSERT INTO listing_observations (id, collection_run_id, neighborhood_id, source_listing_id, source_row, layout, area_sqm, listing_price, days_on_market, status, captured_at, attributes)
VALUES ($1,$2,$3,$4,1,$5,89.5,$6,20,$7,$8,'{}'::jsonb)`,
		uuid.NewString(), run.id, neighborhoodID, sourceListingID, layout, price, status, capturedAt); err != nil {
		t.Fatalf("insert listing error = %v", err)
	}
}

func insertMetricTransaction(t *testing.T, ctx context.Context, db *pgxpool.Pool, run metricRunFixture, neighborhoodID, sourceRecordID string, price float64, date time.Time) {
	t.Helper()
	if _, err := db.Exec(ctx, `
INSERT INTO transaction_observations (id, collection_run_id, neighborhood_id, source_record_id, source_row, layout, area_sqm, transaction_price, transaction_date, captured_at)
VALUES ($1,$2,$3,$4,1,'三房',89.5,$5,$6,$7)`,
		uuid.NewString(), run.id, neighborhoodID, sourceRecordID, price, date, run.collectedAt); err != nil {
		t.Fatalf("insert transaction error = %v", err)
	}
}

func upsertMinimalMetric(t *testing.T, ctx context.Context, repo *Repository, neighborhoodID, runID, sourceID string, listedHomes int) {
	t.Helper()
	now := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	if _, err := repo.UpsertNeighborhoodMetric(ctx, appmetric.MetricSnapshot{
		NeighborhoodID:           neighborhoodID,
		CollectionRunID:          runID,
		InventoryCollectionRunID: &runID,
		SourceIDs:                []string{sourceID},
		LatestObservedAt:         now,
		ListedHomes:              listedHomes,
		TransactionMomentum:      domainneighborhood.TransactionMomentumStable,
		Coverage:                 domainneighborhood.CoverageFull,
		Freshness:                domainneighborhood.FreshnessCurrent,
		QualityState:             domainneighborhood.MarketQualitySufficient,
	}); err != nil {
		t.Fatalf("UpsertNeighborhoodMetric() error = %v", err)
	}
}
