package neighborhood

import (
	"context"
	"errors"
	"testing"
	"time"

	domainneighborhood "github.com/sine-io/propulse/internal/domain/neighborhood"
)

const historyTestAlgorithmVersion = "market-metrics/test.1"

func TestMetricHistorySelectsLatestFullWeeklyAndMonthlyBaselines(t *testing.T) {
	currentAt := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	repo := historyRepository(
		historyRecord("month-start", currentAt.Add(-45*24*time.Hour), domainneighborhood.CoverageFull, 4, 1, 1),
		historyRecord("month-end", currentAt.Add(-30*24*time.Hour), domainneighborhood.CoverageFull, 5, 2, 2),
		historyRecord("week-start", currentAt.Add(-14*24*time.Hour), domainneighborhood.CoverageFull, 8, 2, 2),
		historyRecord("week-end", currentAt.Add(-7*24*time.Hour), domainneighborhood.CoverageFull, 10, 3, 3),
		historyRecord("too-recent", currentAt.Add(-7*24*time.Hour+time.Second), domainneighborhood.CoverageFull, 999, 999, 999),
		historyRecord("current", currentAt, domainneighborhood.CoverageFull, 15, 5, 6),
	)
	service := NewServiceWithMetricConfig(repo, historyTestAlgorithmVersion, func() time.Time { return currentAt })

	result, err := service.MetricHistory(context.Background(), MetricHistoryQuery{NeighborhoodID: "neighborhood_1", TargetLayout: "三房", From: currentAt, To: currentAt})
	if err != nil {
		t.Fatalf("MetricHistory() error = %v", err)
	}
	if result.Status != MetricHistoryReady || len(result.Items) != 1 {
		t.Fatalf("result = %#v", result)
	}
	point := result.Items[0]
	if point.WeeklyComparison.Status != domainneighborhood.MetricComparisonAvailable || point.WeeklyComparison.BaselineBatch == nil || point.WeeklyComparison.BaselineBatch.CollectionRunID != "week-end" {
		t.Fatalf("weekly comparison = %#v", point.WeeklyComparison)
	}
	if point.MonthlyComparison.Status != domainneighborhood.MetricComparisonAvailable || point.MonthlyComparison.BaselineBatch == nil || point.MonthlyComparison.BaselineBatch.CollectionRunID != "month-end" {
		t.Fatalf("monthly comparison = %#v", point.MonthlyComparison)
	}
	if point.WeeklyComparison.ListedHomes == nil || point.WeeklyComparison.ListedHomes.AbsoluteChange != 5 || point.WeeklyComparison.RecentThirtyDayTransactions == nil || point.WeeklyComparison.RecentThirtyDayTransactions.AbsoluteChange != 3 {
		t.Fatalf("weekly changes = %#v", point.WeeklyComparison)
	}
}

func TestMetricHistoryReturnsUnavailableForPartialCurrentRun(t *testing.T) {
	currentAt := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	repo := historyRepository(
		historyRecord("baseline", currentAt.Add(-7*24*time.Hour), domainneighborhood.CoverageFull, 10, 2, 2),
		historyRecord("current", currentAt, domainneighborhood.CoveragePartial, 15, 3, 3),
	)
	result, err := NewServiceWithMetricConfig(repo, historyTestAlgorithmVersion, func() time.Time { return currentAt }).
		MetricHistory(context.Background(), MetricHistoryQuery{NeighborhoodID: "neighborhood_1", TargetLayout: "三房", From: currentAt, To: currentAt})
	if err != nil {
		t.Fatalf("MetricHistory() error = %v", err)
	}
	comparison := result.Items[0].WeeklyComparison
	if comparison.Status != domainneighborhood.MetricComparisonUnavailable || comparison.Reason != domainneighborhood.ComparisonReasonCurrentPartialCoverage || comparison.ListedHomes != nil {
		t.Fatalf("comparison = %#v", comparison)
	}
}

func TestMetricHistoryReturnsUnavailableWithoutFullBaseline(t *testing.T) {
	currentAt := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	repo := historyRepository(
		historyRecord("partial-baseline", currentAt.Add(-7*24*time.Hour), domainneighborhood.CoveragePartial, 10, 2, 2),
		historyRecord("current", currentAt, domainneighborhood.CoverageFull, 15, 3, 3),
	)
	result, err := NewServiceWithMetricConfig(repo, historyTestAlgorithmVersion, func() time.Time { return currentAt }).
		MetricHistory(context.Background(), MetricHistoryQuery{NeighborhoodID: "neighborhood_1", TargetLayout: "三房", From: currentAt, To: currentAt})
	if err != nil {
		t.Fatalf("MetricHistory() error = %v", err)
	}
	comparison := result.Items[0].WeeklyComparison
	if comparison.Status != domainneighborhood.MetricComparisonUnavailable || comparison.Reason != domainneighborhood.ComparisonReasonFullBaselineNotFound || comparison.BaselineBatch != nil {
		t.Fatalf("comparison = %#v", comparison)
	}
}

func TestMetricHistoryMarksPercentageAsZeroBaseline(t *testing.T) {
	currentAt := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	repo := historyRepository(
		historyRecord("baseline", currentAt.Add(-7*24*time.Hour), domainneighborhood.CoverageFull, 0, 0, 0),
		historyRecord("current", currentAt, domainneighborhood.CoverageFull, 3, 2, 1),
	)
	result, err := NewServiceWithMetricConfig(repo, historyTestAlgorithmVersion, func() time.Time { return currentAt }).
		MetricHistory(context.Background(), MetricHistoryQuery{NeighborhoodID: "neighborhood_1", TargetLayout: "三房", From: currentAt, To: currentAt})
	if err != nil {
		t.Fatalf("MetricHistory() error = %v", err)
	}
	change := result.Items[0].WeeklyComparison.ListedHomes
	if change == nil || change.AbsoluteChange != 3 || change.PercentageChange != nil || change.PercentageStatus != domainneighborhood.PercentageChangeZeroBaseline {
		t.Fatalf("change = %#v", change)
	}
}

func TestMetricHistoryReturnsUnavailableWhenTransactionEvidenceIsMissing(t *testing.T) {
	currentAt := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	baseline := historyRecord("baseline", currentAt.Add(-7*24*time.Hour), domainneighborhood.CoverageFull, 10, 2, 2)
	baseline.Metric.TransactionEvidence = nil
	repo := historyRepository(
		baseline,
		historyRecord("current", currentAt, domainneighborhood.CoverageFull, 15, 3, 3),
	)

	result, err := NewServiceWithMetricConfig(repo, historyTestAlgorithmVersion, func() time.Time { return currentAt }).
		MetricHistory(context.Background(), MetricHistoryQuery{NeighborhoodID: "neighborhood_1", TargetLayout: "三房", From: currentAt, To: currentAt})
	if err != nil {
		t.Fatalf("MetricHistory() error = %v", err)
	}
	comparison := result.Items[0].WeeklyComparison
	if comparison.Status != domainneighborhood.MetricComparisonUnavailable || comparison.Reason != domainneighborhood.ComparisonReasonTransactionEvidenceMissing || comparison.ListedHomes != nil {
		t.Fatalf("comparison = %#v", comparison)
	}
}

func TestMetricHistoryReturnsExplicitEmptyDefaultWindow(t *testing.T) {
	now := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	repo := historyRepository()
	result, err := NewServiceWithMetricConfig(repo, historyTestAlgorithmVersion, func() time.Time { return now }).
		MetricHistory(context.Background(), MetricHistoryQuery{NeighborhoodID: "neighborhood_1", TargetLayout: "三房"})
	if err != nil {
		t.Fatalf("MetricHistory() error = %v", err)
	}
	if result.Status != MetricHistoryEmpty || len(result.Items) != 0 || !result.From.Equal(now.Add(-8*7*24*time.Hour)) || !result.To.Equal(now) || result.AlgorithmVersion != historyTestAlgorithmVersion {
		t.Fatalf("result = %#v", result)
	}
}

func TestMetricHistoryRejectsInvalidAndOversizedWindows(t *testing.T) {
	now := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	service := NewServiceWithMetricConfig(historyRepository(), historyTestAlgorithmVersion, func() time.Time { return now })
	for name, query := range map[string]MetricHistoryQuery{
		"reversed": {NeighborhoodID: "neighborhood_1", TargetLayout: "三房", From: now, To: now.Add(-time.Hour)},
		"too long": {NeighborhoodID: "neighborhood_1", TargetLayout: "三房", From: now.Add(-52*7*24*time.Hour - time.Second), To: now},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := service.MetricHistory(context.Background(), query)
			if !errors.Is(err, ErrInvalidMetricHistoryWindow) {
				t.Fatalf("error = %v, want ErrInvalidMetricHistoryWindow", err)
			}
		})
	}
}

func historyRepository(records ...MetricHistoryRecord) *memoryRepository {
	repo := newMemoryRepository()
	repo.neighborhoods["neighborhood_1"] = Neighborhood{ID: "neighborhood_1", AvailableLayouts: []string{"三房"}}
	repo.history = records
	return repo
}

func historyRecord(id string, collectedAt time.Time, coverage domainneighborhood.Coverage, listedHomes, priceCutHomes, recentTransactions int) MetricHistoryRecord {
	evidence := domainneighborhood.NewTransactionMomentumEvidence(collectedAt, recentTransactions, 3)
	inventoryRunID := id
	metric := MetricSnapshot{
		ID:                         "metric-" + id,
		NeighborhoodID:             "neighborhood_1",
		CollectionRunID:            id,
		AlgorithmVersion:           historyTestAlgorithmVersion,
		InventoryCollectionRunID:   &inventoryRunID,
		SourceIDs:                  []string{"source_1"},
		LatestObservedAt:           collectedAt,
		CollectedAt:                collectedAt,
		ListedHomes:                listedHomes,
		PriceCutHomes:              priceCutHomes,
		TransactionMomentum:        domainneighborhood.CalculateTransactionMomentum(evidence),
		TransactionEvidence:        &evidence,
		TargetLayoutSupplyByLayout: map[string]int{"三房": listedHomes / 2},
		ListingSampleCount:         listedHomes,
		TransactionSampleCount:     evidence.SampleCount,
		Coverage:                   coverage,
		Freshness:                  domainneighborhood.FreshnessCurrent,
		InventoryCollectedAt:       &collectedAt,
		QualityState:               domainneighborhood.MarketQualitySufficient,
		CalculatedAt:               collectedAt.Add(time.Hour),
	}
	return MetricHistoryRecord{
		Metric: metric,
		Batch: CollectionRunReference{
			CollectionRunID: id,
			DataSourceID:    "source_1",
			SourceRef:       "ref-" + id,
			CollectedAt:     collectedAt,
			Coverage:        coverage,
		},
	}
}
