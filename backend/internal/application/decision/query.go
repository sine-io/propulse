package decision

import (
	"context"
	"errors"

	appcapacity "github.com/propulse/propulse/backend/internal/application/capacity"
	appneighborhood "github.com/propulse/propulse/backend/internal/application/neighborhood"
	domaindecision "github.com/propulse/propulse/backend/internal/domain/decision"
)

const demoUserID = "demo-user"

var ErrCapacityRequired = errors.New("capacity required")

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
}

func NewService(capacity CapacityReader, neighborhood NeighborhoodReader) *Service {
	return &Service{
		capacity:     capacity,
		neighborhood: neighborhood,
	}
}

type GetActionWindowQuery struct {
	NeighborhoodID string
}

func (s *Service) GetActionWindow(ctx context.Context, query GetActionWindowQuery) (domaindecision.ActionWindowResult, error) {
	capacity, err := s.capacity.LatestCalculation(ctx, appcapacity.LatestCalculationQuery{UserID: demoUserID})
	if err != nil {
		if errors.Is(err, appcapacity.ErrCalculationNotFound) {
			return domaindecision.ActionWindowResult{}, ErrCapacityRequired
		}
		return domaindecision.ActionWindowResult{}, err
	}

	watchlist, err := s.neighborhood.ListWatchlist(ctx, appneighborhood.ListWatchlistQuery{UserID: demoUserID})
	if err != nil {
		return domaindecision.ActionWindowResult{}, err
	}

	neighborhoodID := query.NeighborhoodID
	if neighborhoodID == "" && len(watchlist) > 0 {
		neighborhoodID = watchlist[0].NeighborhoodID
	}

	metric, err := s.neighborhood.LatestMetric(ctx, appneighborhood.LatestMetricQuery{NeighborhoodID: neighborhoodID})
	if err != nil {
		return domaindecision.ActionWindowResult{}, err
	}

	return domaindecision.RecommendActionWindow(domaindecision.ActionWindowInput{
		BudgetPressure:       capacity.Result.PressureLevel,
		HasDownPaymentGap:    capacity.Result.DownPaymentGap > 0,
		NeighborhoodStatus:   metric.Signal.Status,
		TargetLayoutScarcity: metric.Signal.TargetLayoutScarcity,
		AlternativesBetter:   len(watchlist) > 1,
	}), nil
}
