package decision

import (
	"sort"
	"time"

	domainneighborhood "github.com/sine-io/propulse/internal/domain/neighborhood"
)

const (
	alternativeComparisonWindow        = 7 * 24 * time.Hour
	alternativePriceThreshold          = 0.05
	alternativeSupplyRelativeThreshold = 0.20
	alternativeSupplyAbsoluteThreshold = 2
	minimumAlternativeImprovements     = 2
	minimumAlternativeTransactions     = 3
)

type AlternativeComparisonStatus string

const (
	AlternativeComparisonBetterFound AlternativeComparisonStatus = "better_found"
	AlternativeComparisonNone        AlternativeComparisonStatus = "none"
	AlternativeComparisonUnknown     AlternativeComparisonStatus = "unknown"
)

type AlternativeCandidateStatus string

const (
	AlternativeCandidateBetter    AlternativeCandidateStatus = "better"
	AlternativeCandidateNotBetter AlternativeCandidateStatus = "not_better"
	AlternativeCandidateUnknown   AlternativeCandidateStatus = "unknown"
)

type AlternativeComparisonReason string

const (
	AlternativeReasonLayoutMismatch                  AlternativeComparisonReason = "layout_mismatch"
	AlternativeReasonMetricMissing                   AlternativeComparisonReason = "metric_missing"
	AlternativeReasonAlgorithmVersionMismatch        AlternativeComparisonReason = "algorithm_version_mismatch"
	AlternativeReasonCoverageNotFull                 AlternativeComparisonReason = "coverage_not_full"
	AlternativeReasonMetricNotCurrent                AlternativeComparisonReason = "metric_not_current"
	AlternativeReasonMetricQualityInsufficient       AlternativeComparisonReason = "metric_quality_insufficient"
	AlternativeReasonTransactionEvidenceInsufficient AlternativeComparisonReason = "transaction_evidence_insufficient"
	AlternativeReasonComparisonWindowMismatch        AlternativeComparisonReason = "comparison_window_mismatch"
	AlternativeReasonTransactionPriceMissing         AlternativeComparisonReason = "transaction_price_missing"
	AlternativeReasonTargetEvidenceInsufficient      AlternativeComparisonReason = "target_evidence_insufficient"
	AlternativeReasonSignalNotComparable             AlternativeComparisonReason = "signal_not_comparable"
	AlternativeReasonOverBudget                      AlternativeComparisonReason = "over_budget"
	AlternativeReasonInsufficientImprovements        AlternativeComparisonReason = "insufficient_improvements"
	AlternativeReasonDeteriorationPresent            AlternativeComparisonReason = "deterioration_present"
	AlternativeReasonBetterThresholdMet              AlternativeComparisonReason = "better_threshold_met"
)

type AlternativeComparisonDimension string

const (
	AlternativeDimensionTransactionPrice   AlternativeComparisonDimension = "transaction_price"
	AlternativeDimensionMarketSignal       AlternativeComparisonDimension = "market_signal"
	AlternativeDimensionTargetLayoutSupply AlternativeComparisonDimension = "target_layout_supply"
)

type AlternativeComparisonPolicy struct {
	RuleVersion            string
	MetricAlgorithmVersion string
}

type AlternativeComparable struct {
	NeighborhoodID                 string
	Name                           string
	TargetLayout                   string
	HasMetric                      bool
	AlgorithmVersion               string
	CollectedAt                    time.Time
	Coverage                       domainneighborhood.Coverage
	Freshness                      domainneighborhood.Freshness
	QualityState                   domainneighborhood.MarketQualityState
	TransactionEvidenceSampleCount int
	TransactionPriceMin            *float64
	TransactionPriceMax            *float64
	Signal                         domainneighborhood.NeighborhoodStatus
	TargetLayoutSupply             int
}

type AlternativeComparisonInput struct {
	SafeTotalPrice float64
	Target         AlternativeComparable
	Candidates     []AlternativeComparable
}

type AlternativeComparisonResult struct {
	Status      AlternativeComparisonStatus
	RuleVersion string
	Candidates  []AlternativeCandidateResult
}

type AlternativeCandidateResult struct {
	NeighborhoodID                    string
	Name                              string
	Status                            AlternativeCandidateStatus
	Reasons                           []AlternativeComparisonReason
	Improvements                      []AlternativeComparisonDimension
	Deteriorations                    []AlternativeComparisonDimension
	WithinBudget                      *bool
	TargetTransactionPriceMidpoint    *float64
	CandidateTransactionPriceMidpoint *float64
	PriceDifference                   *float64
	PriceDifferencePct                *float64
	TargetSignal                      *domainneighborhood.NeighborhoodStatus
	CandidateSignal                   *domainneighborhood.NeighborhoodStatus
	SignalRankDifference              *int
	TargetLayoutSupply                int
	CandidateTargetLayoutSupply       *int
	SupplyDifference                  *int
	SupplyDifferencePct               *float64
}

func NewAlternativeComparisonPolicy(ruleVersion, metricAlgorithmVersion string) AlternativeComparisonPolicy {
	return AlternativeComparisonPolicy{RuleVersion: ruleVersion, MetricAlgorithmVersion: metricAlgorithmVersion}
}

func (policy AlternativeComparisonPolicy) Compare(input AlternativeComparisonInput) AlternativeComparisonResult {
	result := AlternativeComparisonResult{
		Status:      AlternativeComparisonNone,
		RuleVersion: policy.RuleVersion,
		Candidates:  make([]AlternativeCandidateResult, 0, len(input.Candidates)),
	}
	if len(input.Candidates) == 0 {
		return result
	}

	targetReady := policy.comparableEvidenceReady(input.Target)
	for _, candidate := range input.Candidates {
		candidateResult := AlternativeCandidateResult{
			NeighborhoodID:     candidate.NeighborhoodID,
			Name:               candidate.Name,
			Status:             AlternativeCandidateUnknown,
			Reasons:            []AlternativeComparisonReason{},
			Improvements:       []AlternativeComparisonDimension{},
			Deteriorations:     []AlternativeComparisonDimension{},
			TargetLayoutSupply: input.Target.TargetLayoutSupply,
		}

		if candidate.TargetLayout != input.Target.TargetLayout {
			candidateResult.Status = AlternativeCandidateNotBetter
			candidateResult.Reasons = append(candidateResult.Reasons, AlternativeReasonLayoutMismatch)
			result.Candidates = append(result.Candidates, candidateResult)
			continue
		}
		if !targetReady {
			candidateResult.Reasons = append(candidateResult.Reasons, AlternativeReasonTargetEvidenceInsufficient)
			result.Candidates = append(result.Candidates, candidateResult)
			continue
		}

		dataReasons := policy.candidateDataReasons(input.Target, candidate)
		if len(dataReasons) > 0 {
			candidateResult.Reasons = append(candidateResult.Reasons, dataReasons...)
			result.Candidates = append(result.Candidates, candidateResult)
			continue
		}

		policy.evaluateCandidate(input, candidate, &candidateResult)
		result.Candidates = append(result.Candidates, candidateResult)
	}

	sortAlternativeCandidates(result.Candidates)
	result.Status = summarizeAlternativeComparison(result.Candidates)
	return result
}

func (policy AlternativeComparisonPolicy) comparableEvidenceReady(value AlternativeComparable) bool {
	if !value.HasMetric || value.AlgorithmVersion != policy.MetricAlgorithmVersion || value.CollectedAt.IsZero() {
		return false
	}
	if value.Coverage != domainneighborhood.CoverageFull || value.Freshness != domainneighborhood.FreshnessCurrent || value.QualityState != domainneighborhood.MarketQualitySufficient {
		return false
	}
	if value.TransactionEvidenceSampleCount < minimumAlternativeTransactions || !validTransactionRange(value.TransactionPriceMin, value.TransactionPriceMax) {
		return false
	}
	_, ok := alternativeSignalRank(value.Signal)
	return ok
}

func (policy AlternativeComparisonPolicy) candidateDataReasons(target, candidate AlternativeComparable) []AlternativeComparisonReason {
	reasons := make([]AlternativeComparisonReason, 0)
	if !candidate.HasMetric {
		return append(reasons, AlternativeReasonMetricMissing)
	}
	if candidate.AlgorithmVersion != policy.MetricAlgorithmVersion || candidate.AlgorithmVersion != target.AlgorithmVersion {
		reasons = append(reasons, AlternativeReasonAlgorithmVersionMismatch)
	}
	if candidate.Coverage != domainneighborhood.CoverageFull {
		reasons = append(reasons, AlternativeReasonCoverageNotFull)
	}
	if candidate.Freshness != domainneighborhood.FreshnessCurrent {
		reasons = append(reasons, AlternativeReasonMetricNotCurrent)
	}
	if candidate.QualityState != domainneighborhood.MarketQualitySufficient {
		reasons = append(reasons, AlternativeReasonMetricQualityInsufficient)
	}
	if candidate.TransactionEvidenceSampleCount < minimumAlternativeTransactions {
		reasons = append(reasons, AlternativeReasonTransactionEvidenceInsufficient)
	}
	if candidate.CollectedAt.IsZero() || durationAbs(candidate.CollectedAt.Sub(target.CollectedAt)) > alternativeComparisonWindow {
		reasons = append(reasons, AlternativeReasonComparisonWindowMismatch)
	}
	if !validTransactionRange(candidate.TransactionPriceMin, candidate.TransactionPriceMax) {
		reasons = append(reasons, AlternativeReasonTransactionPriceMissing)
	}
	if _, ok := alternativeSignalRank(candidate.Signal); !ok {
		reasons = append(reasons, AlternativeReasonSignalNotComparable)
	}
	return reasons
}

func (policy AlternativeComparisonPolicy) evaluateCandidate(input AlternativeComparisonInput, candidate AlternativeComparable, result *AlternativeCandidateResult) {
	targetMidpoint := transactionMidpoint(input.Target.TransactionPriceMin, input.Target.TransactionPriceMax)
	candidateMidpoint := transactionMidpoint(candidate.TransactionPriceMin, candidate.TransactionPriceMax)
	withinBudget := candidateMidpoint <= input.SafeTotalPrice
	priceDifference := candidateMidpoint - targetMidpoint
	priceDifferencePct := priceDifference / targetMidpoint * 100
	targetRank, _ := alternativeSignalRank(input.Target.Signal)
	candidateRank, _ := alternativeSignalRank(candidate.Signal)
	signalRankDifference := candidateRank - targetRank
	supplyDifference := candidate.TargetLayoutSupply - input.Target.TargetLayoutSupply

	result.WithinBudget = &withinBudget
	result.TargetTransactionPriceMidpoint = &targetMidpoint
	result.CandidateTransactionPriceMidpoint = &candidateMidpoint
	result.PriceDifference = &priceDifference
	result.PriceDifferencePct = &priceDifferencePct
	result.TargetSignal = neighborhoodStatusPtr(input.Target.Signal)
	result.CandidateSignal = neighborhoodStatusPtr(candidate.Signal)
	result.SignalRankDifference = &signalRankDifference
	result.CandidateTargetLayoutSupply = intPtr(candidate.TargetLayoutSupply)
	result.SupplyDifference = &supplyDifference
	if input.Target.TargetLayoutSupply > 0 {
		supplyDifferencePct := float64(supplyDifference) / float64(input.Target.TargetLayoutSupply) * 100
		result.SupplyDifferencePct = &supplyDifferencePct
	}

	if candidateMidpoint <= targetMidpoint*(1-alternativePriceThreshold) {
		result.Improvements = append(result.Improvements, AlternativeDimensionTransactionPrice)
	} else if candidateMidpoint >= targetMidpoint*(1+alternativePriceThreshold) {
		result.Deteriorations = append(result.Deteriorations, AlternativeDimensionTransactionPrice)
	}
	if signalRankDifference > 0 {
		result.Improvements = append(result.Improvements, AlternativeDimensionMarketSignal)
	} else if signalRankDifference < 0 {
		result.Deteriorations = append(result.Deteriorations, AlternativeDimensionMarketSignal)
	}
	if alternativeSupplyImproved(input.Target.TargetLayoutSupply, candidate.TargetLayoutSupply) {
		result.Improvements = append(result.Improvements, AlternativeDimensionTargetLayoutSupply)
	} else if alternativeSupplyDeteriorated(input.Target.TargetLayoutSupply, candidate.TargetLayoutSupply) {
		result.Deteriorations = append(result.Deteriorations, AlternativeDimensionTargetLayoutSupply)
	}

	if withinBudget && len(result.Improvements) >= minimumAlternativeImprovements && len(result.Deteriorations) == 0 {
		result.Status = AlternativeCandidateBetter
		result.Reasons = append(result.Reasons, AlternativeReasonBetterThresholdMet)
		return
	}

	result.Status = AlternativeCandidateNotBetter
	if !withinBudget {
		result.Reasons = append(result.Reasons, AlternativeReasonOverBudget)
	}
	if len(result.Improvements) < minimumAlternativeImprovements {
		result.Reasons = append(result.Reasons, AlternativeReasonInsufficientImprovements)
	}
	if len(result.Deteriorations) > 0 {
		result.Reasons = append(result.Reasons, AlternativeReasonDeteriorationPresent)
	}
}

func alternativeSupplyImproved(target, candidate int) bool {
	if target == 0 {
		return candidate >= alternativeSupplyAbsoluteThreshold
	}
	return candidate-target >= alternativeSupplyAbsoluteThreshold &&
		float64(candidate) >= float64(target)*(1+alternativeSupplyRelativeThreshold)
}

func alternativeSupplyDeteriorated(target, candidate int) bool {
	if target == 0 {
		return false
	}
	return target-candidate >= alternativeSupplyAbsoluteThreshold &&
		float64(candidate) <= float64(target)*(1-alternativeSupplyRelativeThreshold)
}

func alternativeSignalRank(status domainneighborhood.NeighborhoodStatus) (int, bool) {
	switch status {
	case domainneighborhood.NeighborhoodStatusBargain:
		return 5, true
	case domainneighborhood.NeighborhoodStatusFocus:
		return 4, true
	case domainneighborhood.NeighborhoodStatusObserve:
		return 3, true
	case domainneighborhood.NeighborhoodStatusNotSuggest:
		return 2, true
	case domainneighborhood.NeighborhoodStatusPriceHard:
		return 1, true
	default:
		return 0, false
	}
}

func sortAlternativeCandidates(candidates []AlternativeCandidateResult) {
	sort.SliceStable(candidates, func(i, j int) bool {
		left, right := candidates[i], candidates[j]
		if alternativeCandidateRank(left.Status) != alternativeCandidateRank(right.Status) {
			return alternativeCandidateRank(left.Status) < alternativeCandidateRank(right.Status)
		}
		if len(left.Improvements) != len(right.Improvements) {
			return len(left.Improvements) > len(right.Improvements)
		}
		if (left.CandidateTransactionPriceMidpoint == nil) != (right.CandidateTransactionPriceMidpoint == nil) {
			return left.CandidateTransactionPriceMidpoint != nil
		}
		if left.CandidateTransactionPriceMidpoint != nil && right.CandidateTransactionPriceMidpoint != nil &&
			*left.CandidateTransactionPriceMidpoint != *right.CandidateTransactionPriceMidpoint {
			return *left.CandidateTransactionPriceMidpoint < *right.CandidateTransactionPriceMidpoint
		}
		if left.Name != right.Name {
			return left.Name < right.Name
		}
		return left.NeighborhoodID < right.NeighborhoodID
	})
}

func summarizeAlternativeComparison(candidates []AlternativeCandidateResult) AlternativeComparisonStatus {
	hasEvaluable := false
	for _, candidate := range candidates {
		if candidate.Status == AlternativeCandidateBetter {
			return AlternativeComparisonBetterFound
		}
		if candidate.Status == AlternativeCandidateNotBetter {
			hasEvaluable = true
		}
	}
	if hasEvaluable {
		return AlternativeComparisonNone
	}
	return AlternativeComparisonUnknown
}

func alternativeCandidateRank(status AlternativeCandidateStatus) int {
	switch status {
	case AlternativeCandidateBetter:
		return 0
	case AlternativeCandidateNotBetter:
		return 1
	default:
		return 2
	}
}

func validTransactionRange(minimum, maximum *float64) bool {
	return minimum != nil && maximum != nil && *minimum > 0 && *maximum >= *minimum
}

func transactionMidpoint(minimum, maximum *float64) float64 {
	return (*minimum + *maximum) / 2
}

func durationAbs(value time.Duration) time.Duration {
	if value < 0 {
		return -value
	}
	return value
}

func neighborhoodStatusPtr(value domainneighborhood.NeighborhoodStatus) *domainneighborhood.NeighborhoodStatus {
	return &value
}

func intPtr(value int) *int { return &value }
