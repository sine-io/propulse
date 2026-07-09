package neighborhood

import "testing"

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

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
