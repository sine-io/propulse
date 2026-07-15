package decision

import (
	"context"
	"errors"
	"testing"
	"time"

	appcapacity "github.com/sine-io/propulse/internal/application/capacity"
	appneighborhood "github.com/sine-io/propulse/internal/application/neighborhood"
	"github.com/sine-io/propulse/internal/application/user"
	domaincapacity "github.com/sine-io/propulse/internal/domain/capacity"
	domaindecision "github.com/sine-io/propulse/internal/domain/decision"
	domainneighborhood "github.com/sine-io/propulse/internal/domain/neighborhood"
)

const (
	testTargetNeighborhoodID    = "11111111-1111-1111-1111-111111111111"
	testCandidateNeighborhoodID = "11111111-1111-1111-1111-111111111112"
	testUnwatchedNeighborhoodID = "11111111-1111-1111-1111-111111111113"
)

func TestGetActionWindowComposesTraceableFactorsWithoutInventingAlternativeEvidence(t *testing.T) {
	calculationTime := time.Date(2026, 7, 14, 7, 30, 0, 0, time.UTC)
	collectedAt := time.Date(2026, 7, 14, 8, 0, 0, 0, time.UTC)
	calculatedAt := time.Date(2026, 7, 14, 8, 5, 0, 0, time.UTC)
	transactionEvidence := domainneighborhood.NewTransactionMomentumEvidence(collectedAt, 1, 4)
	capacity := &stubCapacityReader{
		record: appcapacity.CalculationRecord{
			ID:        "calc_1",
			UserID:    user.SingleUserID,
			CreatedAt: calculationTime,
			Input:     domaincapacity.HousingCapacityInput{TargetTotalPrice: 520},
			Result: domaincapacity.HousingCapacityResult{
				PressureLevel:       domaincapacity.PressureStrained,
				DownPaymentGap:      0,
				SafeTotalPrice:      500,
				MonthlyPaymentRatio: 0.32,
				DeployableCash:      220,
				NetOldHomeProceeds:  180,
				RuleVersion:         "capacity/2026.07.14.1",
				TraceabilityStatus:  domaincapacity.TraceabilityComplete,
			},
		},
	}
	neighborhood := &stubNeighborhoodReader{
		watchlist: []appneighborhood.WatchlistItemSummary{
			{NeighborhoodID: testTargetNeighborhoodID, TargetLayout: "三房"},
			{NeighborhoodID: testCandidateNeighborhoodID, TargetLayout: "四房"},
		},
		neighborhood: appneighborhood.Neighborhood{
			ID: testTargetNeighborhoodID, Name: "青枫花园", Area: "滨江核心",
		},
		metric: appneighborhood.MetricWithSignal{
			Metric: appneighborhood.MetricSnapshot{
				ID:                     "metric_1",
				NeighborhoodID:         testTargetNeighborhoodID,
				CollectionRunID:        "run_1",
				AlgorithmVersion:       "market-metrics/test.1",
				SourceIDs:              []string{"source_1"},
				CollectedAt:            collectedAt,
				CalculatedAt:           calculatedAt,
				ListedHomes:            42,
				PriceCutHomes:          11,
				TransactionMomentum:    domainneighborhood.TransactionMomentumWeak,
				TransactionEvidence:    &transactionEvidence,
				TargetLayoutSupply:     8,
				ListingSampleCount:     42,
				TransactionSampleCount: 5,
				Coverage:               domainneighborhood.CoverageFull,
				Freshness:              domainneighborhood.FreshnessCurrent,
				QualityState:           domainneighborhood.MarketQualitySufficient,
			},
			Signal: domainneighborhood.SignalResult{
				Status:               domainneighborhood.NeighborhoodStatusBargain,
				SupplyPressure:       domainneighborhood.SupplyPressureHigh,
				PriceCutShare:        0.262,
				PriceGapPct:          0.06,
				TargetLayoutScarcity: domainneighborhood.ScarcityMedium,
				QualityState:         domainneighborhood.MarketQualitySufficient,
				NextAction:           "试探近期成交低位。",
			},
		},
	}

	result, err := newTestDecisionService(capacity, neighborhood, user.SingleUserID).GetActionWindow(context.Background(), GetActionWindowQuery{NeighborhoodID: testTargetNeighborhoodID})
	if err != nil {
		t.Fatalf("GetActionWindow() error = %v", err)
	}

	if capacity.userID != user.SingleUserID {
		t.Fatalf("capacity userID = %q, want %q", capacity.userID, user.SingleUserID)
	}
	if neighborhood.watchlistUserID != user.SingleUserID {
		t.Fatalf("watchlist userID = %q, want %q", neighborhood.watchlistUserID, user.SingleUserID)
	}
	if neighborhood.metricNeighborhoodID != testTargetNeighborhoodID {
		t.Fatalf("metric neighborhoodID = %q, want %q", neighborhood.metricNeighborhoodID, testTargetNeighborhoodID)
	}
	if len(neighborhood.metricQueries) != 1 || neighborhood.metricQueries[0].TargetLayout != "三房" {
		t.Fatalf("metric queries = %#v, want target watchlist layout 三房", neighborhood.metricQueries)
	}
	if result.Action != domaindecision.ActionBargain || result.Confidence != domaindecision.ConfidenceMedium {
		t.Fatalf("result = %#v", result)
	}
	if result.Target.NeighborhoodID != testTargetNeighborhoodID || result.Target.Name != "青枫花园" || result.Target.TargetLayout != "三房" {
		t.Fatalf("Target = %#v", result.Target)
	}
	if result.CapacityCalculation.ID != "calc_1" || !result.CapacityCalculation.CreatedAt.Equal(calculationTime) || result.CapacityCalculation.RuleVersion != "capacity/2026.07.14.1" {
		t.Fatalf("CapacityCalculation = %#v", result.CapacityCalculation)
	}
	if result.Metric.ID != "metric_1" || result.Metric.CollectionRunID != "run_1" || result.Metric.AlgorithmVersion != "market-metrics/test.1" || !result.Metric.CollectedAt.Equal(collectedAt) || result.Metric.TransactionSampleCount != 5 {
		t.Fatalf("Metric = %#v", result.Metric)
	}
	wantKeys := []FactorKey{
		FactorBudgetPressure, FactorDownPaymentGap, FactorMarketSignal,
		FactorTransactionMomentum, FactorTargetLayoutSupply, FactorAlternatives,
	}
	if len(result.Factors) != len(wantKeys) {
		t.Fatalf("Factors = %#v", result.Factors)
	}
	for index, key := range wantKeys {
		if result.Factors[index].Key != key {
			t.Fatalf("Factors[%d].Key = %q, want %q", index, result.Factors[index].Key, key)
		}
	}
	if result.Factors[0].Status != FactorStatusCaution || result.Factors[0].Source == nil || result.Factors[0].Source.ID != "calc_1" {
		t.Fatalf("budget factor = %#v", result.Factors[0])
	}
	if targetPrice := result.Factors[0].Evidence[1].NumberValue; targetPrice == nil || *targetPrice != 520 {
		t.Fatalf("target total price evidence = %#v", result.Factors[0].Evidence)
	}
	if result.Factors[3].Status != FactorStatusPositive || len(result.Factors[3].Evidence) != 8 || result.Factors[3].Source == nil || result.Factors[3].Source.ID != "metric_1" {
		t.Fatalf("transaction factor = %#v", result.Factors[3])
	}
	if result.Factors[4].Summary != "目标户型当前供给 8 套，稀缺度为中。" {
		t.Fatalf("target layout factor = %#v", result.Factors[4])
	}
	alternatives := result.Factors[5]
	if alternatives.Status != FactorStatusNeutral || alternatives.Source == nil || alternatives.Source.ID != "alternative-comparison/test.1" || len(alternatives.Evidence) != 5 {
		t.Fatalf("alternatives factor = %#v", alternatives)
	}
	if result.AlternativeComparison.Status != domaindecision.AlternativeComparisonNone || len(result.AlternativeComparison.Candidates) != 1 || result.AlternativeComparison.Candidates[0].Status != domaindecision.AlternativeCandidateNotBetter || result.AlternativeComparison.Candidates[0].Reasons[0] != domaindecision.AlternativeReasonLayoutMismatch {
		t.Fatalf("AlternativeComparison = %#v", result.AlternativeComparison)
	}
	if len(result.ConfidenceReasons) != 1 || result.ConfidenceReasons[0] != "目标小区支持议价，但备选比较没有发现满足规则的更优候选。" {
		t.Fatalf("ConfidenceReasons = %#v", result.ConfidenceReasons)
	}
}

func TestGetActionWindowUsesRequestedNeighborhoodID(t *testing.T) {
	requestedNeighborhoodID := testCandidateNeighborhoodID
	transactionEvidence := domainneighborhood.NewTransactionMomentumEvidence(time.Date(2026, 7, 14, 0, 0, 0, 0, time.UTC), 3, 1)
	neighborhood := &stubNeighborhoodReader{
		watchlist: []appneighborhood.WatchlistItemSummary{
			{NeighborhoodID: testTargetNeighborhoodID, Name: "备选小区", TargetLayout: "三房"},
			{NeighborhoodID: requestedNeighborhoodID, TargetLayout: "两房"},
		},
		neighborhood: appneighborhood.Neighborhood{ID: requestedNeighborhoodID, Name: "请求小区", Area: "南城"},
		metric: appneighborhood.MetricWithSignal{
			Metric: appneighborhood.MetricSnapshot{
				TransactionMomentum: domainneighborhood.TransactionMomentumStrong,
				TransactionEvidence: &transactionEvidence,
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

	result, err := newTestDecisionService(&stubCapacityReader{
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
	if len(neighborhood.metricQueries) == 0 || neighborhood.metricQueries[0].TargetLayout != "两房" {
		t.Fatalf("metric queries = %#v, want requested watchlist layout 两房", neighborhood.metricQueries)
	}
	if result.Action != domaindecision.ActionAct {
		t.Fatalf("Action = %q, want %q", result.Action, domaindecision.ActionAct)
	}
	if len(result.AlternativeComparison.Candidates) != 1 || result.AlternativeComparison.Candidates[0].NeighborhoodID != testTargetNeighborhoodID {
		t.Fatalf("alternative candidates = %#v, want selected target excluded and other watchlist item compared", result.AlternativeComparison.Candidates)
	}
}

func TestGetActionWindowRaisesBargainConfidenceForTraceableBetterAlternative(t *testing.T) {
	targetMetric := alternativeMetricFixture(testTargetNeighborhoodID, 500, domainneighborhood.NeighborhoodStatusBargain, 10)
	candidateMetric := alternativeMetricFixture(testCandidateNeighborhoodID, 450, domainneighborhood.NeighborhoodStatusBargain, 12)
	neighborhood := &stubNeighborhoodReader{
		watchlist: []appneighborhood.WatchlistItemSummary{
			{NeighborhoodID: testTargetNeighborhoodID, Name: "目标小区", TargetLayout: "三房"},
			{NeighborhoodID: testCandidateNeighborhoodID, Name: "更优候选", Area: "南城", TargetLayout: "三房"},
		},
		neighborhood: appneighborhood.Neighborhood{ID: testTargetNeighborhoodID, Name: "目标小区", Area: "北城"},
		metrics: map[string]appneighborhood.MetricWithSignal{
			testTargetNeighborhoodID: targetMetric, testCandidateNeighborhoodID: candidateMetric,
		},
	}
	result, err := newTestDecisionService(&stubCapacityReader{record: appcapacity.CalculationRecord{
		ID: "calc", Result: domaincapacity.HousingCapacityResult{PressureLevel: domaincapacity.PressureSafe, SafeTotalPrice: 500},
	}}, neighborhood, user.SingleUserID).GetActionWindow(context.Background(), GetActionWindowQuery{NeighborhoodID: testTargetNeighborhoodID})
	if err != nil {
		t.Fatalf("GetActionWindow() error = %v", err)
	}
	if result.Action != domaindecision.ActionBargain || result.Confidence != domaindecision.ConfidenceHigh || result.AlternativeComparison.Status != domaindecision.AlternativeComparisonBetterFound {
		t.Fatalf("result = %#v", result)
	}
	if len(result.AlternativeComparison.Candidates) != 1 {
		t.Fatalf("Candidates = %#v", result.AlternativeComparison.Candidates)
	}
	candidate := result.AlternativeComparison.Candidates[0]
	if candidate.Status != domaindecision.AlternativeCandidateBetter || candidate.Metric == nil || candidate.Metric.CollectionRunID != "run_"+testCandidateNeighborhoodID || len(candidate.Improvements) != 2 || len(candidate.Deteriorations) != 0 {
		t.Fatalf("candidate = %#v", candidate)
	}
	if len(result.ConfidenceReasons) != 1 || result.ConfidenceReasons[0] != "目标小区支持议价，且版本化比较发现至少一个预算内更优备选。" {
		t.Fatalf("ConfidenceReasons = %#v", result.ConfidenceReasons)
	}
	alternativeFactor := result.Factors[5]
	if alternativeFactor.Status != FactorStatusPositive || alternativeFactor.Source == nil || alternativeFactor.Source.ID != "alternative-comparison/test.1" {
		t.Fatalf("alternative factor = %#v", alternativeFactor)
	}
}

func TestGetActionWindowKeepsBargainConfidenceMediumWhenAlternativeMetricIsMissing(t *testing.T) {
	targetMetric := alternativeMetricFixture(testTargetNeighborhoodID, 500, domainneighborhood.NeighborhoodStatusBargain, 10)
	neighborhood := &stubNeighborhoodReader{
		watchlist: []appneighborhood.WatchlistItemSummary{
			{NeighborhoodID: testTargetNeighborhoodID, Name: "目标小区", TargetLayout: "三房"},
			{NeighborhoodID: testCandidateNeighborhoodID, Name: "缺指标候选", TargetLayout: "三房"},
		},
		neighborhood: appneighborhood.Neighborhood{ID: testTargetNeighborhoodID, Name: "目标小区"},
		metrics:      map[string]appneighborhood.MetricWithSignal{testTargetNeighborhoodID: targetMetric},
		metricErrors: map[string]error{testCandidateNeighborhoodID: appneighborhood.ErrMetricNotFound},
	}
	result, err := newTestDecisionService(&stubCapacityReader{record: appcapacity.CalculationRecord{
		Result: domaincapacity.HousingCapacityResult{PressureLevel: domaincapacity.PressureSafe, SafeTotalPrice: 500},
	}}, neighborhood, user.SingleUserID).GetActionWindow(context.Background(), GetActionWindowQuery{NeighborhoodID: testTargetNeighborhoodID})
	if err != nil {
		t.Fatalf("GetActionWindow() error = %v", err)
	}
	if result.Confidence != domaindecision.ConfidenceMedium || result.AlternativeComparison.Status != domaindecision.AlternativeComparisonUnknown {
		t.Fatalf("result = %#v", result)
	}
	candidate := result.AlternativeComparison.Candidates[0]
	if candidate.Status != domaindecision.AlternativeCandidateUnknown || candidate.Metric != nil || candidate.Reasons[0] != domaindecision.AlternativeReasonMetricMissing {
		t.Fatalf("candidate = %#v", candidate)
	}
	if result.Factors[5].Status != FactorStatusUnknown {
		t.Fatalf("alternative factor = %#v", result.Factors[5])
	}
}

func TestGetActionWindowFailsWhenAlternativeMetricReadFails(t *testing.T) {
	targetMetric := alternativeMetricFixture(testTargetNeighborhoodID, 500, domainneighborhood.NeighborhoodStatusBargain, 10)
	readErr := errors.New("candidate metric read failed")
	neighborhood := &stubNeighborhoodReader{
		watchlist: []appneighborhood.WatchlistItemSummary{
			{NeighborhoodID: testTargetNeighborhoodID, Name: "目标小区", TargetLayout: "三房"},
			{NeighborhoodID: testCandidateNeighborhoodID, Name: "读取失败候选", TargetLayout: "三房"},
		},
		neighborhood: appneighborhood.Neighborhood{ID: testTargetNeighborhoodID, Name: "目标小区"},
		metrics:      map[string]appneighborhood.MetricWithSignal{testTargetNeighborhoodID: targetMetric},
		metricErrors: map[string]error{testCandidateNeighborhoodID: readErr},
	}
	result, err := newTestDecisionService(&stubCapacityReader{record: appcapacity.CalculationRecord{
		Result: domaincapacity.HousingCapacityResult{PressureLevel: domaincapacity.PressureSafe, SafeTotalPrice: 500},
	}}, neighborhood, user.SingleUserID).GetActionWindow(context.Background(), GetActionWindowQuery{NeighborhoodID: testTargetNeighborhoodID})
	if !errors.Is(err, readErr) {
		t.Fatalf("result/error = %#v/%v, want candidate read error", result, err)
	}
	assertEmptyActionWindow(t, result)
}

func TestGetActionWindowReturnsCapacityRequiredWhenMissingLatestCalculation(t *testing.T) {
	_, err := newTestDecisionService(
		&stubCapacityReader{err: appcapacity.ErrCalculationNotFound},
		&stubNeighborhoodReader{watchlist: []appneighborhood.WatchlistItemSummary{{NeighborhoodID: testTargetNeighborhoodID}}},
		user.SingleUserID,
	).GetActionWindow(context.Background(), GetActionWindowQuery{NeighborhoodID: testTargetNeighborhoodID})

	if !errors.Is(err, ErrCapacityRequired) {
		t.Fatalf("error = %v, want ErrCapacityRequired", err)
	}
}

func TestGetActionWindowReturnsWatchlistRequiredWithoutFallingBackToFirstItem(t *testing.T) {
	capacity := &stubCapacityReader{
		record: appcapacity.CalculationRecord{
			Result: domaincapacity.HousingCapacityResult{PressureLevel: domaincapacity.PressureSafe},
		},
	}
	neighborhood := &stubNeighborhoodReader{
		watchlist: []appneighborhood.WatchlistItemSummary{{NeighborhoodID: testTargetNeighborhoodID}},
	}

	_, err := newTestDecisionService(capacity, neighborhood, user.SingleUserID).
		GetActionWindow(context.Background(), GetActionWindowQuery{NeighborhoodID: " \t "})

	if !errors.Is(err, ErrWatchlistRequired) {
		t.Fatalf("error = %v, want ErrWatchlistRequired", err)
	}
	if capacity.called {
		t.Fatal("LatestCalculation was called without an explicit neighborhood")
	}
	if neighborhood.watchlistCalled {
		t.Fatal("ListWatchlist was called without an explicit neighborhood")
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

	_, err := newTestDecisionService(&stubCapacityReader{
		record: appcapacity.CalculationRecord{
			Result: domaincapacity.HousingCapacityResult{PressureLevel: domaincapacity.PressureSafe},
		},
	}, neighborhood, user.SingleUserID).GetActionWindow(context.Background(), GetActionWindowQuery{NeighborhoodID: "not-a-uuid"})

	if !errors.Is(err, ErrInvalidNeighborhoodID) {
		t.Fatalf("error = %v, want ErrInvalidNeighborhoodID", err)
	}
	if neighborhood.watchlistCalled {
		t.Fatal("ListWatchlist was called for a malformed explicit neighborhood ID")
	}
	if neighborhood.metricCalled {
		t.Fatal("LatestMetric was called for a malformed explicit neighborhood ID")
	}
}

func TestGetActionWindowReturnsNeighborhoodNotWatched(t *testing.T) {
	capacity := &stubCapacityReader{}
	neighborhood := &stubNeighborhoodReader{
		watchlist: []appneighborhood.WatchlistItemSummary{{NeighborhoodID: testTargetNeighborhoodID}},
	}

	result, err := newTestDecisionService(capacity, neighborhood, user.SingleUserID).
		GetActionWindow(context.Background(), GetActionWindowQuery{NeighborhoodID: testUnwatchedNeighborhoodID})

	if !errors.Is(err, ErrNeighborhoodNotWatched) {
		t.Fatalf("result/error = %#v/%v, want ErrNeighborhoodNotWatched", result, err)
	}
	assertEmptyActionWindow(t, result)
	if capacity.called || neighborhood.metricCalled || neighborhood.neighborhoodCalled {
		t.Fatal("decision data was read for a neighborhood outside the watchlist")
	}
}

func TestGetActionWindowReturnsNeighborhoodNotWatchedForEmptyWatchlist(t *testing.T) {
	capacity := &stubCapacityReader{}
	neighborhood := &stubNeighborhoodReader{}

	_, err := newTestDecisionService(capacity, neighborhood, user.SingleUserID).
		GetActionWindow(context.Background(), GetActionWindowQuery{NeighborhoodID: testTargetNeighborhoodID})

	if !errors.Is(err, ErrNeighborhoodNotWatched) {
		t.Fatalf("error = %v, want ErrNeighborhoodNotWatched", err)
	}
	if capacity.called || neighborhood.metricCalled || neighborhood.neighborhoodCalled {
		t.Fatal("decision data was read for an empty watchlist")
	}
}

func TestGetActionWindowReturnsMetricRequiredWhenLatestMetricIsMissing(t *testing.T) {
	_, err := newTestDecisionService(&stubCapacityReader{
		record: appcapacity.CalculationRecord{
			Result: domaincapacity.HousingCapacityResult{PressureLevel: domaincapacity.PressureSafe},
		},
	}, &stubNeighborhoodReader{
		watchlist: []appneighborhood.WatchlistItemSummary{
			{NeighborhoodID: testTargetNeighborhoodID},
		},
		metricErr: appneighborhood.ErrMetricNotFound,
	}, user.SingleUserID).GetActionWindow(context.Background(), GetActionWindowQuery{NeighborhoodID: testTargetNeighborhoodID})

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
			result, err := newTestDecisionService(&stubCapacityReader{record: appcapacity.CalculationRecord{
				Result: domaincapacity.HousingCapacityResult{PressureLevel: domaincapacity.PressureSafe},
			}}, &stubNeighborhoodReader{
				watchlist: []appneighborhood.WatchlistItemSummary{{NeighborhoodID: testTargetNeighborhoodID}},
				metric: appneighborhood.MetricWithSignal{
					Metric: appneighborhood.MetricSnapshot{Freshness: domainneighborhood.FreshnessCurrent, QualityState: state, TransactionMomentum: domainneighborhood.TransactionMomentumWeak},
					Signal: domainneighborhood.SignalResult{Status: domainneighborhood.NeighborhoodStatusBargain, QualityState: state},
				},
			}, user.SingleUserID).GetActionWindow(context.Background(), GetActionWindowQuery{NeighborhoodID: testTargetNeighborhoodID})
			if !errors.Is(err, ErrMetricInsufficient) {
				t.Fatalf("error = %v, want ErrMetricInsufficient", err)
			}
			assertEmptyActionWindow(t, result)
		})
	}
}

func TestGetActionWindowReturnsMetricInsufficientForUnknownMomentum(t *testing.T) {
	result, err := newTestDecisionService(&stubCapacityReader{record: appcapacity.CalculationRecord{
		Result: domaincapacity.HousingCapacityResult{PressureLevel: domaincapacity.PressureSafe},
	}}, &stubNeighborhoodReader{
		watchlist: []appneighborhood.WatchlistItemSummary{{NeighborhoodID: testTargetNeighborhoodID}},
		metric: appneighborhood.MetricWithSignal{
			Metric: appneighborhood.MetricSnapshot{Freshness: domainneighborhood.FreshnessCurrent, QualityState: domainneighborhood.MarketQualitySufficient, TransactionMomentum: domainneighborhood.TransactionMomentumUnknown},
			Signal: domainneighborhood.SignalResult{QualityState: domainneighborhood.MarketQualitySufficient},
		},
	}, user.SingleUserID).GetActionWindow(context.Background(), GetActionWindowQuery{NeighborhoodID: testTargetNeighborhoodID})
	if !errors.Is(err, ErrMetricInsufficient) {
		t.Fatalf("result/error = %#v/%v", result, err)
	}
	assertEmptyActionWindow(t, result)
}

func TestGetActionWindowReturnsMetricInsufficientWithoutTransactionWindowEvidence(t *testing.T) {
	result, err := newTestDecisionService(&stubCapacityReader{record: appcapacity.CalculationRecord{
		Result: domaincapacity.HousingCapacityResult{PressureLevel: domaincapacity.PressureSafe},
	}}, &stubNeighborhoodReader{
		watchlist: []appneighborhood.WatchlistItemSummary{{NeighborhoodID: testTargetNeighborhoodID}},
		metric: appneighborhood.MetricWithSignal{
			Metric: appneighborhood.MetricSnapshot{
				Freshness: domainneighborhood.FreshnessCurrent, QualityState: domainneighborhood.MarketQualitySufficient,
				TransactionMomentum: domainneighborhood.TransactionMomentumWeak, TransactionEvidence: nil,
			},
			Signal: domainneighborhood.SignalResult{QualityState: domainneighborhood.MarketQualitySufficient},
		},
	}, user.SingleUserID).GetActionWindow(context.Background(), GetActionWindowQuery{NeighborhoodID: testTargetNeighborhoodID})
	if !errors.Is(err, ErrMetricInsufficient) {
		t.Fatalf("result/error = %#v/%v", result, err)
	}
	assertEmptyActionWindow(t, result)
}

func TestGetActionWindowReturnsMetricStaleWithoutRecommendation(t *testing.T) {
	for _, freshness := range []domainneighborhood.Freshness{domainneighborhood.FreshnessStale, domainneighborhood.FreshnessExpired} {
		t.Run(string(freshness), func(t *testing.T) {
			result, err := newTestDecisionService(&stubCapacityReader{record: appcapacity.CalculationRecord{
				Result: domaincapacity.HousingCapacityResult{PressureLevel: domaincapacity.PressureSafe},
			}}, &stubNeighborhoodReader{
				watchlist: []appneighborhood.WatchlistItemSummary{{NeighborhoodID: testTargetNeighborhoodID}},
				metric: appneighborhood.MetricWithSignal{
					Metric: appneighborhood.MetricSnapshot{Freshness: freshness, QualityState: domainneighborhood.MarketQualityLowConfidence, TransactionMomentum: domainneighborhood.TransactionMomentumWeak},
					Signal: domainneighborhood.SignalResult{QualityState: domainneighborhood.MarketQualityLowConfidence},
				},
			}, user.SingleUserID).GetActionWindow(context.Background(), GetActionWindowQuery{NeighborhoodID: testTargetNeighborhoodID})
			if !errors.Is(err, ErrMetricStale) {
				t.Fatalf("result/error = %#v/%v", result, err)
			}
			assertEmptyActionWindow(t, result)
		})
	}
}

func assertEmptyActionWindow(t *testing.T, result ActionWindowResult) {
	t.Helper()
	if result.Action != "" || result.Confidence != "" || result.Summary != "" || len(result.ConfidenceReasons) != 0 || len(result.Factors) != 0 || len(result.Checklist) != 0 || len(result.Risks) != 0 {
		t.Fatalf("result = %#v, want no recommendation fields", result)
	}
}

type stubCapacityReader struct {
	userID string
	called bool
	record appcapacity.CalculationRecord
	err    error
}

func (s *stubCapacityReader) LatestCalculation(_ context.Context, query appcapacity.LatestCalculationQuery) (appcapacity.CalculationRecord, error) {
	s.called = true
	s.userID = query.UserID
	if s.err != nil {
		return appcapacity.CalculationRecord{}, s.err
	}
	return s.record, nil
}

type stubNeighborhoodReader struct {
	watchlistUserID      string
	watchlistCalled      bool
	metricNeighborhoodID string
	metricQueries        []appneighborhood.LatestMetricQuery
	metricCalled         bool
	neighborhoodCalled   bool
	watchlist            []appneighborhood.WatchlistItemSummary
	neighborhood         appneighborhood.Neighborhood
	metric               appneighborhood.MetricWithSignal
	metrics              map[string]appneighborhood.MetricWithSignal
	err                  error
	neighborhoodErr      error
	metricErr            error
	metricErrors         map[string]error
}

func (s *stubNeighborhoodReader) ListWatchlist(_ context.Context, query appneighborhood.ListWatchlistQuery) ([]appneighborhood.WatchlistItemSummary, error) {
	s.watchlistCalled = true
	s.watchlistUserID = query.UserID
	if s.err != nil {
		return nil, s.err
	}
	return s.watchlist, nil
}

func (s *stubNeighborhoodReader) GetNeighborhood(_ context.Context, query appneighborhood.GetNeighborhoodQuery) (appneighborhood.Neighborhood, error) {
	s.neighborhoodCalled = true
	if s.neighborhoodErr != nil {
		return appneighborhood.Neighborhood{}, s.neighborhoodErr
	}
	if s.neighborhood.ID == "" {
		s.neighborhood.ID = query.ID
	}
	return s.neighborhood, nil
}

func (s *stubNeighborhoodReader) LatestMetric(_ context.Context, query appneighborhood.LatestMetricQuery) (appneighborhood.MetricWithSignal, error) {
	s.metricCalled = true
	s.metricNeighborhoodID = query.NeighborhoodID
	s.metricQueries = append(s.metricQueries, query)
	if err := s.metricErrors[query.NeighborhoodID]; err != nil {
		return appneighborhood.MetricWithSignal{}, err
	}
	if s.metricErr != nil {
		return appneighborhood.MetricWithSignal{}, s.metricErr
	}
	if metric, ok := s.metrics[query.NeighborhoodID]; ok {
		return metric, nil
	}
	return s.metric, nil
}

func newTestDecisionService(capacity CapacityReader, neighborhood NeighborhoodReader, userID string) *Service {
	return NewService(capacity, neighborhood, userID, "alternative-comparison/test.1", "market-metrics/test.1")
}

func alternativeMetricFixture(
	id string,
	transactionMidpoint float64,
	status domainneighborhood.NeighborhoodStatus,
	supply int,
) appneighborhood.MetricWithSignal {
	minimum := transactionMidpoint - 5
	maximum := transactionMidpoint + 5
	evidence := domainneighborhood.NewTransactionMomentumEvidence(time.Date(2026, 7, 14, 8, 0, 0, 0, time.UTC), 1, 2)
	return appneighborhood.MetricWithSignal{
		Metric: appneighborhood.MetricSnapshot{
			ID: "metric_" + id, NeighborhoodID: id, CollectionRunID: "run_" + id,
			AlgorithmVersion: "market-metrics/test.1", CollectedAt: time.Date(2026, 7, 14, 8, 0, 0, 0, time.UTC),
			CalculatedAt: time.Date(2026, 7, 14, 8, 5, 0, 0, time.UTC), TransactionPriceMin: &minimum, TransactionPriceMax: &maximum,
			TransactionMomentum: domainneighborhood.TransactionMomentumWeak, TransactionEvidence: &evidence,
			TargetLayoutSupply: supply, ListingSampleCount: 10, TransactionSampleCount: 3,
			Coverage: domainneighborhood.CoverageFull, Freshness: domainneighborhood.FreshnessCurrent,
			QualityState: domainneighborhood.MarketQualitySufficient,
		},
		Signal: domainneighborhood.SignalResult{
			Status: status, TargetLayoutScarcity: domainneighborhood.ScarcityMedium,
			QualityState: domainneighborhood.MarketQualitySufficient,
		},
	}
}
