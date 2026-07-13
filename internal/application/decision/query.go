package decision

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	appcapacity "github.com/sine-io/propulse/internal/application/capacity"
	appneighborhood "github.com/sine-io/propulse/internal/application/neighborhood"
	domaindecision "github.com/sine-io/propulse/internal/domain/decision"
	domainneighborhood "github.com/sine-io/propulse/internal/domain/neighborhood"
)

var ErrCapacityRequired = errors.New("capacity required")
var ErrWatchlistRequired = errors.New("watchlist required")
var ErrMetricRequired = errors.New("metric required")
var ErrInvalidNeighborhoodID = errors.New("invalid neighborhood id")

type CapacityReader interface {
	LatestCalculation(ctx context.Context, query appcapacity.LatestCalculationQuery) (appcapacity.CalculationRecord, error)
}

type NeighborhoodReader interface {
	ListWatchlist(ctx context.Context, query appneighborhood.ListWatchlistQuery) ([]appneighborhood.WatchlistItemSummary, error)
	LatestMetric(ctx context.Context, query appneighborhood.LatestMetricQuery) (appneighborhood.MetricWithSignal, error)
}

type Service struct {
	capacity     CapacityReader
	neighborhood NeighborhoodReader
	userID       string
}

func NewService(capacity CapacityReader, neighborhood NeighborhoodReader, userID string) *Service {
	return &Service{
		capacity:     capacity,
		neighborhood: neighborhood,
		userID:       userID,
	}
}

type GetActionWindowQuery struct {
	NeighborhoodID string
}

func (s *Service) GetActionWindow(ctx context.Context, query GetActionWindowQuery) (domaindecision.ActionWindowResult, error) {
	capacity, err := s.capacity.LatestCalculation(ctx, appcapacity.LatestCalculationQuery{UserID: s.userID})
	if err != nil {
		if errors.Is(err, appcapacity.ErrCalculationNotFound) {
			return domaindecision.ActionWindowResult{}, ErrCapacityRequired
		}
		return domaindecision.ActionWindowResult{}, err
	}

	watchlist, err := s.neighborhood.ListWatchlist(ctx, appneighborhood.ListWatchlistQuery{UserID: s.userID})
	if err != nil {
		return domaindecision.ActionWindowResult{}, err
	}

	explicitNeighborhoodID := strings.TrimSpace(query.NeighborhoodID)
	if explicitNeighborhoodID != "" {
		if _, err := uuid.Parse(explicitNeighborhoodID); err != nil {
			return domaindecision.ActionWindowResult{}, ErrInvalidNeighborhoodID
		}
	}

	neighborhoodID := explicitNeighborhoodID
	if neighborhoodID == "" && len(watchlist) > 0 {
		neighborhoodID = watchlist[0].NeighborhoodID
	}
	if neighborhoodID == "" {
		return domaindecision.ActionWindowResult{}, ErrWatchlistRequired
	}

	metric, err := s.neighborhood.LatestMetric(ctx, appneighborhood.LatestMetricQuery{NeighborhoodID: neighborhoodID})
	if err != nil {
		if errors.Is(err, appneighborhood.ErrMetricNotFound) {
			return domaindecision.ActionWindowResult{}, ErrMetricRequired
		}
		return domaindecision.ActionWindowResult{}, err
	}
	if metric.Signal.QualityState == domainneighborhood.MarketQualityLowConfidence ||
		metric.Signal.QualityState == domainneighborhood.MarketQualityInsufficientData {
		return domaindecision.ActionWindowResult{
			Action:     domaindecision.ActionWait,
			Confidence: domaindecision.ConfidenceLow,
			Summary:    "目标小区的市场数据尚不足以支持买入或议价建议，先补充完整且新鲜的数据。",
			Checklist:  []string{"补充最新完整挂牌和近 90 天成交记录。", "核对数据来源、覆盖范围和采集时间。"},
			Risks:      []string{"不完整或过期样本会放大单套房源对判断的影响。"},
		}, nil
	}

	return domaindecision.RecommendActionWindow(domaindecision.ActionWindowInput{
		BudgetPressure:       capacity.Result.PressureLevel,
		HasDownPaymentGap:    capacity.Result.DownPaymentGap > 0,
		NeighborhoodStatus:   metric.Signal.Status,
		TargetLayoutScarcity: metric.Signal.TargetLayoutScarcity,
		AlternativesBetter:   len(watchlist) > 1,
	}), nil
}
