package decision

import (
	"context"
	"errors"

	appcapacity "github.com/sine-io/propulse/internal/application/capacity"
	appneighborhood "github.com/sine-io/propulse/internal/application/neighborhood"
	domaindecision "github.com/sine-io/propulse/internal/domain/decision"
)

type alternativeCandidateContext struct {
	item   appneighborhood.WatchlistItemSummary
	metric *appneighborhood.MetricWithSignal
}

func (s *Service) compareAlternatives(
	ctx context.Context,
	calculation appcapacity.CalculationRecord,
	target appneighborhood.Neighborhood,
	targetLayout string,
	targetMetric appneighborhood.MetricWithSignal,
	watchlist []appneighborhood.WatchlistItemSummary,
) (AlternativeComparisonResult, error) {
	domainCandidates := make([]domaindecision.AlternativeComparable, 0, len(watchlist))
	candidateContexts := make(map[string]alternativeCandidateContext, len(watchlist))
	for _, item := range watchlist {
		if item.NeighborhoodID == target.ID {
			continue
		}

		candidateContext := alternativeCandidateContext{item: item}
		comparable := alternativeComparable(item.NeighborhoodID, item.Name, item.TargetLayout, nil)
		if item.TargetLayout == targetLayout {
			metric, err := s.neighborhood.LatestMetric(ctx, appneighborhood.LatestMetricQuery{
				NeighborhoodID: item.NeighborhoodID,
				TargetLayout:   item.TargetLayout,
			})
			if err != nil {
				if !errors.Is(err, appneighborhood.ErrMetricNotFound) {
					return AlternativeComparisonResult{}, err
				}
			} else {
				candidateContext.metric = &metric
				comparable = alternativeComparable(item.NeighborhoodID, item.Name, item.TargetLayout, &metric)
			}
		}
		candidateContexts[item.NeighborhoodID] = candidateContext
		domainCandidates = append(domainCandidates, comparable)
	}

	domainResult := s.alternativePolicy.Compare(domaindecision.AlternativeComparisonInput{
		SafeTotalPrice: calculation.Result.SafeTotalPrice,
		Target:         alternativeComparable(target.ID, target.Name, targetLayout, &targetMetric),
		Candidates:     domainCandidates,
	})
	result := AlternativeComparisonResult{
		Status:               domainResult.Status,
		RuleVersion:          domainResult.RuleVersion,
		ReferenceCollectedAt: targetMetric.Metric.CollectedAt,
		SafeTotalPrice:       calculation.Result.SafeTotalPrice,
		Candidates:           make([]AlternativeCandidateComparison, 0, len(domainResult.Candidates)),
	}
	for _, candidate := range domainResult.Candidates {
		candidateContext := candidateContexts[candidate.NeighborhoodID]
		mapped := AlternativeCandidateComparison{
			NeighborhoodID:                    candidate.NeighborhoodID,
			Name:                              candidate.Name,
			Area:                              candidateContext.item.Area,
			TargetLayout:                      candidateContext.item.TargetLayout,
			Status:                            candidate.Status,
			Reasons:                           append([]domaindecision.AlternativeComparisonReason(nil), candidate.Reasons...),
			Improvements:                      append([]domaindecision.AlternativeComparisonDimension(nil), candidate.Improvements...),
			Deteriorations:                    append([]domaindecision.AlternativeComparisonDimension(nil), candidate.Deteriorations...),
			WithinBudget:                      candidate.WithinBudget,
			TargetTransactionPriceMidpoint:    candidate.TargetTransactionPriceMidpoint,
			CandidateTransactionPriceMidpoint: candidate.CandidateTransactionPriceMidpoint,
			PriceDifference:                   candidate.PriceDifference,
			PriceDifferencePct:                candidate.PriceDifferencePct,
			TargetSignal:                      candidate.TargetSignal,
			CandidateSignal:                   candidate.CandidateSignal,
			SignalRankDifference:              candidate.SignalRankDifference,
			TargetLayoutSupply:                candidate.TargetLayoutSupply,
			CandidateTargetLayoutSupply:       candidate.CandidateTargetLayoutSupply,
			SupplyDifference:                  candidate.SupplyDifference,
			SupplyDifferencePct:               candidate.SupplyDifferencePct,
		}
		if candidateContext.metric != nil {
			metricReference := newDecisionMetricReference(candidateContext.metric.Metric)
			mapped.Metric = &metricReference
		}
		result.Candidates = append(result.Candidates, mapped)
	}
	return result, nil
}

func alternativeComparable(
	neighborhoodID string,
	name string,
	targetLayout string,
	metric *appneighborhood.MetricWithSignal,
) domaindecision.AlternativeComparable {
	comparable := domaindecision.AlternativeComparable{
		NeighborhoodID: neighborhoodID,
		Name:           name,
		TargetLayout:   targetLayout,
	}
	if metric == nil {
		return comparable
	}
	comparable.HasMetric = true
	comparable.AlgorithmVersion = metric.Metric.AlgorithmVersion
	comparable.CollectedAt = metric.Metric.CollectedAt
	comparable.Coverage = metric.Metric.Coverage
	comparable.Freshness = metric.Metric.Freshness
	comparable.QualityState = metric.Metric.QualityState
	comparable.TransactionPriceMin = metric.Metric.TransactionPriceMin
	comparable.TransactionPriceMax = metric.Metric.TransactionPriceMax
	comparable.Signal = metric.Signal.Status
	comparable.TargetLayoutSupply = metric.Metric.TargetLayoutSupply
	if metric.Metric.TransactionEvidence != nil {
		comparable.TransactionEvidenceSampleCount = metric.Metric.TransactionEvidence.SampleCount
	}
	return comparable
}
