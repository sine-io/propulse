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
var ErrMetricStale = errors.New("metric stale")
var ErrMetricInsufficient = errors.New("metric insufficient")
var ErrInvalidNeighborhoodID = errors.New("invalid neighborhood id")

type CapacityReader interface {
	LatestCalculation(ctx context.Context, query appcapacity.LatestCalculationQuery) (appcapacity.CalculationRecord, error)
}

type NeighborhoodReader interface {
	ListWatchlist(ctx context.Context, query appneighborhood.ListWatchlistQuery) ([]appneighborhood.WatchlistItemSummary, error)
	GetNeighborhood(ctx context.Context, query appneighborhood.GetNeighborhoodQuery) (appneighborhood.Neighborhood, error)
	LatestMetric(ctx context.Context, query appneighborhood.LatestMetricQuery) (appneighborhood.MetricWithSignal, error)
}

type Service struct {
	capacity          CapacityReader
	neighborhood      NeighborhoodReader
	userID            string
	alternativePolicy domaindecision.AlternativeComparisonPolicy
}

func NewService(
	capacity CapacityReader,
	neighborhood NeighborhoodReader,
	userID string,
	alternativeRuleVersion string,
	metricAlgorithmVersion string,
) *Service {
	return &Service{
		capacity:          capacity,
		neighborhood:      neighborhood,
		userID:            userID,
		alternativePolicy: domaindecision.NewAlternativeComparisonPolicy(alternativeRuleVersion, metricAlgorithmVersion),
	}
}

type GetActionWindowQuery struct {
	NeighborhoodID string
}

func (s *Service) GetActionWindow(ctx context.Context, query GetActionWindowQuery) (ActionWindowResult, error) {
	capacity, err := s.capacity.LatestCalculation(ctx, appcapacity.LatestCalculationQuery{UserID: s.userID})
	if err != nil {
		if errors.Is(err, appcapacity.ErrCalculationNotFound) {
			return ActionWindowResult{}, ErrCapacityRequired
		}
		return ActionWindowResult{}, err
	}

	watchlist, err := s.neighborhood.ListWatchlist(ctx, appneighborhood.ListWatchlistQuery{UserID: s.userID})
	if err != nil {
		return ActionWindowResult{}, err
	}

	explicitNeighborhoodID := strings.TrimSpace(query.NeighborhoodID)
	if explicitNeighborhoodID != "" {
		if _, err := uuid.Parse(explicitNeighborhoodID); err != nil {
			return ActionWindowResult{}, ErrInvalidNeighborhoodID
		}
	}

	neighborhoodID := explicitNeighborhoodID
	if neighborhoodID == "" && len(watchlist) > 0 {
		neighborhoodID = watchlist[0].NeighborhoodID
	}
	if neighborhoodID == "" {
		return ActionWindowResult{}, ErrWatchlistRequired
	}

	metric, err := s.neighborhood.LatestMetric(ctx, appneighborhood.LatestMetricQuery{NeighborhoodID: neighborhoodID})
	if err != nil {
		if errors.Is(err, appneighborhood.ErrMetricNotFound) {
			return ActionWindowResult{}, ErrMetricRequired
		}
		return ActionWindowResult{}, err
	}
	if metric.Metric.Freshness == domainneighborhood.FreshnessStale || metric.Metric.Freshness == domainneighborhood.FreshnessExpired {
		return ActionWindowResult{}, ErrMetricStale
	}
	if metric.Metric.QualityState != domainneighborhood.MarketQualitySufficient ||
		metric.Metric.TransactionMomentum == domainneighborhood.TransactionMomentumUnknown ||
		metric.Metric.TransactionEvidence == nil ||
		metric.Signal.QualityState != domainneighborhood.MarketQualitySufficient {
		return ActionWindowResult{}, ErrMetricInsufficient
	}

	target, err := s.neighborhood.GetNeighborhood(ctx, appneighborhood.GetNeighborhoodQuery{ID: neighborhoodID})
	if err != nil {
		return ActionWindowResult{}, err
	}
	alternativeComparison, err := s.compareAlternatives(ctx, capacity, target, metric, watchlist)
	if err != nil {
		return ActionWindowResult{}, err
	}
	recommendation := domaindecision.RecommendActionWindow(domaindecision.ActionWindowInput{
		BudgetPressure:        capacity.Result.PressureLevel,
		HasDownPaymentGap:     capacity.Result.DownPaymentGap > 0,
		NeighborhoodStatus:    metric.Signal.Status,
		TargetLayoutScarcity:  metric.Signal.TargetLayoutScarcity,
		AlternativeComparison: alternativeComparison.Status,
	})
	return newActionWindowResult(capacity, target, metric, alternativeComparison, recommendation), nil
}
