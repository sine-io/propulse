package metric

import (
	"context"
	"errors"
	"testing"
	"time"

	domainneighborhood "github.com/sine-io/propulse/internal/domain/neighborhood"
)

func TestCalculateNeighborhoodAggregatesSnapshotsAndWritesMetric(t *testing.T) {
	repo := &memoryRepository{
		neighborhood: Neighborhood{
			ID:           "neighborhood_1",
			TargetLayout: "三房",
		},
		aggregate: ListingSnapshotAggregate{
			ListedHomes:         42,
			PriceCutHomes:       11,
			AvgDaysOnMarket:     78,
			ListingPriceMin:     520,
			ListingPriceMax:     620,
			TransactionPriceMin: 495,
			TransactionPriceMax: 545,
			TargetLayoutSupply:  12,
		},
		insertedMetric: MetricSnapshot{
			ID:           "metric_1",
			CalculatedAt: time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC),
		},
	}
	service := NewService(repo)

	err := service.CalculateNeighborhood(context.Background(), "neighborhood_1")
	if err != nil {
		t.Fatalf("CalculateNeighborhood() error = %v", err)
	}

	if repo.aggregateNeighborhoodID != "neighborhood_1" {
		t.Fatalf("aggregateNeighborhoodID = %q, want neighborhood_1", repo.aggregateNeighborhoodID)
	}
	if repo.aggregateTargetLayout != "三房" {
		t.Fatalf("aggregateTargetLayout = %q, want 三房", repo.aggregateTargetLayout)
	}
	if repo.insertCount != 1 {
		t.Fatalf("insertCount = %d, want 1", repo.insertCount)
	}
	got := repo.lastInserted
	if got.NeighborhoodID != "neighborhood_1" {
		t.Fatalf("NeighborhoodID = %q, want neighborhood_1", got.NeighborhoodID)
	}
	if got.ListedHomes != 42 || got.PriceCutHomes != 11 || got.TargetLayoutSupply != 12 {
		t.Fatalf("metric counts = listed %d, cuts %d, target %d", got.ListedHomes, got.PriceCutHomes, got.TargetLayoutSupply)
	}
	if got.AvgDaysOnMarket == nil || *got.AvgDaysOnMarket != 78 || got.ListingPriceMin == nil || *got.ListingPriceMin != 520 || got.TransactionPriceMax == nil || *got.TransactionPriceMax != 545 {
		t.Fatalf("metric values = %#v", got)
	}
	if got.TransactionMomentum != domainneighborhood.TransactionMomentumWeak {
		t.Fatalf("TransactionMomentum = %q, want weak", got.TransactionMomentum)
	}
}

func TestCalculateCollectionRunUsesLatestRunForProvenance(t *testing.T) {
	now := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	inventoryAt := now.Add(-time.Hour)
	repo := newMetricMemoryRepository()
	repo.neighborhood = Neighborhood{ID: "neighborhood_1", TargetLayout: "三房"}
	repo.latestRun = CompletedCollectionRun{ID: "run_latest", DataSourceID: "source_a", NeighborhoodID: "neighborhood_1", CollectedAt: now, Coverage: domainneighborhood.CoveragePartial}
	repo.marketAggregate = MarketAggregate{CollectionRunID: "run_latest", InventoryCollectionRunID: strPtr("run_full"), SourceIDs: []string{"source_a"}, LatestObservedAt: now, InventoryCollectedAt: &inventoryAt, Coverage: domainneighborhood.CoveragePartial, ListedHomes: 8, PriceCutHomes: 1, AvgDaysOnMarket: floatPtr(30), ListingPriceMin: floatPtr(500), ListingPriceMax: floatPtr(600), TransactionPriceMin: floatPtr(480), TransactionPriceMax: floatPtr(550), TargetLayoutSupply: 3, ListingSampleCount: 8, TransactionSampleCount: 3, LastThirtyDayTransactionCount: 2, PrecedingSixtyDayTransactionCount: 2}

	err := NewServiceWithClock(repo, func() time.Time { return now }).CalculateCollectionRun(context.Background(), CalculateCollectionRunCommand{NeighborhoodID: "neighborhood_1"})
	if err != nil {
		t.Fatalf("CalculateCollectionRun() error = %v", err)
	}

	if repo.aggregateParams.TriggerRunID != "run_latest" {
		t.Fatalf("TriggerRunID = %q, want run_latest", repo.aggregateParams.TriggerRunID)
	}
	got := repo.lastUpserted
	if got.CollectionRunID != "run_latest" {
		t.Fatalf("CollectionRunID = %q, want run_latest", got.CollectionRunID)
	}
	if got.InventoryCollectionRunID == nil || *got.InventoryCollectionRunID != "run_full" {
		t.Fatalf("InventoryCollectionRunID = %#v, want run_full", got.InventoryCollectionRunID)
	}
	if got.QualityState != domainneighborhood.MarketQualityLowConfidence {
		t.Fatalf("QualityState = %q, want low_confidence", got.QualityState)
	}
}

func TestCalculateCollectionRunUsesLatestFullRunForInventory(t *testing.T) {
	now := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	fullAt := now.Add(-24 * time.Hour)
	repo := newMetricMemoryRepository()
	repo.neighborhood = Neighborhood{ID: "neighborhood_1", TargetLayout: "三房"}
	repo.completedRuns["partial_new"] = CompletedCollectionRun{ID: "partial_new", DataSourceID: "source_a", NeighborhoodID: "neighborhood_1", CollectedAt: now, Coverage: domainneighborhood.CoveragePartial}
	repo.marketAggregate = MarketAggregate{CollectionRunID: "partial_new", InventoryCollectionRunID: strPtr("full_old"), SourceIDs: []string{"source_a"}, LatestObservedAt: now, InventoryCollectedAt: &fullAt, Coverage: domainneighborhood.CoveragePartial, ListedHomes: 11, PriceCutHomes: 2, AvgDaysOnMarket: floatPtr(41), ListingPriceMin: floatPtr(510), ListingPriceMax: floatPtr(650), TransactionPriceMin: floatPtr(500), TransactionPriceMax: floatPtr(620), TargetLayoutSupply: 6, ListingSampleCount: 11, TransactionSampleCount: 4, LastThirtyDayTransactionCount: 2, PrecedingSixtyDayTransactionCount: 3}

	if err := NewServiceWithClock(repo, func() time.Time { return now }).CalculateCollectionRun(context.Background(), CalculateCollectionRunCommand{NeighborhoodID: "neighborhood_1", CollectionRunID: "partial_new"}); err != nil {
		t.Fatalf("CalculateCollectionRun() error = %v", err)
	}

	got := repo.lastUpserted
	if got.CollectionRunID != "partial_new" {
		t.Fatalf("CollectionRunID = %q, want partial_new", got.CollectionRunID)
	}
	if got.InventoryCollectionRunID == nil || *got.InventoryCollectionRunID != "full_old" {
		t.Fatalf("InventoryCollectionRunID = %#v, want full_old", got.InventoryCollectionRunID)
	}
}

func TestCalculateCollectionRunDerivesPriceCutsFromPriorListingObservation(t *testing.T) {
	repo := sufficientMetricRepo()
	repo.marketAggregate.PriceCutHomes = 2
	if err := NewServiceWithClock(repo, fixedMetricClock).CalculateCollectionRun(context.Background(), CalculateCollectionRunCommand{NeighborhoodID: "neighborhood_1", CollectionRunID: "run_1"}); err != nil {
		t.Fatalf("CalculateCollectionRun() error = %v", err)
	}
	if repo.lastUpserted.PriceCutHomes != 2 {
		t.Fatalf("PriceCutHomes = %d, want 2", repo.lastUpserted.PriceCutHomes)
	}
}

func TestCalculateCollectionRunUsesOnlyTransactionsWithinNinetyDays(t *testing.T) {
	repo := sufficientMetricRepo()
	repo.marketAggregate.TransactionSampleCount = 3
	repo.marketAggregate.LastThirtyDayTransactionCount = 1
	repo.marketAggregate.PrecedingSixtyDayTransactionCount = 4
	if err := NewServiceWithClock(repo, fixedMetricClock).CalculateCollectionRun(context.Background(), CalculateCollectionRunCommand{NeighborhoodID: "neighborhood_1", CollectionRunID: "run_1"}); err != nil {
		t.Fatalf("CalculateCollectionRun() error = %v", err)
	}
	if repo.lastUpserted.TransactionSampleCount != 3 {
		t.Fatalf("TransactionSampleCount = %d, want 3", repo.lastUpserted.TransactionSampleCount)
	}
	if repo.lastUpserted.TransactionMomentum != domainneighborhood.TransactionMomentumWeak {
		t.Fatalf("TransactionMomentum = %q, want weak", repo.lastUpserted.TransactionMomentum)
	}
}

func TestCalculateCollectionRunDoesNotCompareListingIDsAcrossSources(t *testing.T) {
	repo := sufficientMetricRepo()
	repo.marketAggregate.SourceIDs = []string{"source_a", "source_b"}
	repo.marketAggregate.PriceCutHomes = 0
	if err := NewServiceWithClock(repo, fixedMetricClock).CalculateCollectionRun(context.Background(), CalculateCollectionRunCommand{NeighborhoodID: "neighborhood_1", CollectionRunID: "run_1"}); err != nil {
		t.Fatalf("CalculateCollectionRun() error = %v", err)
	}
	if repo.lastUpserted.PriceCutHomes != 0 {
		t.Fatalf("PriceCutHomes = %d, want 0", repo.lastUpserted.PriceCutHomes)
	}
}

func TestCalculateCollectionRunUsesTriggerRunAsOfTime(t *testing.T) {
	repo := sufficientMetricRepo()
	repo.completedRuns["older_run"] = CompletedCollectionRun{ID: "older_run", DataSourceID: "source_a", NeighborhoodID: "neighborhood_1", CollectedAt: time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC), Coverage: domainneighborhood.CoverageFull}
	if err := NewServiceWithClock(repo, fixedMetricClock).CalculateCollectionRun(context.Background(), CalculateCollectionRunCommand{NeighborhoodID: "neighborhood_1", CollectionRunID: "older_run"}); err != nil {
		t.Fatalf("CalculateCollectionRun() error = %v", err)
	}
	if repo.aggregateParams.TriggerRunID != "older_run" {
		t.Fatalf("TriggerRunID = %q, want older_run", repo.aggregateParams.TriggerRunID)
	}
}

func TestCalculateCollectionRunRejectsRunFromAnotherNeighborhood(t *testing.T) {
	repo := sufficientMetricRepo()
	repo.completedRuns["run_other"] = CompletedCollectionRun{ID: "run_other", DataSourceID: "source_a", NeighborhoodID: "neighborhood_2", CollectedAt: fixedMetricClock(), Coverage: domainneighborhood.CoverageFull}
	err := NewServiceWithClock(repo, fixedMetricClock).CalculateCollectionRun(context.Background(), CalculateCollectionRunCommand{NeighborhoodID: "neighborhood_1", CollectionRunID: "run_other"})
	if !errors.Is(err, ErrCollectionRunNeighborhoodMismatch) {
		t.Fatalf("error = %v, want ErrCollectionRunNeighborhoodMismatch", err)
	}
	if repo.upsertCount != 0 {
		t.Fatalf("upsertCount = %d, want 0", repo.upsertCount)
	}
}

func TestCalculateCollectionRunStoresLowConfidenceForPartialOrInsufficientData(t *testing.T) {
	repo := sufficientMetricRepo()
	repo.marketAggregate.Coverage = domainneighborhood.CoveragePartial
	repo.marketAggregate.TransactionSampleCount = 1
	repo.marketAggregate.LastThirtyDayTransactionCount = 1
	repo.marketAggregate.PrecedingSixtyDayTransactionCount = 0
	if err := NewServiceWithClock(repo, fixedMetricClock).CalculateCollectionRun(context.Background(), CalculateCollectionRunCommand{NeighborhoodID: "neighborhood_1", CollectionRunID: "run_1"}); err != nil {
		t.Fatalf("CalculateCollectionRun() error = %v", err)
	}
	if repo.lastUpserted.QualityState != domainneighborhood.MarketQualityLowConfidence {
		t.Fatalf("QualityState = %q, want low_confidence", repo.lastUpserted.QualityState)
	}
	if repo.lastUpserted.TransactionMomentum != domainneighborhood.TransactionMomentumUnknown {
		t.Fatalf("TransactionMomentum = %q, want unknown", repo.lastUpserted.TransactionMomentum)
	}
}

func TestCalculateCollectionRunUpsertsOneMetricPerCollectionRun(t *testing.T) {
	repo := sufficientMetricRepo()
	service := NewServiceWithClock(repo, fixedMetricClock)
	if err := service.CalculateCollectionRun(context.Background(), CalculateCollectionRunCommand{NeighborhoodID: "neighborhood_1", CollectionRunID: "run_1"}); err != nil {
		t.Fatalf("first CalculateCollectionRun() error = %v", err)
	}
	if err := service.CalculateCollectionRun(context.Background(), CalculateCollectionRunCommand{NeighborhoodID: "neighborhood_1", CollectionRunID: "run_1"}); err != nil {
		t.Fatalf("second CalculateCollectionRun() error = %v", err)
	}
	if repo.upsertCount != 2 {
		t.Fatalf("upsertCount = %d, want 2 retries through upsert", repo.upsertCount)
	}
	if repo.lastUpserted.CollectionRunID != "run_1" {
		t.Fatalf("CollectionRunID = %q, want run_1", repo.lastUpserted.CollectionRunID)
	}
}

func TestCalculateCollectionRunMarksResolvedRunMetricCompleted(t *testing.T) {
	repo := sufficientMetricRepo()
	repo.latestRun = repo.completedRuns["run_1"]
	if err := NewServiceWithClock(repo, fixedMetricClock).CalculateCollectionRun(context.Background(), CalculateCollectionRunCommand{NeighborhoodID: "neighborhood_1"}); err != nil {
		t.Fatalf("CalculateCollectionRun() error = %v", err)
	}
	if repo.markedRunID != "run_1" {
		t.Fatalf("markedRunID = %q, want run_1", repo.markedRunID)
	}
}

func TestCalculateNeighborhoodMapsStrongMomentum(t *testing.T) {
	repo := &memoryRepository{
		neighborhood: Neighborhood{ID: "neighborhood_1", TargetLayout: "四房"},
		aggregate: ListingSnapshotAggregate{
			ListedHomes:        14,
			PriceCutHomes:      1,
			AvgDaysOnMarket:    35,
			ListingPriceMin:    700,
			ListingPriceMax:    760,
			TargetLayoutSupply: 3,
		},
	}
	service := NewService(repo)

	if err := service.CalculateNeighborhood(context.Background(), "neighborhood_1"); err != nil {
		t.Fatalf("CalculateNeighborhood() error = %v", err)
	}

	if repo.lastInserted.TransactionMomentum != domainneighborhood.TransactionMomentumStrong {
		t.Fatalf("TransactionMomentum = %q, want strong", repo.lastInserted.TransactionMomentum)
	}
}

type memoryRepository struct {
	neighborhood    Neighborhood
	aggregate       ListingSnapshotAggregate
	completedRuns   map[string]CompletedCollectionRun
	latestRun       CompletedCollectionRun
	marketAggregate MarketAggregate

	aggregateNeighborhoodID string
	aggregateTargetLayout   string
	aggregateParams         AggregateMarketParams
	lastInserted            MetricSnapshot
	insertedMetric          MetricSnapshot
	insertCount             int
	lastUpserted            MetricSnapshot
	upsertCount             int
	markedRunID             string
}

func newMetricMemoryRepository() *memoryRepository {
	return &memoryRepository{completedRuns: map[string]CompletedCollectionRun{}}
}

func sufficientMetricRepo() *memoryRepository {
	now := fixedMetricClock()
	inventoryAt := now.Add(-time.Hour)
	repo := newMetricMemoryRepository()
	repo.neighborhood = Neighborhood{ID: "neighborhood_1", TargetLayout: "三房"}
	repo.completedRuns["run_1"] = CompletedCollectionRun{ID: "run_1", DataSourceID: "source_a", NeighborhoodID: "neighborhood_1", CollectedAt: now, Coverage: domainneighborhood.CoverageFull}
	repo.marketAggregate = MarketAggregate{
		CollectionRunID:                   "run_1",
		InventoryCollectionRunID:          strPtr("run_1"),
		SourceIDs:                         []string{"source_a"},
		LatestObservedAt:                  now,
		InventoryCollectedAt:              &inventoryAt,
		Coverage:                          domainneighborhood.CoverageFull,
		ListedHomes:                       8,
		PriceCutHomes:                     1,
		AvgDaysOnMarket:                   floatPtr(30),
		ListingPriceMin:                   floatPtr(500),
		ListingPriceMax:                   floatPtr(600),
		TransactionPriceMin:               floatPtr(480),
		TransactionPriceMax:               floatPtr(550),
		TargetLayoutSupply:                3,
		ListingSampleCount:                8,
		TransactionSampleCount:            3,
		LastThirtyDayTransactionCount:     2,
		PrecedingSixtyDayTransactionCount: 3,
	}
	return repo
}

func fixedMetricClock() time.Time {
	return time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
}

func (m *memoryRepository) GetNeighborhood(_ context.Context, id string) (Neighborhood, error) {
	if m.neighborhood.ID != id {
		return Neighborhood{}, ErrNeighborhoodNotFound
	}
	return m.neighborhood, nil
}

func (m *memoryRepository) AggregateListingSnapshots(_ context.Context, neighborhoodID string, targetLayout string) (ListingSnapshotAggregate, error) {
	m.aggregateNeighborhoodID = neighborhoodID
	m.aggregateTargetLayout = targetLayout
	return m.aggregate, nil
}

func (m *memoryRepository) GetCompletedCollectionRun(_ context.Context, id string) (CompletedCollectionRun, error) {
	run, ok := m.completedRuns[id]
	if !ok {
		return CompletedCollectionRun{}, ErrCollectionRunNotFound
	}
	return run, nil
}

func (m *memoryRepository) LatestCompletedCollectionRun(_ context.Context, neighborhoodID string) (CompletedCollectionRun, error) {
	if m.latestRun.ID != "" && m.latestRun.NeighborhoodID == neighborhoodID {
		return m.latestRun, nil
	}
	return CompletedCollectionRun{}, ErrCollectionRunNotFound
}

func (m *memoryRepository) AggregateMarketObservations(_ context.Context, params AggregateMarketParams) (MarketAggregate, error) {
	m.aggregateParams = params
	aggregate := m.marketAggregate
	aggregate.CollectionRunID = params.TriggerRunID
	return aggregate, nil
}

func (m *memoryRepository) UpsertNeighborhoodMetric(_ context.Context, snapshot MetricSnapshot) (MetricSnapshot, error) {
	m.upsertCount++
	m.lastUpserted = snapshot
	return snapshot, nil
}

func (m *memoryRepository) MarkCollectionRunMetricCompleted(_ context.Context, collectionRunID string) error {
	m.markedRunID = collectionRunID
	return nil
}

func (m *memoryRepository) InsertNeighborhoodMetric(_ context.Context, snapshot MetricSnapshot) (MetricSnapshot, error) {
	m.insertCount++
	m.lastInserted = snapshot
	if m.insertedMetric.ID != "" {
		snapshot.ID = m.insertedMetric.ID
		snapshot.CalculatedAt = m.insertedMetric.CalculatedAt
	}
	return snapshot, nil
}

func strPtr(value string) *string { return &value }

func floatPtr(value float64) *float64 { return &value }
