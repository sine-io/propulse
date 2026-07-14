package neighborhood

import "math"

type MetricComparisonStatus string

const (
	MetricComparisonAvailable   MetricComparisonStatus = "available"
	MetricComparisonUnavailable MetricComparisonStatus = "unavailable"
)

type MetricComparisonReason string

const (
	ComparisonReasonCurrentPartialCoverage     MetricComparisonReason = "current_partial_coverage"
	ComparisonReasonFullBaselineNotFound       MetricComparisonReason = "full_baseline_not_found"
	ComparisonReasonTransactionEvidenceMissing MetricComparisonReason = "transaction_evidence_missing"
)

type PercentageChangeStatus string

const (
	PercentageChangeAvailable    PercentageChangeStatus = "available"
	PercentageChangeZeroBaseline PercentageChangeStatus = "zero_baseline"
)

type MetricChangeValue struct {
	Current          int
	Baseline         int
	AbsoluteChange   int
	PercentageChange *float64
	PercentageStatus PercentageChangeStatus
}

func CalculateMetricChange(current, baseline int) MetricChangeValue {
	change := MetricChangeValue{
		Current:        current,
		Baseline:       baseline,
		AbsoluteChange: current - baseline,
	}
	if baseline == 0 {
		change.PercentageStatus = PercentageChangeZeroBaseline
		return change
	}
	percentage := math.Round((float64(change.AbsoluteChange)/float64(baseline))*10000) / 100
	change.PercentageChange = &percentage
	change.PercentageStatus = PercentageChangeAvailable
	return change
}
