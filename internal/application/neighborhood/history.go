package neighborhood

import (
	"context"
	"errors"
	"time"

	domainneighborhood "github.com/sine-io/propulse/internal/domain/neighborhood"
)

const (
	defaultMetricHistoryWindow = 8 * 7 * 24 * time.Hour
	maximumMetricHistoryWindow = 52 * 7 * 24 * time.Hour
	comparisonLookback         = 45 * 24 * time.Hour
)

var ErrInvalidMetricHistoryWindow = errors.New("invalid_metric_history_window")

type MetricHistoryQuery struct {
	NeighborhoodID string
	From           time.Time
	To             time.Time
}

func (s *Service) MetricHistory(ctx context.Context, query MetricHistoryQuery) (MetricHistoryResult, error) {
	if _, err := s.repo.GetNeighborhood(ctx, query.NeighborhoodID); err != nil {
		return MetricHistoryResult{}, err
	}

	from, to, err := s.resolveMetricHistoryWindow(query)
	if err != nil {
		return MetricHistoryResult{}, err
	}
	records, err := s.repo.ListMetricHistory(ctx, MetricHistoryRepositoryQuery{
		NeighborhoodID: query.NeighborhoodID,
		From:           from.Add(-comparisonLookback),
		To:             to,
	})
	if err != nil {
		return MetricHistoryResult{}, err
	}

	result := MetricHistoryResult{
		Status:           MetricHistoryEmpty,
		NeighborhoodID:   query.NeighborhoodID,
		AlgorithmVersion: s.algorithmVersion,
		From:             from,
		To:               to,
		Items:            []MetricHistoryPoint{},
	}
	for index, record := range records {
		if record.Batch.CollectedAt.Before(from) || record.Batch.CollectedAt.After(to) {
			continue
		}
		metric := refreshMetricQuality(record.Metric, s.now())
		if result.AlgorithmVersion == "" {
			result.AlgorithmVersion = metric.AlgorithmVersion
		}
		result.Items = append(result.Items, MetricHistoryPoint{
			Metric:            metric,
			Batch:             record.Batch,
			WeeklyComparison:  buildMetricComparison(record, records[:index], 14*24*time.Hour, 7*24*time.Hour),
			MonthlyComparison: buildMetricComparison(record, records[:index], 45*24*time.Hour, 30*24*time.Hour),
		})
	}
	if len(result.Items) > 0 {
		result.Status = MetricHistoryReady
	}
	return result, nil
}

func (s *Service) resolveMetricHistoryWindow(query MetricHistoryQuery) (time.Time, time.Time, error) {
	to := query.To.UTC()
	if query.To.IsZero() {
		to = s.now().UTC()
	}
	from := query.From.UTC()
	if query.From.IsZero() {
		from = to.Add(-defaultMetricHistoryWindow)
	}
	if from.After(to) || to.Sub(from) > maximumMetricHistoryWindow {
		return time.Time{}, time.Time{}, ErrInvalidMetricHistoryWindow
	}
	return from, to, nil
}

func buildMetricComparison(current MetricHistoryRecord, candidates []MetricHistoryRecord, windowStartAgo, windowEndAgo time.Duration) MetricComparison {
	comparison := MetricComparison{
		Status:       domainneighborhood.MetricComparisonUnavailable,
		CurrentBatch: current.Batch,
	}
	if current.Batch.Coverage != domainneighborhood.CoverageFull {
		comparison.Reason = domainneighborhood.ComparisonReasonCurrentPartialCoverage
		return comparison
	}
	baseline := latestFullBaseline(candidates, current.Batch.CollectedAt.Add(-windowStartAgo), current.Batch.CollectedAt.Add(-windowEndAgo))
	if baseline == nil {
		comparison.Reason = domainneighborhood.ComparisonReasonFullBaselineNotFound
		return comparison
	}
	comparison.BaselineBatch = &baseline.Batch
	if current.Metric.TransactionEvidence == nil || baseline.Metric.TransactionEvidence == nil {
		comparison.Reason = domainneighborhood.ComparisonReasonTransactionEvidenceMissing
		return comparison
	}

	comparison.Status = domainneighborhood.MetricComparisonAvailable
	comparison.ListedHomes = metricChangePtr(domainneighborhood.CalculateMetricChange(current.Metric.ListedHomes, baseline.Metric.ListedHomes))
	comparison.PriceCutHomes = metricChangePtr(domainneighborhood.CalculateMetricChange(current.Metric.PriceCutHomes, baseline.Metric.PriceCutHomes))
	comparison.RecentThirtyDayTransactions = metricChangePtr(domainneighborhood.CalculateMetricChange(
		current.Metric.TransactionEvidence.RecentThirtyDayCount,
		baseline.Metric.TransactionEvidence.RecentThirtyDayCount,
	))
	return comparison
}

func latestFullBaseline(candidates []MetricHistoryRecord, from, to time.Time) *MetricHistoryRecord {
	var latest *MetricHistoryRecord
	for index := range candidates {
		candidate := &candidates[index]
		if candidate.Batch.Coverage != domainneighborhood.CoverageFull || candidate.Batch.CollectedAt.Before(from) || candidate.Batch.CollectedAt.After(to) {
			continue
		}
		if latest == nil || candidate.Batch.CollectedAt.After(latest.Batch.CollectedAt) ||
			(candidate.Batch.CollectedAt.Equal(latest.Batch.CollectedAt) && candidate.Batch.CollectionRunID > latest.Batch.CollectionRunID) {
			latest = candidate
		}
	}
	return latest
}

func metricChangePtr(value domainneighborhood.MetricChangeValue) *domainneighborhood.MetricChangeValue {
	return &value
}
