package decision

import (
	"context"
	"errors"
	"testing"

	appcapacity "github.com/propulse/propulse/backend/internal/application/capacity"
	appneighborhood "github.com/propulse/propulse/backend/internal/application/neighborhood"
	domaincapacity "github.com/propulse/propulse/backend/internal/domain/capacity"
	domaindecision "github.com/propulse/propulse/backend/internal/domain/decision"
	domainneighborhood "github.com/propulse/propulse/backend/internal/domain/neighborhood"
)

func TestGetActionWindowComposesLatestCapacityFirstWatchlistMetricAndAlternatives(t *testing.T) {
	capacity := &stubCapacityReader{
		record: appcapacity.CalculationRecord{
			ID:     "calc_1",
			UserID: "demo-user",
			Result: domaincapacity.HousingCapacityResult{
				PressureLevel:  domaincapacity.PressureStrained,
				DownPaymentGap: 0,
			},
		},
	}
	neighborhood := &stubNeighborhoodReader{
		watchlist: []appneighborhood.WatchlistItemSummary{
			{NeighborhoodID: "neighborhood_1"},
			{NeighborhoodID: "neighborhood_2"},
		},
		metric: appneighborhood.MetricWithSignal{
			Signal: domainneighborhood.SignalResult{
				Status:               domainneighborhood.NeighborhoodStatusBargain,
				TargetLayoutScarcity: domainneighborhood.ScarcityMedium,
			},
		},
	}

	result, err := NewService(capacity, neighborhood).GetActionWindow(context.Background(), GetActionWindowQuery{})
	if err != nil {
		t.Fatalf("GetActionWindow() error = %v", err)
	}

	if capacity.userID != "demo-user" {
		t.Fatalf("capacity userID = %q, want demo-user", capacity.userID)
	}
	if neighborhood.watchlistUserID != "demo-user" {
		t.Fatalf("watchlist userID = %q, want demo-user", neighborhood.watchlistUserID)
	}
	if neighborhood.metricNeighborhoodID != "neighborhood_1" {
		t.Fatalf("metric neighborhoodID = %q, want neighborhood_1", neighborhood.metricNeighborhoodID)
	}
	if result.Action != domaindecision.ActionBargain || result.Confidence != domaindecision.ConfidenceHigh {
		t.Fatalf("result = %#v", result)
	}
}

func TestGetActionWindowUsesRequestedNeighborhoodID(t *testing.T) {
	neighborhood := &stubNeighborhoodReader{
		watchlist: []appneighborhood.WatchlistItemSummary{
			{NeighborhoodID: "neighborhood_1"},
			{NeighborhoodID: "neighborhood_2"},
		},
		metric: appneighborhood.MetricWithSignal{
			Signal: domainneighborhood.SignalResult{
				Status:               domainneighborhood.NeighborhoodStatusFocus,
				TargetLayoutScarcity: domainneighborhood.ScarcityHigh,
			},
		},
	}

	result, err := NewService(&stubCapacityReader{
		record: appcapacity.CalculationRecord{
			Result: domaincapacity.HousingCapacityResult{PressureLevel: domaincapacity.PressureSafe},
		},
	}, neighborhood).GetActionWindow(context.Background(), GetActionWindowQuery{NeighborhoodID: "neighborhood_2"})
	if err != nil {
		t.Fatalf("GetActionWindow() error = %v", err)
	}

	if neighborhood.metricNeighborhoodID != "neighborhood_2" {
		t.Fatalf("metric neighborhoodID = %q, want neighborhood_2", neighborhood.metricNeighborhoodID)
	}
	if result.Action != domaindecision.ActionAct {
		t.Fatalf("Action = %q, want %q", result.Action, domaindecision.ActionAct)
	}
}

func TestGetActionWindowReturnsCapacityRequiredWhenMissingLatestCalculation(t *testing.T) {
	_, err := NewService(&stubCapacityReader{err: appcapacity.ErrCalculationNotFound}, &stubNeighborhoodReader{}).
		GetActionWindow(context.Background(), GetActionWindowQuery{})

	if !errors.Is(err, ErrCapacityRequired) {
		t.Fatalf("error = %v, want ErrCapacityRequired", err)
	}
}

type stubCapacityReader struct {
	userID string
	record appcapacity.CalculationRecord
	err    error
}

func (s *stubCapacityReader) LatestCalculation(_ context.Context, query appcapacity.LatestCalculationQuery) (appcapacity.CalculationRecord, error) {
	s.userID = query.UserID
	if s.err != nil {
		return appcapacity.CalculationRecord{}, s.err
	}
	return s.record, nil
}

type stubNeighborhoodReader struct {
	watchlistUserID      string
	metricNeighborhoodID string
	watchlist            []appneighborhood.WatchlistItemSummary
	metric               appneighborhood.MetricWithSignal
	err                  error
}

func (s *stubNeighborhoodReader) ListWatchlist(_ context.Context, query appneighborhood.ListWatchlistQuery) ([]appneighborhood.WatchlistItemSummary, error) {
	s.watchlistUserID = query.UserID
	if s.err != nil {
		return nil, s.err
	}
	return s.watchlist, nil
}

func (s *stubNeighborhoodReader) LatestMetric(_ context.Context, query appneighborhood.LatestMetricQuery) (appneighborhood.MetricWithSignal, error) {
	s.metricNeighborhoodID = query.NeighborhoodID
	if s.err != nil {
		return appneighborhood.MetricWithSignal{}, s.err
	}
	return s.metric, nil
}
