package decision

import (
	"slices"
	"testing"
	"time"

	domainneighborhood "github.com/sine-io/propulse/internal/domain/neighborhood"
)

const testMetricAlgorithmVersion = "market-metrics/test.1"
const testAlternativeRuleVersion = "alternative-comparison/test.1"

func TestAlternativeComparisonBetterThresholds(t *testing.T) {
	tests := []struct {
		name               string
		safeTotalPrice     float64
		targetPrice        float64
		candidatePrice     float64
		targetSignal       domainneighborhood.NeighborhoodStatus
		candidateSignal    domainneighborhood.NeighborhoodStatus
		targetSupply       int
		candidateSupply    int
		wantStatus         AlternativeCandidateStatus
		wantImprovements   []AlternativeComparisonDimension
		wantDeteriorations []AlternativeComparisonDimension
	}{
		{
			name: "price and signal improve at threshold", safeTotalPrice: 500, targetPrice: 500, candidatePrice: 475,
			targetSignal: domainneighborhood.NeighborhoodStatusObserve, candidateSignal: domainneighborhood.NeighborhoodStatusFocus,
			targetSupply: 10, candidateSupply: 10, wantStatus: AlternativeCandidateBetter,
			wantImprovements: []AlternativeComparisonDimension{AlternativeDimensionTransactionPrice, AlternativeDimensionMarketSignal},
		},
		{
			name: "price and supply improve by absolute and relative thresholds", safeTotalPrice: 500, targetPrice: 500, candidatePrice: 470,
			targetSignal: domainneighborhood.NeighborhoodStatusObserve, candidateSignal: domainneighborhood.NeighborhoodStatusObserve,
			targetSupply: 10, candidateSupply: 12, wantStatus: AlternativeCandidateBetter,
			wantImprovements: []AlternativeComparisonDimension{AlternativeDimensionTransactionPrice, AlternativeDimensionTargetLayoutSupply},
		},
		{
			name: "zero target supply improves at two", safeTotalPrice: 500, targetPrice: 500, candidatePrice: 470,
			targetSignal: domainneighborhood.NeighborhoodStatusObserve, candidateSignal: domainneighborhood.NeighborhoodStatusObserve,
			targetSupply: 0, candidateSupply: 2, wantStatus: AlternativeCandidateBetter,
			wantImprovements: []AlternativeComparisonDimension{AlternativeDimensionTransactionPrice, AlternativeDimensionTargetLayoutSupply},
		},
		{
			name: "one improvement is not enough", safeTotalPrice: 500, targetPrice: 500, candidatePrice: 470,
			targetSignal: domainneighborhood.NeighborhoodStatusObserve, candidateSignal: domainneighborhood.NeighborhoodStatusObserve,
			targetSupply: 10, candidateSupply: 11, wantStatus: AlternativeCandidateNotBetter,
			wantImprovements: []AlternativeComparisonDimension{AlternativeDimensionTransactionPrice},
		},
		{
			name: "deterioration blocks two improvements", safeTotalPrice: 600, targetPrice: 500, candidatePrice: 525,
			targetSignal: domainneighborhood.NeighborhoodStatusPriceHard, candidateSignal: domainneighborhood.NeighborhoodStatusFocus,
			targetSupply: 10, candidateSupply: 12, wantStatus: AlternativeCandidateNotBetter,
			wantImprovements:   []AlternativeComparisonDimension{AlternativeDimensionMarketSignal, AlternativeDimensionTargetLayoutSupply},
			wantDeteriorations: []AlternativeComparisonDimension{AlternativeDimensionTransactionPrice},
		},
		{
			name: "supply deterioration blocks price and signal improvements", safeTotalPrice: 500, targetPrice: 500, candidatePrice: 470,
			targetSignal: domainneighborhood.NeighborhoodStatusObserve, candidateSignal: domainneighborhood.NeighborhoodStatusFocus,
			targetSupply: 10, candidateSupply: 8, wantStatus: AlternativeCandidateNotBetter,
			wantImprovements:   []AlternativeComparisonDimension{AlternativeDimensionTransactionPrice, AlternativeDimensionMarketSignal},
			wantDeteriorations: []AlternativeComparisonDimension{AlternativeDimensionTargetLayoutSupply},
		},
		{
			name: "signal deterioration blocks price and supply improvements", safeTotalPrice: 500, targetPrice: 500, candidatePrice: 470,
			targetSignal: domainneighborhood.NeighborhoodStatusFocus, candidateSignal: domainneighborhood.NeighborhoodStatusObserve,
			targetSupply: 10, candidateSupply: 12, wantStatus: AlternativeCandidateNotBetter,
			wantImprovements:   []AlternativeComparisonDimension{AlternativeDimensionTransactionPrice, AlternativeDimensionTargetLayoutSupply},
			wantDeteriorations: []AlternativeComparisonDimension{AlternativeDimensionMarketSignal},
		},
		{
			name: "over budget blocks otherwise better candidate", safeTotalPrice: 430, targetPrice: 500, candidatePrice: 440,
			targetSignal: domainneighborhood.NeighborhoodStatusObserve, candidateSignal: domainneighborhood.NeighborhoodStatusBargain,
			targetSupply: 10, candidateSupply: 12, wantStatus: AlternativeCandidateNotBetter,
			wantImprovements: []AlternativeComparisonDimension{AlternativeDimensionTransactionPrice, AlternativeDimensionMarketSignal, AlternativeDimensionTargetLayoutSupply},
		},
	}

	policy := NewAlternativeComparisonPolicy(testAlternativeRuleVersion, testMetricAlgorithmVersion)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target := comparableFixture("target", "目标", tt.targetPrice, tt.targetSignal, tt.targetSupply)
			candidate := comparableFixture("candidate", "候选", tt.candidatePrice, tt.candidateSignal, tt.candidateSupply)
			result := policy.Compare(AlternativeComparisonInput{SafeTotalPrice: tt.safeTotalPrice, Target: target, Candidates: []AlternativeComparable{candidate}})
			got := result.Candidates[0]
			if got.Status != tt.wantStatus || !slices.Equal(got.Improvements, tt.wantImprovements) || !slices.Equal(got.Deteriorations, tt.wantDeteriorations) {
				t.Fatalf("candidate = %#v", got)
			}
		})
	}
}

func TestAlternativeComparisonDataEligibility(t *testing.T) {
	tests := []struct {
		name       string
		mutate     func(*AlternativeComparable)
		wantStatus AlternativeCandidateStatus
		wantReason AlternativeComparisonReason
	}{
		{name: "layout mismatch is definitively not better", mutate: func(value *AlternativeComparable) { value.TargetLayout = "两房" }, wantStatus: AlternativeCandidateNotBetter, wantReason: AlternativeReasonLayoutMismatch},
		{name: "metric missing", mutate: func(value *AlternativeComparable) { value.HasMetric = false }, wantStatus: AlternativeCandidateUnknown, wantReason: AlternativeReasonMetricMissing},
		{name: "algorithm mismatch", mutate: func(value *AlternativeComparable) { value.AlgorithmVersion = "old" }, wantStatus: AlternativeCandidateUnknown, wantReason: AlternativeReasonAlgorithmVersionMismatch},
		{name: "partial coverage", mutate: func(value *AlternativeComparable) { value.Coverage = domainneighborhood.CoveragePartial }, wantStatus: AlternativeCandidateUnknown, wantReason: AlternativeReasonCoverageNotFull},
		{name: "stale metric", mutate: func(value *AlternativeComparable) { value.Freshness = domainneighborhood.FreshnessStale }, wantStatus: AlternativeCandidateUnknown, wantReason: AlternativeReasonMetricNotCurrent},
		{name: "insufficient quality", mutate: func(value *AlternativeComparable) { value.QualityState = domainneighborhood.MarketQualityLowConfidence }, wantStatus: AlternativeCandidateUnknown, wantReason: AlternativeReasonMetricQualityInsufficient},
		{name: "too few transaction samples", mutate: func(value *AlternativeComparable) { value.TransactionEvidenceSampleCount = 2 }, wantStatus: AlternativeCandidateUnknown, wantReason: AlternativeReasonTransactionEvidenceInsufficient},
		{name: "outside seven day window", mutate: func(value *AlternativeComparable) {
			value.CollectedAt = value.CollectedAt.Add(7*time.Hour*24 + time.Second)
		}, wantStatus: AlternativeCandidateUnknown, wantReason: AlternativeReasonComparisonWindowMismatch},
		{name: "transaction price missing", mutate: func(value *AlternativeComparable) { value.TransactionPriceMin = nil }, wantStatus: AlternativeCandidateUnknown, wantReason: AlternativeReasonTransactionPriceMissing},
		{name: "signal unknown", mutate: func(value *AlternativeComparable) {
			value.Signal = domainneighborhood.NeighborhoodStatusInsufficientData
		}, wantStatus: AlternativeCandidateUnknown, wantReason: AlternativeReasonSignalNotComparable},
	}

	policy := NewAlternativeComparisonPolicy(testAlternativeRuleVersion, testMetricAlgorithmVersion)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target := comparableFixture("target", "目标", 500, domainneighborhood.NeighborhoodStatusObserve, 10)
			candidate := comparableFixture("candidate", "候选", 470, domainneighborhood.NeighborhoodStatusFocus, 12)
			tt.mutate(&candidate)
			got := policy.Compare(AlternativeComparisonInput{SafeTotalPrice: 500, Target: target, Candidates: []AlternativeComparable{candidate}}).Candidates[0]
			if got.Status != tt.wantStatus || !slices.Contains(got.Reasons, tt.wantReason) {
				t.Fatalf("candidate = %#v, want status/reason %q/%q", got, tt.wantStatus, tt.wantReason)
			}
		})
	}
}

func TestAlternativeComparisonOverallStatusAndStableSort(t *testing.T) {
	policy := NewAlternativeComparisonPolicy(testAlternativeRuleVersion, testMetricAlgorithmVersion)
	target := comparableFixture("target", "目标", 500, domainneighborhood.NeighborhoodStatusObserve, 10)
	betterHigh := comparableFixture("better-high", "乙候选", 460, domainneighborhood.NeighborhoodStatusFocus, 12)
	betterLow := comparableFixture("better-low", "甲候选", 450, domainneighborhood.NeighborhoodStatusFocus, 12)
	notBetter := comparableFixture("not-better", "普通候选", 490, domainneighborhood.NeighborhoodStatusObserve, 10)
	unknown := comparableFixture("unknown", "未知候选", 470, domainneighborhood.NeighborhoodStatusFocus, 12)
	unknown.HasMetric = false

	tests := []struct {
		name       string
		candidates []AlternativeComparable
		want       AlternativeComparisonStatus
	}{
		{name: "no candidates", want: AlternativeComparisonNone},
		{name: "all unknown", candidates: []AlternativeComparable{unknown}, want: AlternativeComparisonUnknown},
		{name: "evaluable but none better", candidates: []AlternativeComparable{notBetter}, want: AlternativeComparisonNone},
		{name: "better found", candidates: []AlternativeComparable{notBetter, unknown, betterHigh}, want: AlternativeComparisonBetterFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := policy.Compare(AlternativeComparisonInput{SafeTotalPrice: 500, Target: target, Candidates: tt.candidates})
			if got.Status != tt.want || got.RuleVersion != testAlternativeRuleVersion {
				t.Fatalf("result = %#v", got)
			}
		})
	}

	sorted := policy.Compare(AlternativeComparisonInput{
		SafeTotalPrice: 500,
		Target:         target,
		Candidates:     []AlternativeComparable{unknown, betterHigh, notBetter, betterLow},
	}).Candidates
	wantIDs := []string{"better-low", "better-high", "not-better", "unknown"}
	for index, wantID := range wantIDs {
		if sorted[index].NeighborhoodID != wantID {
			t.Fatalf("sorted[%d] = %q, want %q; all=%#v", index, sorted[index].NeighborhoodID, wantID, sorted)
		}
	}
}

func comparableFixture(id, name string, transactionMidpoint float64, signal domainneighborhood.NeighborhoodStatus, supply int) AlternativeComparable {
	minimum := transactionMidpoint - 5
	maximum := transactionMidpoint + 5
	return AlternativeComparable{
		NeighborhoodID: id, Name: name, TargetLayout: "三房", HasMetric: true,
		AlgorithmVersion: testMetricAlgorithmVersion,
		CollectedAt:      time.Date(2026, 7, 14, 8, 0, 0, 0, time.UTC),
		Coverage:         domainneighborhood.CoverageFull, Freshness: domainneighborhood.FreshnessCurrent,
		QualityState: domainneighborhood.MarketQualitySufficient, TransactionEvidenceSampleCount: 3,
		TransactionPriceMin: &minimum, TransactionPriceMax: &maximum,
		Signal: signal, TargetLayoutSupply: supply,
	}
}
