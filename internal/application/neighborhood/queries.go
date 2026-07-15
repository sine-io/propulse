package neighborhood

import (
	"context"
	"slices"
	"strings"
	"time"

	domainneighborhood "github.com/sine-io/propulse/internal/domain/neighborhood"
)

type GetNeighborhoodQuery struct {
	ID string
}

func (s *Service) GetNeighborhood(ctx context.Context, query GetNeighborhoodQuery) (Neighborhood, error) {
	return s.repo.GetNeighborhood(ctx, query.ID)
}

const (
	defaultSearchPageSize = 20
	maxSearchPageSize     = 100
)

type SearchNeighborhoodsQuery struct {
	Query        string
	City         string
	Area         string
	TargetLayout string
	Page         int
	PageSize     int
}

type SearchNeighborhoodsPage struct {
	Items    []Neighborhood
	Total    int
	Page     int
	PageSize int
	Filters  NeighborhoodSearchFilters
}

// SearchNeighborhoods 归一分页参数后委托 repository 查询，返回当页结果与总数。
func (s *Service) SearchNeighborhoods(ctx context.Context, query SearchNeighborhoodsQuery) (SearchNeighborhoodsPage, error) {
	page := query.Page
	if page < 1 {
		page = 1
	}
	pageSize := query.PageSize
	if pageSize < 1 {
		pageSize = defaultSearchPageSize
	}
	if pageSize > maxSearchPageSize {
		pageSize = maxSearchPageSize
	}

	result, err := s.repo.SearchNeighborhoods(ctx, SearchNeighborhoodsInput{
		Query:        strings.TrimSpace(query.Query),
		City:         strings.TrimSpace(query.City),
		Area:         strings.TrimSpace(query.Area),
		TargetLayout: strings.TrimSpace(query.TargetLayout),
		Limit:        pageSize,
		Offset:       (page - 1) * pageSize,
	})
	if err != nil {
		return SearchNeighborhoodsPage{}, err
	}

	return SearchNeighborhoodsPage{
		Items:    result.Items,
		Total:    result.Total,
		Page:     page,
		PageSize: pageSize,
		Filters:  result.Filters,
	}, nil
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
				ID:                  item.ID,
				NeighborhoodID:      item.NeighborhoodID,
				Name:                item.Name,
				City:                item.City,
				Area:                item.Area,
				TargetLayout:        item.TargetLayout,
				Status:              domainneighborhood.NeighborhoodStatusInsufficientData,
				TransactionMomentum: domainneighborhood.TransactionMomentumUnknown,
				TargetLayoutScarcity: domainneighborhood.ScarcityUnknown,
				Advice:              "暂无指标数据，等待导入或计算后再判断。",
				HasMetric:           false,
				SourceIDs:           []string{},
				Coverage:            domainneighborhood.CoverageUnknown,
				Freshness:           domainneighborhood.FreshnessUnknown,
				QualityState:        domainneighborhood.MarketQualityInsufficientData,
				QualityWarnings:     []domainneighborhood.QualityWarning{domainneighborhood.WarningMetricUnavailable},
			})
			continue
		}

		item.Metric = projectMetric(item.Metric, item.TargetLayout)
		item.Metric = refreshMetricQuality(item.Metric, s.now())
		weeklyComparison, err := s.weeklyComparisonForMetric(ctx, item.Metric)
		if err != nil {
			return nil, err
		}
		signal := evaluateMetric(item.Name, item.Metric)
		collectedAt := item.Metric.CollectedAt
		summaries = append(summaries, WatchlistItemSummary{
			ID:                     item.ID,
			NeighborhoodID:         item.NeighborhoodID,
			Name:                   item.Name,
			City:                   item.City,
			Area:                   item.Area,
			TargetLayout:           item.TargetLayout,
			Status:                 signal.Status,
			ListedHomes:            item.Metric.ListedHomes,
			PriceCutHomes:          item.Metric.PriceCutHomes,
			TransactionMomentum:    item.Metric.TransactionMomentum,
			TargetLayoutSupply:     item.Metric.TargetLayoutSupply,
			TargetLayoutScarcity:   signal.TargetLayoutScarcity,
			Advice:                 signal.NextAction,
			HasMetric:              true,
			CollectionRunID:        item.Metric.CollectionRunID,
			AlgorithmVersion:       item.Metric.AlgorithmVersion,
			SourceIDs:              append([]string(nil), item.Metric.SourceIDs...),
			CollectedAt:            &collectedAt,
			TransactionSampleCount: item.Metric.TransactionSampleCount,
			Coverage:               item.Metric.Coverage,
			Freshness:              item.Metric.Freshness,
			QualityState:           item.Metric.QualityState,
			QualityWarnings:        append([]domainneighborhood.QualityWarning(nil), item.Metric.QualityWarnings...),
			WeeklyComparison:       weeklyComparison,
		})
	}

	return summaries, nil
}

type LatestMetricQuery struct {
	NeighborhoodID string
	TargetLayout   string
}

func (s *Service) LatestMetric(ctx context.Context, query LatestMetricQuery) (MetricWithSignal, error) {
	neighborhood, err := s.repo.GetNeighborhood(ctx, query.NeighborhoodID)
	if err != nil {
		return MetricWithSignal{}, err
	}
	targetLayout := strings.TrimSpace(query.TargetLayout)
	if targetLayout == "" || !slices.Contains(neighborhood.AvailableLayouts, targetLayout) {
		return MetricWithSignal{}, ErrInvalidTargetLayout
	}
	metric, err := s.repo.LatestMetric(ctx, query.NeighborhoodID)
	if err != nil {
		return MetricWithSignal{}, err
	}
	metric = projectMetric(metric, targetLayout)
	metric = refreshMetricQuality(metric, s.now())

	return MetricWithSignal{
		Metric: metric,
		Signal: evaluateMetric("", metric),
	}, nil
}

func projectMetric(metric MetricSnapshot, targetLayout string) MetricSnapshot {
	metric.TargetLayout = targetLayout
	metric.TargetLayoutSupply = metric.TargetLayoutSupplyByLayout[targetLayout]
	return metric
}

func refreshMetricQuality(metric MetricSnapshot, now time.Time) MetricSnapshot {
	quality := domainneighborhood.AssessQuality(domainneighborhood.QualityInput{
		Now:                    now,
		InventoryCollectedAt:   metric.InventoryCollectedAt,
		LatestCoverage:         metric.Coverage,
		HasFullInventory:       metric.InventoryCollectionRunID != nil,
		ListingSampleCount:     metric.ListingSampleCount,
		TransactionSampleCount: metric.TransactionSampleCount,
	})
	metric.Coverage = quality.Coverage
	metric.Freshness = quality.Freshness
	metric.QualityState = quality.State
	metric.QualityWarnings = quality.Warnings
	return metric
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
