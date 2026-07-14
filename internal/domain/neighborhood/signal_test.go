package neighborhood

import (
	"reflect"
	"testing"
)

func TestEvaluateSignalOpensBargainingWindow(t *testing.T) {
	result := EvaluateSignal(SignalInput{
		Name:                  "青枫花园",
		ListingPriceRange:     PriceRange{Min: 520, Max: 620},
		TransactionPriceRange: PriceRange{Min: 495, Max: 545},
		ListedHomes:           42,
		ListedHomesChangePct:  18,
		PriceCutHomes:         11,
		AvgDaysOnMarket:       78,
		TransactionMomentum:   TransactionMomentumWeak,
		TargetLayoutSupply:    12,
		Quality:               sufficientQuality(),
	})

	if result.Status != NeighborhoodStatusBargain {
		t.Fatalf("Status = %q, want %q", result.Status, NeighborhoodStatusBargain)
	}
	if result.SupplyPressure != SupplyPressureHigh {
		t.Fatalf("SupplyPressure = %q, want %q", result.SupplyPressure, SupplyPressureHigh)
	}
	if result.PriceGapPct <= 0.08 {
		t.Fatalf("PriceGapPct = %v, want greater than 0.08", result.PriceGapPct)
	}
	wantReasons := []string{
		"挂牌量明显增加，买方可选择空间扩大。",
		"降价房源占比超过 20%，房东预期开始松动。",
		"成交偏弱，挂牌价缺少成交支撑。",
	}
	for _, want := range wantReasons {
		if !contains(result.Reasons, want) {
			t.Fatalf("Reasons = %#v, want %q", result.Reasons, want)
		}
	}
}

func TestEvaluateSignalKeepsPriceHard(t *testing.T) {
	result := EvaluateSignal(SignalInput{
		Name:                  "云澜府",
		ListingPriceRange:     PriceRange{Min: 700, Max: 760},
		TransactionPriceRange: PriceRange{Min: 690, Max: 745},
		ListedHomes:           14,
		ListedHomesChangePct:  -6,
		PriceCutHomes:         1,
		AvgDaysOnMarket:       35,
		TransactionMomentum:   TransactionMomentumStrong,
		TargetLayoutSupply:    3,
		Quality:               sufficientQuality(),
	})

	if result.Status != NeighborhoodStatusPriceHard {
		t.Fatalf("Status = %q, want %q", result.Status, NeighborhoodStatusPriceHard)
	}
	if result.SupplyPressure != SupplyPressureLow {
		t.Fatalf("SupplyPressure = %q, want %q", result.SupplyPressure, SupplyPressureLow)
	}
	if result.NextAction != "不要用单套挂牌价追高，等待新增供应或转向替代小区。" {
		t.Fatalf("NextAction = %q", result.NextAction)
	}
}

func TestEvaluateSignalWaitsWhenQualityCannotRecommend(t *testing.T) {
	result := EvaluateSignal(SignalInput{
		ListedHomes:         42,
		PriceCutHomes:       11,
		TransactionMomentum: TransactionMomentumWeak,
		Quality: QualityAssessment{
			Coverage:     CoveragePartial,
			Freshness:    FreshnessCurrent,
			State:        MarketQualityLowConfidence,
			CanRecommend: false,
			Warnings:     []QualityWarning{WarningPartialCoverage},
		},
	})
	if result.Status != NeighborhoodStatusInsufficientData {
		t.Fatalf("Status = %q", result.Status)
	}
	if result.QualityState != MarketQualityLowConfidence {
		t.Fatalf("QualityState = %q", result.QualityState)
	}
	if !reflect.DeepEqual(result.Warnings, []QualityWarning{WarningPartialCoverage}) {
		t.Fatalf("Warnings = %#v", result.Warnings)
	}
	if result.SupplyPressure != SupplyPressureUnknown {
		t.Fatalf("SupplyPressure = %q", result.SupplyPressure)
	}
	if result.TargetLayoutScarcity != ScarcityUnknown {
		t.Fatalf("TargetLayoutScarcity = %q", result.TargetLayoutScarcity)
	}
	if !reflect.DeepEqual(result.Reasons, []string{"市场数据覆盖、样本量或新鲜度不足，不能据此给出买入或议价结论。"}) {
		t.Fatalf("Reasons = %#v", result.Reasons)
	}
	if result.NextAction != "等待补充完整且新鲜的挂牌与成交样本，再判断看房或议价时机。" {
		t.Fatalf("NextAction = %q", result.NextAction)
	}
}

func TestEvaluateSignalFailsClosedWithoutQualityAfterTrustedMetricCutover(t *testing.T) {
	result := EvaluateSignal(SignalInput{
		Name:                  "青枫花园",
		ListingPriceRange:     PriceRange{Min: 520, Max: 620},
		TransactionPriceRange: PriceRange{Min: 495, Max: 545},
		ListedHomes:           42,
		ListedHomesChangePct:  18,
		PriceCutHomes:         11,
		AvgDaysOnMarket:       78,
		TransactionMomentum:   TransactionMomentumWeak,
		TargetLayoutSupply:    12,
	})

	if result.Status != NeighborhoodStatusInsufficientData {
		t.Fatalf("Status = %q, want %q", result.Status, NeighborhoodStatusInsufficientData)
	}
	if result.QualityState != MarketQualityInsufficientData {
		t.Fatalf("QualityState = %q, want %q", result.QualityState, MarketQualityInsufficientData)
	}
}

func TestEvaluateSignalTreatsUnknownTransactionMomentumAsInsufficient(t *testing.T) {
	result := EvaluateSignal(SignalInput{
		ListingPriceRange:     PriceRange{Min: 520, Max: 620},
		TransactionPriceRange: PriceRange{Min: 495, Max: 545},
		ListedHomes:           42,
		ListedHomesChangePct:  18,
		PriceCutHomes:         11,
		AvgDaysOnMarket:       78,
		TransactionMomentum:   TransactionMomentumUnknown,
		TargetLayoutSupply:    12,
		Quality:               sufficientQuality(),
	})

	if result.Status != NeighborhoodStatusInsufficientData || result.SupplyPressure != SupplyPressureUnknown || result.TargetLayoutScarcity != ScarcityUnknown {
		t.Fatalf("result = %#v", result)
	}
	if !containsQualityWarning(result.Warnings, WarningInsufficientTransactions) {
		t.Fatalf("warnings = %#v, want insufficient transactions", result.Warnings)
	}
}

func sufficientQuality() QualityAssessment {
	return QualityAssessment{
		Coverage:     CoverageFull,
		Freshness:    FreshnessCurrent,
		State:        MarketQualitySufficient,
		CanRecommend: true,
		Warnings:     nil,
	}
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
