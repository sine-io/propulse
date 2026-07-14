package decision

import (
	"context"
	"errors"
	"testing"

	appcapacity "github.com/sine-io/propulse/internal/application/capacity"
	appneighborhood "github.com/sine-io/propulse/internal/application/neighborhood"
	"github.com/sine-io/propulse/internal/application/user"
	domaincapacity "github.com/sine-io/propulse/internal/domain/capacity"
	domaindecision "github.com/sine-io/propulse/internal/domain/decision"
	domainneighborhood "github.com/sine-io/propulse/internal/domain/neighborhood"
)

func TestGetActionWindowComposesLatestCapacityFirstWatchlistMetricAndAlternatives(t *testing.T) {
	capacity := &stubCapacityReader{
		record: appcapacity.CalculationRecord{
			ID:     "calc_1",
			UserID: user.SingleUserID,
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
			Metric: appneighborhood.MetricSnapshot{
				TransactionMomentum: domainneighborhood.TransactionMomentumWeak,
				Freshness:           domainneighborhood.FreshnessCurrent,
				QualityState:        domainneighborhood.MarketQualitySufficient,
			},
			Signal: domainneighborhood.SignalResult{
				Status:               domainneighborhood.NeighborhoodStatusBargain,
				TargetLayoutScarcity: domainneighborhood.ScarcityMedium,
				QualityState:         domainneighborhood.MarketQualitySufficient,
			},
		},
	}

	result, err := NewService(capacity, neighborhood, user.SingleUserID).GetActionWindow(context.Background(), GetActionWindowQuery{})
	if err != nil {
		t.Fatalf("GetActionWindow() error = %v", err)
	}

	if capacity.userID != user.SingleUserID {
		t.Fatalf("capacity userID = %q, want %q", capacity.userID, user.SingleUserID)
	}
	if neighborhood.watchlistUserID != user.SingleUserID {
		t.Fatalf("watchlist userID = %q, want %q", neighborhood.watchlistUserID, user.SingleUserID)
	}
	if neighborhood.metricNeighborhoodID != "neighborhood_1" {
		t.Fatalf("metric neighborhoodID = %q, want neighborhood_1", neighborhood.metricNeighborhoodID)
	}
	if result.Action != domaindecision.ActionBargain || result.Confidence != domaindecision.ConfidenceHigh {
		t.Fatalf("result = %#v", result)
	}
}

func TestGetActionWindowUsesRequestedNeighborhoodID(t *testing.T) {
	requestedNeighborhoodID := "11111111-1111-1111-1111-111111111112"
	neighborhood := &stubNeighborhoodReader{
		watchlist: []appneighborhood.WatchlistItemSummary{
			{NeighborhoodID: "11111111-1111-1111-1111-111111111111"},
			{NeighborhoodID: requestedNeighborhoodID},
		},
		metric: appneighborhood.MetricWithSignal{
			Metric: appneighborhood.MetricSnapshot{
				TransactionMomentum: domainneighborhood.TransactionMomentumStrong,
				Freshness:           domainneighborhood.FreshnessCurrent,
				QualityState:        domainneighborhood.MarketQualitySufficient,
			},
			Signal: domainneighborhood.SignalResult{
				Status:               domainneighborhood.NeighborhoodStatusFocus,
				TargetLayoutScarcity: domainneighborhood.ScarcityHigh,
				QualityState:         domainneighborhood.MarketQualitySufficient,
			},
		},
	}

	result, err := NewService(&stubCapacityReader{
		record: appcapacity.CalculationRecord{
			Result: domaincapacity.HousingCapacityResult{PressureLevel: domaincapacity.PressureSafe},
		},
	}, neighborhood, user.SingleUserID).GetActionWindow(context.Background(), GetActionWindowQuery{NeighborhoodID: requestedNeighborhoodID})
	if err != nil {
		t.Fatalf("GetActionWindow() error = %v", err)
	}

	if neighborhood.metricNeighborhoodID != requestedNeighborhoodID {
		t.Fatalf("metric neighborhoodID = %q, want %q", neighborhood.metricNeighborhoodID, requestedNeighborhoodID)
	}
	if result.Action != domaindecision.ActionAct {
		t.Fatalf("Action = %q, want %q", result.Action, domaindecision.ActionAct)
	}
}

func TestGetActionWindowReturnsCapacityRequiredWhenMissingLatestCalculation(t *testing.T) {
	_, err := NewService(&stubCapacityReader{err: appcapacity.ErrCalculationNotFound}, &stubNeighborhoodReader{}, user.SingleUserID).
		GetActionWindow(context.Background(), GetActionWindowQuery{})

	if !errors.Is(err, ErrCapacityRequired) {
		t.Fatalf("error = %v, want ErrCapacityRequired", err)
	}
}

func TestGetActionWindowReturnsWatchlistRequiredWhenDefaultNeighborhoodIsUnavailable(t *testing.T) {
	neighborhood := &stubNeighborhoodReader{}

	_, err := NewService(&stubCapacityReader{
		record: appcapacity.CalculationRecord{
			Result: domaincapacity.HousingCapacityResult{PressureLevel: domaincapacity.PressureSafe},
		},
	}, neighborhood, user.SingleUserID).GetActionWindow(context.Background(), GetActionWindowQuery{})

	if !errors.Is(err, ErrWatchlistRequired) {
		t.Fatalf("error = %v, want ErrWatchlistRequired", err)
	}
	if neighborhood.metricCalled {
		t.Fatal("LatestMetric was called without a neighborhood")
	}
}

func TestGetActionWindowReturnsInvalidNeighborhoodIDForMalformedExplicitID(t *testing.T) {
	neighborhood := &stubNeighborhoodReader{
		watchlist: []appneighborhood.WatchlistItemSummary{
			{NeighborhoodID: "11111111-1111-1111-1111-111111111111"},
		},
	}

	_, err := NewService(&stubCapacityReader{
		record: appcapacity.CalculationRecord{
			Result: domaincapacity.HousingCapacityResult{PressureLevel: domaincapacity.PressureSafe},
		},
	}, neighborhood, user.SingleUserID).GetActionWindow(context.Background(), GetActionWindowQuery{NeighborhoodID: "not-a-uuid"})

	if !errors.Is(err, ErrInvalidNeighborhoodID) {
		t.Fatalf("error = %v, want ErrInvalidNeighborhoodID", err)
	}
	if neighborhood.metricCalled {
		t.Fatal("LatestMetric was called for a malformed explicit neighborhood ID")
	}
}

func TestGetActionWindowReturnsMetricRequiredWhenLatestMetricIsMissing(t *testing.T) {
	_, err := NewService(&stubCapacityReader{
		record: appcapacity.CalculationRecord{
			Result: domaincapacity.HousingCapacityResult{PressureLevel: domaincapacity.PressureSafe},
		},
	}, &stubNeighborhoodReader{
		watchlist: []appneighborhood.WatchlistItemSummary{
			{NeighborhoodID: "neighborhood_1"},
		},
		metricErr: appneighborhood.ErrMetricNotFound,
	}, user.SingleUserID).GetActionWindow(context.Background(), GetActionWindowQuery{})

	if !errors.Is(err, ErrMetricRequired) {
		t.Fatalf("error = %v, want ErrMetricRequired", err)
	}
}

func TestGetActionWindowReturnsMetricInsufficientWithoutRecommendation(t *testing.T) {
	for _, state := range []domainneighborhood.MarketQualityState{
		domainneighborhood.MarketQualityLowConfidence,
		domainneighborhood.MarketQualityInsufficientData,
	} {
		t.Run(string(state), func(t *testing.T) {
			result, err := NewService(&stubCapacityReader{record: appcapacity.CalculationRecord{
				Result: domaincapacity.HousingCapacityResult{PressureLevel: domaincapacity.PressureSafe},
			}}, &stubNeighborhoodReader{
				watchlist: []appneighborhood.WatchlistItemSummary{{NeighborhoodID: "neighborhood_1"}},
				metric: appneighborhood.MetricWithSignal{
					Metric: appneighborhood.MetricSnapshot{Freshness: domainneighborhood.FreshnessCurrent, QualityState: state, TransactionMomentum: domainneighborhood.TransactionMomentumWeak},
					Signal: domainneighborhood.SignalResult{Status: domainneighborhood.NeighborhoodStatusBargain, QualityState: state},
				},
			}, user.SingleUserID).GetActionWindow(context.Background(), GetActionWindowQuery{})
			if !errors.Is(err, ErrMetricInsufficient) {
				t.Fatalf("error = %v, want ErrMetricInsufficient", err)
			}
			assertEmptyActionWindow(t, result)
		})
	}
}

func TestGetActionWindowReturnsMetricInsufficientForUnknownMomentum(t *testing.T) {
	result, err := NewService(&stubCapacityReader{record: appcapacity.CalculationRecord{
		Result: domaincapacity.HousingCapacityResult{PressureLevel: domaincapacity.PressureSafe},
	}}, &stubNeighborhoodReader{
		watchlist: []appneighborhood.WatchlistItemSummary{{NeighborhoodID: "neighborhood_1"}},
		metric: appneighborhood.MetricWithSignal{
			Metric: appneighborhood.MetricSnapshot{Freshness: domainneighborhood.FreshnessCurrent, QualityState: domainneighborhood.MarketQualitySufficient, TransactionMomentum: domainneighborhood.TransactionMomentumUnknown},
			Signal: domainneighborhood.SignalResult{QualityState: domainneighborhood.MarketQualitySufficient},
		},
	}, user.SingleUserID).GetActionWindow(context.Background(), GetActionWindowQuery{})
	if !errors.Is(err, ErrMetricInsufficient) {
		t.Fatalf("result/error = %#v/%v", result, err)
	}
	assertEmptyActionWindow(t, result)
}

func TestGetActionWindowReturnsMetricStaleWithoutRecommendation(t *testing.T) {
	for _, freshness := range []domainneighborhood.Freshness{domainneighborhood.FreshnessStale, domainneighborhood.FreshnessExpired} {
		t.Run(string(freshness), func(t *testing.T) {
			result, err := NewService(&stubCapacityReader{record: appcapacity.CalculationRecord{
				Result: domaincapacity.HousingCapacityResult{PressureLevel: domaincapacity.PressureSafe},
			}}, &stubNeighborhoodReader{
				watchlist: []appneighborhood.WatchlistItemSummary{{NeighborhoodID: "neighborhood_1"}},
				metric: appneighborhood.MetricWithSignal{
					Metric: appneighborhood.MetricSnapshot{Freshness: freshness, QualityState: domainneighborhood.MarketQualityLowConfidence, TransactionMomentum: domainneighborhood.TransactionMomentumWeak},
					Signal: domainneighborhood.SignalResult{QualityState: domainneighborhood.MarketQualityLowConfidence},
				},
			}, user.SingleUserID).GetActionWindow(context.Background(), GetActionWindowQuery{})
			if !errors.Is(err, ErrMetricStale) {
				t.Fatalf("result/error = %#v/%v", result, err)
			}
			assertEmptyActionWindow(t, result)
		})
	}
}

func assertEmptyActionWindow(t *testing.T, result domaindecision.ActionWindowResult) {
	t.Helper()
	if result.Action != "" || result.Confidence != "" || result.Summary != "" || len(result.Checklist) != 0 || len(result.Risks) != 0 {
		t.Fatalf("result = %#v, want no recommendation fields", result)
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
	metricCalled         bool
	watchlist            []appneighborhood.WatchlistItemSummary
	metric               appneighborhood.MetricWithSignal
	err                  error
	metricErr            error
}

func (s *stubNeighborhoodReader) ListWatchlist(_ context.Context, query appneighborhood.ListWatchlistQuery) ([]appneighborhood.WatchlistItemSummary, error) {
	s.watchlistUserID = query.UserID
	if s.err != nil {
		return nil, s.err
	}
	return s.watchlist, nil
}

func (s *stubNeighborhoodReader) LatestMetric(_ context.Context, query appneighborhood.LatestMetricQuery) (appneighborhood.MetricWithSignal, error) {
	s.metricCalled = true
	s.metricNeighborhoodID = query.NeighborhoodID
	if s.metricErr != nil {
		return appneighborhood.MetricWithSignal{}, s.metricErr
	}
	return s.metric, nil
}
