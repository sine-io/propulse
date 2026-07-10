package neighborhood

import (
	"reflect"
	"testing"
	"time"
)

func TestAssessQualityFreshnessBoundaries(t *testing.T) {
	now := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name string
		age  time.Duration
		want Freshness
	}{
		{name: "seven days is current", age: 7 * 24 * time.Hour, want: FreshnessCurrent},
		{name: "after seven days is stale", age: 7*24*time.Hour + time.Second, want: FreshnessStale},
		{name: "thirty days is stale", age: 30 * 24 * time.Hour, want: FreshnessStale},
		{name: "after thirty days is expired", age: 30*24*time.Hour + time.Second, want: FreshnessExpired},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			collectedAt := now.Add(-tt.age)
			got := AssessQuality(QualityInput{
				Now:                    now,
				InventoryCollectedAt:   &collectedAt,
				LatestCoverage:         CoverageFull,
				HasFullInventory:       true,
				ListingSampleCount:     5,
				TransactionSampleCount: 3,
			})
			if got.Freshness != tt.want {
				t.Fatalf("Freshness = %q, want %q", got.Freshness, tt.want)
			}
		})
	}
}

func TestAssessQualityAllowsFullCurrentMinimumSamples(t *testing.T) {
	got := AssessQuality(qualityInputWithDefaults(func(input *QualityInput) {}))

	if got.State != MarketQualitySufficient {
		t.Fatalf("State = %q, want %q", got.State, MarketQualitySufficient)
	}
	if !got.CanRecommend {
		t.Fatalf("CanRecommend = false, want true")
	}
	if len(got.Warnings) != 0 {
		t.Fatalf("Warnings = %#v, want empty", got.Warnings)
	}
}

func TestAssessQualityRejectsUnknownCoverage(t *testing.T) {
	got := AssessQuality(qualityInputWithDefaults(func(input *QualityInput) {
		input.LatestCoverage = CoverageUnknown
	}))

	assertQualityBlocked(t, got, MarketQualityLowConfidence, []QualityWarning{WarningPartialCoverage})
}

func TestAssessQualityRejectsPartialCoverage(t *testing.T) {
	got := AssessQuality(qualityInputWithDefaults(func(input *QualityInput) {
		input.LatestCoverage = CoveragePartial
	}))

	assertQualityBlocked(t, got, MarketQualityLowConfidence, []QualityWarning{WarningPartialCoverage})
}

func TestAssessQualityDowngradesPendingMetricRefresh(t *testing.T) {
	got := AssessQuality(qualityInputWithDefaults(func(input *QualityInput) {
		input.HasNewerUncalculatedRun = true
	}))

	assertQualityBlocked(t, got, MarketQualityLowConfidence, []QualityWarning{WarningMetricRefreshPending})
}

func TestAssessQualityRequiresFullInventory(t *testing.T) {
	got := AssessQuality(qualityInputWithDefaults(func(input *QualityInput) {
		input.HasFullInventory = false
		input.InventoryCollectedAt = nil
	}))

	assertQualityBlocked(t, got, MarketQualityInsufficientData, []QualityWarning{WarningNoFullInventory})
	if got.Freshness != FreshnessUnknown {
		t.Fatalf("Freshness = %q, want %q", got.Freshness, FreshnessUnknown)
	}
}

func TestAssessQualityRejectsUnknownFreshness(t *testing.T) {
	got := AssessQuality(qualityInputWithDefaults(func(input *QualityInput) {
		input.InventoryCollectedAt = nil
	}))

	assertQualityBlocked(t, got, MarketQualityLowConfidence, []QualityWarning{WarningStaleData})
	if got.Freshness != FreshnessUnknown {
		t.Fatalf("Freshness = %q, want %q", got.Freshness, FreshnessUnknown)
	}
}

func TestAssessQualityRequiresFiveListings(t *testing.T) {
	tests := []struct {
		name  string
		count int
		state MarketQualityState
	}{
		{name: "zero listings is insufficient data", count: 0, state: MarketQualityInsufficientData},
		{name: "fewer than five listings is low confidence", count: 4, state: MarketQualityLowConfidence},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AssessQuality(qualityInputWithDefaults(func(input *QualityInput) {
				input.ListingSampleCount = tt.count
			}))

			assertQualityBlocked(t, got, tt.state, []QualityWarning{WarningInsufficientListings})
		})
	}
}

func TestAssessQualityRequiresThreeTransactions(t *testing.T) {
	tests := []struct {
		name  string
		count int
		state MarketQualityState
	}{
		{name: "zero transactions is insufficient data", count: 0, state: MarketQualityInsufficientData},
		{name: "fewer than three transactions is low confidence", count: 2, state: MarketQualityLowConfidence},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AssessQuality(qualityInputWithDefaults(func(input *QualityInput) {
				input.TransactionSampleCount = tt.count
			}))

			assertQualityBlocked(t, got, tt.state, []QualityWarning{WarningInsufficientTransactions})
		})
	}
}

func TestAssessQualityOrdersWarningsDeterministically(t *testing.T) {
	got := AssessQuality(qualityInputWithDefaults(func(input *QualityInput) {
		input.LatestCoverage = CoveragePartial
		input.HasNewerUncalculatedRun = true
		input.HasFullInventory = false
		input.InventoryCollectedAt = nil
		input.ListingSampleCount = 4
		input.TransactionSampleCount = 2
	}))

	want := []QualityWarning{
		WarningPartialCoverage,
		WarningMetricRefreshPending,
		WarningNoFullInventory,
		WarningInsufficientListings,
		WarningInsufficientTransactions,
	}
	assertQualityBlocked(t, got, MarketQualityInsufficientData, want)
}

func qualityInputWithDefaults(mutate func(*QualityInput)) QualityInput {
	now := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	collectedAt := now.Add(-24 * time.Hour)
	input := QualityInput{
		Now:                    now,
		InventoryCollectedAt:   &collectedAt,
		LatestCoverage:         CoverageFull,
		HasFullInventory:       true,
		ListingSampleCount:     5,
		TransactionSampleCount: 3,
	}
	mutate(&input)
	return input
}

func assertQualityBlocked(t *testing.T, got QualityAssessment, wantState MarketQualityState, wantWarnings []QualityWarning) {
	t.Helper()
	if got.CanRecommend {
		t.Fatalf("CanRecommend = true, want false")
	}
	if got.State != wantState {
		t.Fatalf("State = %q, want %q", got.State, wantState)
	}
	if !reflect.DeepEqual(got.Warnings, wantWarnings) {
		t.Fatalf("Warnings = %#v, want %#v", got.Warnings, wantWarnings)
	}
}
