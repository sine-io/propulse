package capacity

import "testing"

func TestCalculateHousingCapacityClassifiesStrained(t *testing.T) {
	result := Calculate(HousingCapacityInput{
		CashOnHand:                150,
		OldHomeValue:              320,
		OldLoanBalance:            80,
		MonthlyIncome:             3.5,
		CurrentMonthlyMortgage:    0,
		AcceptableMonthlyMortgage: 1.5,
		TargetTotalPrice:          550,
		RenovationBudget:          40,
		TransactionCosts:          18,
		TransitionRentCost:        5,
	})

	if result.NetOldHomeProceeds != 240 {
		t.Fatalf("NetOldHomeProceeds = %v, want 240", result.NetOldHomeProceeds)
	}
	if result.PressureLevel != PressureStrained {
		t.Fatalf("PressureLevel = %q, want %q", result.PressureLevel, PressureStrained)
	}
	if result.SafeTotalPrice <= 500 {
		t.Fatalf("SafeTotalPrice = %v, want > 500", result.SafeTotalPrice)
	}
	if result.SafeTotalPrice >= result.StrainedTotalPrice {
		t.Fatalf("SafeTotalPrice = %v, StrainedTotalPrice = %v, want safe < strained", result.SafeTotalPrice, result.StrainedTotalPrice)
	}
	if result.DangerTotalPrice <= result.StrainedTotalPrice {
		t.Fatalf("DangerTotalPrice = %v, StrainedTotalPrice = %v, want danger > strained", result.DangerTotalPrice, result.StrainedTotalPrice)
	}
	if result.MonthlyPaymentRatio <= 0.35 || result.MonthlyPaymentRatio > 0.45 {
		t.Fatalf("MonthlyPaymentRatio = %v, want 0.35 < ratio <= 0.45", result.MonthlyPaymentRatio)
	}
	if result.Strategy != "先卖后买或同步推进" {
		t.Fatalf("Strategy = %q", result.Strategy)
	}
	if len(result.Reasons) == 0 || result.Reasons[0] != "旧房净回款占首付能力比重较高，未锁定成交前不宜贸然下定。" {
		t.Fatalf("Reasons = %#v", result.Reasons)
	}
}

func TestCalculateHousingCapacityClassifiesDanger(t *testing.T) {
	result := Calculate(HousingCapacityInput{
		CashOnHand:                80,
		OldHomeValue:              260,
		OldLoanBalance:            140,
		MonthlyIncome:             2.4,
		CurrentMonthlyMortgage:    0.35,
		AcceptableMonthlyMortgage: 1.4,
		TargetTotalPrice:          650,
		RenovationBudget:          35,
		TransactionCosts:          22,
		TransitionRentCost:        8,
	})

	if result.PressureLevel != PressureDanger {
		t.Fatalf("PressureLevel = %q, want %q", result.PressureLevel, PressureDanger)
	}
	if result.DownPaymentGap <= 0 {
		t.Fatalf("DownPaymentGap = %v, want > 0", result.DownPaymentGap)
	}
	if result.Strategy != "暂缓改善" {
		t.Fatalf("Strategy = %q, want 暂缓改善", result.Strategy)
	}
	found := false
	for _, reason := range result.Reasons {
		if reason == "目标总价对应的月供收入比超过危险线，现金流缓冲不足。" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("Reasons = %#v, want danger reason", result.Reasons)
	}
}
