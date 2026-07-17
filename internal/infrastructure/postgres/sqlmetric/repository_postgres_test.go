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
	appneighborhood "github.com/sine-io/propulse/internal/application/neighborhood"
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

	got, err := repo.AggregateMarketObservations(ctx, appmetric.AggregateMarketParams{NeighborhoodID: neighborhoodID, TriggerRunID: newPartial.id})
	if err != nil {
		t.Fatalf("AggregateMarketObservations() error = %v", err)
	}
	if got.InventoryCollectionRunID == nil || *got.InventoryCollectionRunID != oldFull.id {
		t.Fatalf("InventoryCollectionRunID = %#v, want %s", got.InventoryCollectionRunID, oldFull.id)
	}
	if got.ListedHomes != 2 {
		t.Fatalf("ListedHomes = %d, want 2", got.ListedHomes)
	}
	if got.TargetLayoutSupplyByLayout["三房"] != 1 || got.TargetLayoutSupplyByLayout["两房"] != 1 {
		t.Fatalf("layout supply = %#v, want one listing per layout", got.TargetLayoutSupplyByLayout)
	}
}

func TestRepositoryAggregateCollectionRunDerivesPriceCutsFromPriorObservations(t *testing.T) {
	ctx, db, repo := openSQLMetricPostgresTest(t)
	neighborhoodID, sourceID := createMetricFixtures(t, ctx, db)
	oldRun := insertMetricRun(t, ctx, db, sourceID, neighborhoodID, time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC), "full")
	newRun := insertMetricRun(t, ctx, db, sourceID, neighborhoodID, time.Date(2026, 7, 2, 9, 0, 0, 0, time.UTC), "full")
	insertMetricListing(t, ctx, db, oldRun, neighborhoodID, "same-listing", "三房", 620, "active", oldRun.collectedAt)
	insertMetricListing(t, ctx, db, newRun, neighborhoodID, "same-listing", "三房", 590, "active", newRun.collectedAt)

	got, err := repo.AggregateMarketObservations(ctx, appmetric.AggregateMarketParams{NeighborhoodID: neighborhoodID, TriggerRunID: newRun.id})
	if err != nil {
		t.Fatalf("AggregateMarketObservations() error = %v", err)
	}
	if got.PriceCutHomes != 1 {
		t.Fatalf("PriceCutHomes = %d, want 1", got.PriceCutHomes)
	}
}

func TestRepositoryAggregateCollectionRunCountsSourceAdjustmentHistory(t *testing.T) {
	ctx, db, repo := openSQLMetricPostgresTest(t)
	neighborhoodID, sourceID := createMetricFixtures(t, ctx, db)
	run := insertMetricRun(t, ctx, db, sourceID, neighborhoodID, time.Date(2026, 7, 2, 9, 0, 0, 0, time.UTC), "full")
	insertMetricListing(t, ctx, db, run, neighborhoodID, "adjusted-listing", "三房", 590, "active", run.collectedAt)
	if _, err := db.Exec(ctx, `
		INSERT INTO listing_adjustments (
			id, collection_run_id, neighborhood_id, room_id, adjusted_at,
			price_before_wan, price_after_wan, amount_wan
		) VALUES ($1, $2, $3, $4, $5, 620, 590, -30)
	`, uuid.NewString(), run.id, neighborhoodID, "adjusted-listing", run.collectedAt); err != nil {
		t.Fatalf("insert listing adjustment error = %v", err)
	}

	got, err := repo.AggregateMarketObservations(ctx, appmetric.AggregateMarketParams{NeighborhoodID: neighborhoodID, TriggerRunID: run.id})
	if err != nil {
		t.Fatalf("AggregateMarketObservations() error = %v", err)
	}
	if got.PriceCutHomes != 1 {
		t.Fatalf("PriceCutHomes = %d, want 1 from source adjustment", got.PriceCutHomes)
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

	got, err := repo.AggregateMarketObservations(ctx, appmetric.AggregateMarketParams{NeighborhoodID: neighborhoodID, TriggerRunID: newRun.id})
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
	insertMetricTransaction(t, ctx, db, run, neighborhoodID, "tx-end", 500, triggerAt)
	insertMetricTransaction(t, ctx, db, run, neighborhoodID, "tx-29", 510, triggerAt.AddDate(0, 0, -29))
	insertMetricTransaction(t, ctx, db, run, neighborhoodID, "tx-30", 520, triggerAt.AddDate(0, 0, -30))
	insertMetricTransaction(t, ctx, db, run, neighborhoodID, "tx-90", 530, triggerAt.AddDate(0, 0, -90))
	insertMetricTransaction(t, ctx, db, run, neighborhoodID, "tx-91", 540, triggerAt.AddDate(0, 0, -91))
	insertMetricTransaction(t, ctx, db, run, neighborhoodID, "tx-future", 550, triggerAt.AddDate(0, 0, 1))

	got, err := repo.AggregateMarketObservations(ctx, appmetric.AggregateMarketParams{NeighborhoodID: neighborhoodID, TriggerRunID: run.id})
	if err != nil {
		t.Fatalf("AggregateMarketObservations() error = %v", err)
	}
	if got.TransactionSampleCount != 4 || got.LastThirtyDayTransactionCount != 2 || got.PrecedingSixtyDayTransactionCount != 2 {
		t.Fatalf("transaction windows = sample %d last30 %d prev60 %d, want 4/2/2", got.TransactionSampleCount, got.LastThirtyDayTransactionCount, got.PrecedingSixtyDayTransactionCount)
	}
}

func TestRepositoryAggregateCollectionRunDeduplicatesTransactionsWithinSource(t *testing.T) {
	ctx, db, repo := openSQLMetricPostgresTest(t)
	neighborhoodID, sourceA := createMetricFixtures(t, ctx, db)
	sourceB := insertMetricSource(t, ctx, db)
	triggerAt := time.Date(2026, 7, 10, 9, 0, 0, 0, time.UTC)
	oldRun := insertMetricRun(t, ctx, db, sourceA, neighborhoodID, triggerAt.Add(-48*time.Hour), "full")
	otherSourceRun := insertMetricRun(t, ctx, db, sourceB, neighborhoodID, triggerAt.Add(-24*time.Hour), "partial")
	triggerRun := insertMetricRun(t, ctx, db, sourceA, neighborhoodID, triggerAt, "full")
	insertMetricTransaction(t, ctx, db, oldRun, neighborhoodID, "same-record", 500, triggerAt.AddDate(0, 0, -20))
	insertMetricTransaction(t, ctx, db, triggerRun, neighborhoodID, "same-record", 510, triggerAt.AddDate(0, 0, -10))
	insertMetricTransaction(t, ctx, db, otherSourceRun, neighborhoodID, "same-record", 520, triggerAt.AddDate(0, 0, -5))

	got, err := repo.AggregateMarketObservations(ctx, appmetric.AggregateMarketParams{NeighborhoodID: neighborhoodID, TriggerRunID: triggerRun.id})
	if err != nil {
		t.Fatalf("AggregateMarketObservations() error = %v", err)
	}
	if got.TransactionSampleCount != 2 || got.LastThirtyDayTransactionCount != 2 || got.TransactionPriceMin == nil || *got.TransactionPriceMin != 510 {
		t.Fatalf("deduplicated transactions = %#v, want two latest source-scoped records", got)
	}
}

func TestRepositoryAggregateCollectionRunReturnsZeroTransactionEvidence(t *testing.T) {
	ctx, db, repo := openSQLMetricPostgresTest(t)
	neighborhoodID, sourceID := createMetricFixtures(t, ctx, db)
	run := insertMetricRun(t, ctx, db, sourceID, neighborhoodID, time.Date(2026, 7, 10, 9, 0, 0, 0, time.UTC), "full")

	got, err := repo.AggregateMarketObservations(ctx, appmetric.AggregateMarketParams{NeighborhoodID: neighborhoodID, TriggerRunID: run.id})
	if err != nil {
		t.Fatalf("AggregateMarketObservations() error = %v", err)
	}
	if got.TransactionSampleCount != 0 || got.LastThirtyDayTransactionCount != 0 || got.PrecedingSixtyDayTransactionCount != 0 {
		t.Fatalf("zero transaction evidence = %#v", got)
	}
}

func TestRepositoryUpsertNeighborhoodMetricPersistsProvenance(t *testing.T) {
	ctx, db, repo := openSQLMetricPostgresTest(t)
	neighborhoodID, sourceID := createMetricFixtures(t, ctx, db)
	run := insertMetricRun(t, ctx, db, sourceID, neighborhoodID, time.Date(2026, 7, 10, 9, 0, 0, 0, time.UTC), "full")
	avg := 22.0
	minPrice := 500.0
	maxPrice := 650.0
	inventoryAt := run.collectedAt
	evidence := domainneighborhood.NewTransactionMomentumEvidence(run.collectedAt, 1, 2)

	got, err := repo.UpsertNeighborhoodMetric(ctx, appmetric.MetricSnapshot{
		NeighborhoodID:             neighborhoodID,
		CollectionRunID:            run.id,
		AlgorithmVersion:           "market-metrics/test.1",
		InventoryCollectionRunID:   &run.id,
		SourceIDs:                  []string{sourceID},
		LatestObservedAt:           run.collectedAt,
		ListedHomes:                6,
		PriceCutHomes:              1,
		AvgDaysOnMarket:            &avg,
		ListingPriceMin:            &minPrice,
		ListingPriceMax:            &maxPrice,
		TransactionPriceMin:        &minPrice,
		TransactionPriceMax:        &maxPrice,
		TransactionMomentum:        domainneighborhood.TransactionMomentumStable,
		TransactionEvidence:        &evidence,
		TargetLayoutSupplyByLayout: map[string]int{"三房": 2},
		ListingSampleCount:         6,
		TransactionSampleCount:     3,
		Coverage:                   domainneighborhood.CoverageFull,
		Freshness:                  domainneighborhood.FreshnessCurrent,
		InventoryCollectedAt:       &inventoryAt,
		QualityState:               domainneighborhood.MarketQualitySufficient,
	})
	if err != nil {
		t.Fatalf("UpsertNeighborhoodMetric() error = %v", err)
	}
	if got.CollectionRunID != run.id || got.InventoryCollectionRunID == nil || *got.InventoryCollectionRunID != run.id || got.QualityState != domainneighborhood.MarketQualitySufficient {
		t.Fatalf("persisted provenance = %#v", got)
	}
	if got.AlgorithmVersion != "market-metrics/test.1" || got.TransactionEvidence == nil || got.TransactionEvidence.PrecedingSixtyDayMonthlyFrequency != 1 {
		t.Fatalf("persisted version/evidence = %#v", got)
	}
	if got.TargetLayoutSupplyByLayout["三房"] != 2 {
		t.Fatalf("persisted layout supply = %#v", got.TargetLayoutSupplyByLayout)
	}
}

func TestRepositoryMetricWriteIsIdempotentPerVersionAndPreservesHistory(t *testing.T) {
	ctx, db, repo := openSQLMetricPostgresTest(t)
	neighborhoodID, sourceID := createMetricFixtures(t, ctx, db)
	run := insertMetricRun(t, ctx, db, sourceID, neighborhoodID, time.Date(2026, 7, 10, 9, 0, 0, 0, time.UTC), "full")
	snapshot := minimalMetricSnapshot(neighborhoodID, run.id, sourceID, run.collectedAt, 5, "market-metrics/test.1")

	first, err := repo.UpsertNeighborhoodMetric(ctx, snapshot)
	if err != nil {
		t.Fatalf("first UpsertNeighborhoodMetric() error = %v", err)
	}
	snapshot.ListedHomes = 999
	retry, err := repo.UpsertNeighborhoodMetric(ctx, snapshot)
	if err != nil {
		t.Fatalf("retry UpsertNeighborhoodMetric() error = %v", err)
	}
	if retry.ID != first.ID || retry.ListedHomes != first.ListedHomes || !retry.CalculatedAt.Equal(first.CalculatedAt) {
		t.Fatalf("same-version retry changed history: first=%#v retry=%#v", first, retry)
	}

	snapshot.AlgorithmVersion = "market-metrics/test.2"
	upgraded, err := repo.UpsertNeighborhoodMetric(ctx, snapshot)
	if err != nil {
		t.Fatalf("upgraded UpsertNeighborhoodMetric() error = %v", err)
	}
	if upgraded.ID == first.ID || upgraded.AlgorithmVersion != "market-metrics/test.2" {
		t.Fatalf("upgraded metric = %#v, first=%#v", upgraded, first)
	}
}

func TestRepositoryLatestMetricMapsProvenance(t *testing.T) {
	ctx, db, repo := openSQLMetricPostgresTest(t)
	neighborhoodID, sourceID := createMetricFixtures(t, ctx, db)
	oldRun := insertMetricRun(t, ctx, db, sourceID, neighborhoodID, time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC), "full")
	newRun := insertMetricRun(t, ctx, db, sourceID, neighborhoodID, time.Date(2026, 7, 2, 9, 0, 0, 0, time.UTC), "full")
	upsertMinimalMetric(t, ctx, repo, neighborhoodID, oldRun, sourceID, 1)
	upsertMinimalMetric(t, ctx, repo, neighborhoodID, newRun, sourceID, 2)
	legacy := minimalMetricSnapshot(neighborhoodID, newRun.id, sourceID, newRun.collectedAt, 999, "legacy_unversioned")
	if _, err := repo.UpsertNeighborhoodMetric(ctx, legacy); err != nil {
		t.Fatalf("insert legacy metric: %v", err)
	}

	got, err := repo.LatestMetric(ctx, neighborhoodID)
	if err != nil {
		t.Fatalf("LatestMetric() error = %v", err)
	}
	if got.CollectionRunID != newRun.id || got.ListedHomes != 2 || got.Coverage != domainneighborhood.CoverageFull {
		t.Fatalf("LatestMetric() = %#v, want newest trigger run", got)
	}
	if got.AlgorithmVersion != "market-metrics/test.1" || got.TransactionEvidence == nil || got.TransactionEvidence.SampleCount != 3 {
		t.Fatalf("LatestMetric() lost versioned transaction evidence: %#v", got)
	}
	if !got.CollectedAt.Equal(newRun.collectedAt) {
		t.Fatalf("CollectedAt = %s, want %s", got.CollectedAt, newRun.collectedAt)
	}
}

func TestRepositoryMetricHistoryOrdersSnapshotsChronologically(t *testing.T) {
	ctx, db, repo := openSQLMetricPostgresTest(t)
	neighborhoodID, sourceID := createMetricFixtures(t, ctx, db)
	oldRun := insertMetricRun(t, ctx, db, sourceID, neighborhoodID, time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC), "full")
	newRun := insertMetricRun(t, ctx, db, sourceID, neighborhoodID, time.Date(2026, 7, 2, 9, 0, 0, 0, time.UTC), "full")
	upsertMinimalMetric(t, ctx, repo, neighborhoodID, newRun, sourceID, 2)
	upsertMinimalMetric(t, ctx, repo, neighborhoodID, oldRun, sourceID, 1)
	legacy := minimalMetricSnapshot(neighborhoodID, newRun.id, sourceID, newRun.collectedAt, 999, "legacy_unversioned")
	if _, err := repo.UpsertNeighborhoodMetric(ctx, legacy); err != nil {
		t.Fatalf("insert legacy metric: %v", err)
	}

	got, err := repo.ListMetricHistory(ctx, appneighborhood.MetricHistoryRepositoryQuery{
		NeighborhoodID: neighborhoodID,
		From:           time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
		To:             time.Date(2026, 7, 3, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("ListMetricHistory() error = %v", err)
	}
	if len(got) != 2 || got[0].Metric.CollectionRunID != oldRun.id || got[1].Metric.CollectionRunID != newRun.id {
		t.Fatalf("history order = %#v", got)
	}
	for _, record := range got {
		if record.Metric.AlgorithmVersion != "market-metrics/test.1" || record.Metric.TransactionEvidence == nil || record.Batch.DataSourceID != sourceID || record.Batch.SourceRef == "" {
			t.Fatalf("history metric lost versioned evidence/source: %#v", record)
		}
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
	return ctx, db, NewRepository(db, "market-metrics/test.1")
}

func createMetricFixtures(t *testing.T, ctx context.Context, db *pgxpool.Pool) (string, string) {
	t.Helper()
	neighborhoodID := uuid.NewString()
	sourceID := insertMetricSource(t, ctx, db)
	if _, err := db.Exec(ctx, `INSERT INTO neighborhoods (id, name, city, area) VALUES ($1, $2, $3, $4)`, neighborhoodID, "metric-neighborhood-"+neighborhoodID, "测试城市", "area"); err != nil {
		t.Fatalf("insert neighborhood error = %v", err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO neighborhood_layouts (neighborhood_id, layout) VALUES ($1, '三房'), ($1, '两房')`, neighborhoodID); err != nil {
		t.Fatalf("insert neighborhood layouts error = %v", err)
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

func upsertMinimalMetric(t *testing.T, ctx context.Context, repo *Repository, neighborhoodID string, run metricRunFixture, sourceID string, listedHomes int) {
	t.Helper()
	if _, err := repo.UpsertNeighborhoodMetric(ctx, minimalMetricSnapshot(neighborhoodID, run.id, sourceID, run.collectedAt, listedHomes, "market-metrics/test.1")); err != nil {
		t.Fatalf("UpsertNeighborhoodMetric() error = %v", err)
	}
}

func minimalMetricSnapshot(neighborhoodID, runID, sourceID string, collectedAt time.Time, listedHomes int, algorithmVersion string) appmetric.MetricSnapshot {
	evidence := domainneighborhood.NewTransactionMomentumEvidence(collectedAt, 1, 2)
	return appmetric.MetricSnapshot{
		NeighborhoodID:           neighborhoodID,
		CollectionRunID:          runID,
		AlgorithmVersion:         algorithmVersion,
		InventoryCollectionRunID: &runID,
		SourceIDs:                []string{sourceID},
		LatestObservedAt:         collectedAt,
		ListedHomes:              listedHomes,
		TransactionMomentum:      domainneighborhood.TransactionMomentumStable,
		TransactionEvidence:      &evidence,
		ListingSampleCount:       listedHomes,
		TransactionSampleCount:   evidence.SampleCount,
		Coverage:                 domainneighborhood.CoverageFull,
		Freshness:                domainneighborhood.FreshnessCurrent,
		QualityState:             domainneighborhood.MarketQualitySufficient,
	}
}
