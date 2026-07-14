package neighborhood

import "time"

type Coverage string

const (
	CoverageUnknown Coverage = "unknown"
	CoverageFull    Coverage = "full"
	CoveragePartial Coverage = "partial"
)

type Freshness string

const (
	FreshnessUnknown Freshness = "unknown"
	FreshnessCurrent Freshness = "current"
	FreshnessStale   Freshness = "stale"
	FreshnessExpired Freshness = "expired"
)

type MarketQualityState string

const (
	MarketQualitySufficient       MarketQualityState = "sufficient"
	MarketQualityLowConfidence    MarketQualityState = "low_confidence"
	MarketQualityInsufficientData MarketQualityState = "insufficient_data"
)

type QualityWarning string

const (
	WarningPartialCoverage          QualityWarning = "partial_coverage"
	WarningNoFullInventory          QualityWarning = "no_full_inventory"
	WarningStaleData                QualityWarning = "stale_data"
	WarningExpiredData              QualityWarning = "expired_data"
	WarningInsufficientListings     QualityWarning = "insufficient_listing_samples"
	WarningInsufficientTransactions QualityWarning = "insufficient_transaction_samples"
	WarningMetricRefreshPending     QualityWarning = "metric_refresh_pending"
	WarningMetricUnavailable        QualityWarning = "metric_unavailable"
)

type QualityInput struct {
	Now                     time.Time
	InventoryCollectedAt    *time.Time
	LatestCoverage          Coverage
	HasFullInventory        bool
	ListingSampleCount      int
	TransactionSampleCount  int
	HasNewerUncalculatedRun bool
}

type QualityAssessment struct {
	Coverage     Coverage
	Freshness    Freshness
	State        MarketQualityState
	CanRecommend bool
	Warnings     []QualityWarning
}

func AssessQuality(input QualityInput) QualityAssessment {
	coverage := input.LatestCoverage
	if coverage == "" {
		coverage = CoverageUnknown
	}

	freshness := assessFreshness(input.Now, input.InventoryCollectedAt)

	warnings := make([]QualityWarning, 0)
	if coverage != CoverageFull {
		warnings = append(warnings, WarningPartialCoverage)
	}
	if input.HasNewerUncalculatedRun {
		warnings = append(warnings, WarningMetricRefreshPending)
	}
	if !input.HasFullInventory {
		warnings = append(warnings, WarningNoFullInventory)
	}
	if freshness == FreshnessExpired {
		warnings = append(warnings, WarningExpiredData)
	} else if freshness == FreshnessStale {
		warnings = append(warnings, WarningStaleData)
	} else if freshness == FreshnessUnknown && input.HasFullInventory {
		warnings = append(warnings, WarningStaleData)
	}
	if input.ListingSampleCount < 5 {
		warnings = append(warnings, WarningInsufficientListings)
	}
	if input.TransactionSampleCount < 3 {
		warnings = append(warnings, WarningInsufficientTransactions)
	}

	state := MarketQualitySufficient
	canRecommend := true
	if len(warnings) > 0 {
		state = MarketQualityLowConfidence
		canRecommend = false
	}
	if !input.HasFullInventory || input.ListingSampleCount == 0 || input.TransactionSampleCount == 0 {
		state = MarketQualityInsufficientData
		canRecommend = false
	}

	return QualityAssessment{
		Coverage:     coverage,
		Freshness:    freshness,
		State:        state,
		CanRecommend: canRecommend,
		Warnings:     warnings,
	}
}

func assessFreshness(now time.Time, inventoryCollectedAt *time.Time) Freshness {
	if inventoryCollectedAt == nil {
		return FreshnessUnknown
	}

	age := now.Sub(*inventoryCollectedAt)
	if age <= 7*24*time.Hour {
		return FreshnessCurrent
	}
	if age <= 30*24*time.Hour {
		return FreshnessStale
	}
	return FreshnessExpired
}
