package capacity

import "testing"

func referenceInput() HousingCapacityInput {
	return HousingCapacityInput{
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
	}
}

func TestCalculateStampsDefaultRuleVersion(t *testing.T) {
	result := Calculate(referenceInput())

	defaults := DefaultAssumptions()
	if result.RuleVersion != defaults.RuleVersion || result.RuleVersion == "" {
		t.Fatalf("RuleVersion = %q, want %q", result.RuleVersion, defaults.RuleVersion)
	}
	if result.EffectiveDate != defaults.EffectiveDate || result.EffectiveDate == "" {
		t.Fatalf("EffectiveDate = %q, want %q", result.EffectiveDate, defaults.EffectiveDate)
	}
}

func TestCalculateWithVariesByAssumptions(t *testing.T) {
	input := referenceInput()
	base := Calculate(input)

	custom := DefaultAssumptions()
	custom.RuleVersion = "test.1"
	custom.Loan.AnnualInterestRate = DefaultAssumptions().Loan.AnnualInterestRate + 0.02 // 提高利率应抬高月供
	adjusted := CalculateWith(input, custom)

	if adjusted.RuleVersion != "test.1" {
		t.Fatalf("RuleVersion = %q, want test.1", adjusted.RuleVersion)
	}
	if !(adjusted.MonthlyPayment > base.MonthlyPayment) {
		t.Fatalf("MonthlyPayment = %v, want > baseline %v after raising rate", adjusted.MonthlyPayment, base.MonthlyPayment)
	}
}

func TestCalculateHonorsLoanOverride(t *testing.T) {
	input := referenceInput()
	base := Calculate(input)

	// 用户把利率调高、期限缩短，月供应上升（#67 LoanOverride 生效）。
	input.LoanOverride = &LoanParams{
		AnnualInterestRate: 0.06,
		LoanTermMonths:     240,
		RepaymentMethod:    RepaymentEqualInstallment,
	}
	overridden := CalculateWith(input, DefaultAssumptions())

	if !(overridden.MonthlyPayment > base.MonthlyPayment) {
		t.Fatalf("MonthlyPayment = %v, want > baseline %v with higher rate + shorter term", overridden.MonthlyPayment, base.MonthlyPayment)
	}
}

func TestCalculateHasNo550SpecialPath(t *testing.T) {
	base := referenceInput()

	at549 := base
	at549.TargetTotalPrice = 549
	at550 := base
	at550.TargetTotalPrice = 550
	at551 := base
	at551.TargetTotalPrice = 551

	r549 := Calculate(at549)
	r550 := Calculate(at550)
	r551 := Calculate(at551)

	// 550 不得触发固定覆盖：三者的首付缺口应随总价单调、连续变化。
	if !(r549.DownPaymentGap <= r550.DownPaymentGap && r550.DownPaymentGap <= r551.DownPaymentGap) {
		t.Fatalf("downPaymentGap not monotonic: 549=%v 550=%v 551=%v", r549.DownPaymentGap, r550.DownPaymentGap, r551.DownPaymentGap)
	}
	if r550.SafeTotalPrice != r549.SafeTotalPrice {
		// SafeTotalPrice 不依赖 targetTotalPrice，应一致（无 550 专属分支）。
		t.Fatalf("SafeTotalPrice differs across target prices: 549=%v 550=%v", r549.SafeTotalPrice, r550.SafeTotalPrice)
	}
}

func TestCalculateHousingCapacityClassifiesStrained(t *testing.T) {
	result := Calculate(HousingCapacityInput{
		CashOnHand:                150,
		OldHomeValue:              320,
		OldLoanBalance:            80,
		MonthlyIncome:             3.5,
		CurrentMonthlyMortgage:    0,
		AcceptableMonthlyMortgage: 1.5,
		TargetTotalPrice:          500,
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
