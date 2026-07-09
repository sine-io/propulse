package neighborhood

import (
	"context"

	domainneighborhood "github.com/propulse/propulse/backend/internal/domain/neighborhood"
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
		ListingPriceRange:     domainneighborhood.PriceRange{Min: metric.ListingPriceMin, Max: metric.ListingPriceMax},
		TransactionPriceRange: domainneighborhood.PriceRange{Min: metric.TransactionPriceMin, Max: metric.TransactionPriceMax},
		ListedHomes:           metric.ListedHomes,
		ListedHomesChangePct:  metric.ListedHomesChangePct,
		PriceCutHomes:         metric.PriceCutHomes,
		AvgDaysOnMarket:       metric.AvgDaysOnMarket,
		TransactionMomentum:   metric.TransactionMomentum,
		TargetLayoutSupply:    metric.TargetLayoutSupply,
	})
}
