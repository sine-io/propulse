package neighborhood

import (
	"context"

	domainneighborhood "github.com/sine-io/propulse/internal/domain/neighborhood"
)

type GetNeighborhoodQuery struct {
	ID string
}

func (s *Service) GetNeighborhood(ctx context.Context, query GetNeighborhoodQuery) (Neighborhood, error) {
	return s.repo.GetNeighborhood(ctx, query.ID)
}

type ListWatchlistQuery struct {
	UserID string
}

func (s *Service) ListWatchlist(ctx context.Context, query ListWatchlistQuery) ([]WatchlistItemSummary, error) {
	items, err := s.repo.ListWatchlist(ctx, query.UserID)
	if err != nil {
		return nil, err
	}

	summaries := make([]WatchlistItemSummary, 0, len(items))
	for _, item := range items {
		if !item.HasMetric {
			summaries = append(summaries, WatchlistItemSummary{
				ID:             item.ID,
				NeighborhoodID: item.NeighborhoodID,
				Name:           item.Name,
				Area:           item.Area,
				TargetLayout:   item.TargetLayout,
				Status:         domainneighborhood.NeighborhoodStatusObserve,
				Advice:         "暂无指标数据，等待导入或计算后再判断。",
			})
			continue
		}

		signal := evaluateMetric(item.Name, item.Metric)
		summaries = append(summaries, WatchlistItemSummary{
			ID:                  item.ID,
			NeighborhoodID:      item.NeighborhoodID,
			Name:                item.Name,
			Area:                item.Area,
			TargetLayout:        item.TargetLayout,
			Status:              signal.Status,
			ListedHomes:         item.Metric.ListedHomes,
			PriceCutHomes:       item.Metric.PriceCutHomes,
			TransactionMomentum: item.Metric.TransactionMomentum,
			Advice:              signal.NextAction,
		})
	}

	return summaries, nil
}

type ListWatchlistNeighborhoodIDsQuery struct{}

func (s *Service) ListWatchlistNeighborhoodIDs(ctx context.Context, _ ListWatchlistNeighborhoodIDsQuery) ([]string, error) {
	return s.repo.ListWatchlistNeighborhoodIDs(ctx)
}

type LatestMetricQuery struct {
	NeighborhoodID string
}

func (s *Service) LatestMetric(ctx context.Context, query LatestMetricQuery) (MetricWithSignal, error) {
	metric, err := s.repo.LatestMetric(ctx, query.NeighborhoodID)
	if err != nil {
		return MetricWithSignal{}, err
	}

	return MetricWithSignal{
		Metric: metric,
		Signal: evaluateMetric("", metric),
	}, nil
}

func evaluateMetric(name string, metric MetricSnapshot) domainneighborhood.SignalResult {
	return domainneighborhood.EvaluateSignal(domainneighborhood.SignalInput{
		Name:                  name,
		ListingPriceRange:     domainneighborhood.PriceRange{Min: derefFloat(metric.ListingPriceMin), Max: derefFloat(metric.ListingPriceMax)},
		TransactionPriceRange: domainneighborhood.PriceRange{Min: derefFloat(metric.TransactionPriceMin), Max: derefFloat(metric.TransactionPriceMax)},
		ListedHomes:           metric.ListedHomes,
		ListedHomesChangePct:  derefFloat(metric.ListedHomesChangePct),
		PriceCutHomes:         metric.PriceCutHomes,
		AvgDaysOnMarket:       derefFloat(metric.AvgDaysOnMarket),
		TransactionMomentum:   metric.TransactionMomentum,
		TargetLayoutSupply:    metric.TargetLayoutSupply,
		Quality: domainneighborhood.QualityAssessment{
			Coverage:     metric.Coverage,
			Freshness:    metric.Freshness,
			State:        metric.QualityState,
			CanRecommend: metric.QualityState == domainneighborhood.MarketQualitySufficient,
			Warnings:     metric.QualityWarnings,
		},
	})
}

func derefFloat(value *float64) float64 {
	if value == nil {
		return 0
	}
	return *value
}
