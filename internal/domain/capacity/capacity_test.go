package capacity

import (
	"errors"
	"math"
	"testing"
	"time"
)

var referenceDate = time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)

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

func referenceAssumptions() Assumptions {
	return Assumptions{
		RuleVersion:   "2026.07.14",
		EffectiveDate: "2026-07-14",
		RuleSource:    "propulse capacity rules",
		Loan: LoanParams{
			AnnualInterestRate: 0.039,
			LoanTermMonths:     360,
			RepaymentMethod:    RepaymentEqualInstallment,
		},
		LoanSource: "configured loan defaults",
		LoanOrigin: OriginConfiguredDefault,
		CityPolicy: CityPolicy{
			City:            "测试市",
			PolicyName:      "测试首付政策",
			DownPaymentRate: 0.35,
			EffectiveDate:   "2026-07-14",
			Source:          "测试政策来源",
			Origin:          OriginConfiguredDefault,
		},
		ReserveMonths: 6,
		PressureThresholds: PressureThresholds{
			SafeRatio:        0.35,
			StrainedRatio:    0.45,
			DangerRatio:      0.55,
			DangerMultiplier: 1.15,
		},
		OldHomeShareThreshold: 0.5,
	}
}

func mustCalculate(t *testing.T, input HousingCapacityInput, assumptions Assumptions) HousingCapacityResult {
	t.Helper()
	result, err := Calculate(input, assumptions, referenceDate)
	if err != nil {
		t.Fatalf("Calculate() error = %v", err)
	}
	return result
}

func TestCalculateStampsCompleteAppliedAssumptions(t *testing.T) {
	assumptions := referenceAssumptions()
	result := mustCalculate(t, referenceInput(), assumptions)

	if result.RuleVersion != assumptions.RuleVersion || result.EffectiveDate != assumptions.EffectiveDate {
		t.Fatalf("result rule metadata = %q/%q, want %q/%q", result.RuleVersion, result.EffectiveDate, assumptions.RuleVersion, assumptions.EffectiveDate)
	}
	if result.TraceabilityStatus != TraceabilityComplete {
		t.Fatalf("TraceabilityStatus = %q, want complete", result.TraceabilityStatus)
	}
	if result.AppliedAssumptions == nil || *result.AppliedAssumptions != assumptions {
		t.Fatalf("AppliedAssumptions = %#v, want %#v", result.AppliedAssumptions, assumptions)
	}
}

func TestCalculateVariesByLoanRateTermAndRepaymentMethod(t *testing.T) {
	input := referenceInput()
	base := mustCalculate(t, input, referenceAssumptions())

	higherRate := referenceAssumptions()
	higherRate.Loan.AnnualInterestRate = 0.06
	withHigherRate := mustCalculate(t, input, higherRate)
	if withHigherRate.MonthlyPayment <= base.MonthlyPayment {
		t.Fatalf("higher-rate payment = %v, want > %v", withHigherRate.MonthlyPayment, base.MonthlyPayment)
	}

	shorterTerm := referenceAssumptions()
	shorterTerm.Loan.LoanTermMonths = 240
	withShorterTerm := mustCalculate(t, input, shorterTerm)
	if withShorterTerm.MonthlyPayment <= base.MonthlyPayment {
		t.Fatalf("shorter-term payment = %v, want > %v", withShorterTerm.MonthlyPayment, base.MonthlyPayment)
	}

	equalPrincipal := referenceAssumptions()
	equalPrincipal.Loan.RepaymentMethod = RepaymentEqualPrincipal
	withEqualPrincipal := mustCalculate(t, input, equalPrincipal)
	if withEqualPrincipal.MonthlyPayment <= base.MonthlyPayment {
		t.Fatalf("equal-principal peak payment = %v, want > %v", withEqualPrincipal.MonthlyPayment, base.MonthlyPayment)
	}
}

func TestCalculateVariesByCityPolicyDownPaymentRate(t *testing.T) {
	input := referenceInput()
	input.CashOnHand = 80
	base := mustCalculate(t, input, referenceAssumptions())

	higherDownPayment := referenceAssumptions()
	higherDownPayment.CityPolicy.DownPaymentRate = 0.5
	adjusted := mustCalculate(t, input, higherDownPayment)

	if adjusted.MonthlyPayment >= base.MonthlyPayment {
		t.Fatalf("higher-down-payment monthly payment = %v, want < %v", adjusted.MonthlyPayment, base.MonthlyPayment)
	}
	if adjusted.DownPaymentGap <= base.DownPaymentGap {
		t.Fatalf("higher-down-payment gap = %v, want > %v", adjusted.DownPaymentGap, base.DownPaymentGap)
	}
}

func TestCalculateMarksChangedOverridesAsUserProvided(t *testing.T) {
	input := referenceInput()
	input.LoanOverride = &LoanParams{
		AnnualInterestRate: 0.05,
		LoanTermMonths:     240,
		RepaymentMethod:    RepaymentEqualInstallment,
	}
	input.CityPolicyOverride = &CityPolicy{
		City:            "覆盖市",
		PolicyName:      "覆盖政策",
		DownPaymentRate: 0.4,
		EffectiveDate:   "2026-07-01",
		Source:          "用户提供来源",
	}

	result := mustCalculate(t, input, referenceAssumptions())
	applied := result.AppliedAssumptions
	if applied == nil {
		t.Fatal("AppliedAssumptions = nil")
	}
	if applied.LoanOrigin != OriginUserOverride || applied.LoanSource != "user_input" {
		t.Fatalf("loan provenance = %q/%q, want user override", applied.LoanOrigin, applied.LoanSource)
	}
	if applied.CityPolicy.Origin != OriginUserOverride || applied.CityPolicy.City != "覆盖市" {
		t.Fatalf("city policy = %#v, want user override", applied.CityPolicy)
	}
}

func TestCalculatePreservesConfiguredOriginForSubmittedUnchangedDefaults(t *testing.T) {
	assumptions := referenceAssumptions()
	input := referenceInput()
	loan := assumptions.Loan
	policy := assumptions.CityPolicy
	policy.Origin = ""
	input.LoanOverride = &loan
	input.CityPolicyOverride = &policy

	result := mustCalculate(t, input, assumptions)
	applied := result.AppliedAssumptions
	if applied == nil || applied.LoanOrigin != OriginConfiguredDefault || applied.CityPolicy.Origin != OriginConfiguredDefault {
		t.Fatalf("applied provenance = %#v, want configured defaults", applied)
	}
}

func TestLoanParamsRejectInvalidValues(t *testing.T) {
	tests := map[string]LoanParams{
		"negative rate": {AnnualInterestRate: -0.01, LoanTermMonths: 360, RepaymentMethod: RepaymentEqualInstallment},
		"rate over one": {AnnualInterestRate: 1.01, LoanTermMonths: 360, RepaymentMethod: RepaymentEqualInstallment},
		"nan rate":      {AnnualInterestRate: math.NaN(), LoanTermMonths: 360, RepaymentMethod: RepaymentEqualInstallment},
		"zero term":     {AnnualInterestRate: 0.03, LoanTermMonths: 0, RepaymentMethod: RepaymentEqualInstallment},
		"long term":     {AnnualInterestRate: 0.03, LoanTermMonths: 601, RepaymentMethod: RepaymentEqualInstallment},
		"bad method":    {AnnualInterestRate: 0.03, LoanTermMonths: 360, RepaymentMethod: "balloon"},
	}
	for name, loan := range tests {
		t.Run(name, func(t *testing.T) {
			if !errors.Is(loan.Validate(), ErrInvalidInput) {
				t.Fatalf("Validate() error = %v, want ErrInvalidInput", loan.Validate())
			}
		})
	}
}

func TestCityPolicyRejectsInvalidOrFutureValues(t *testing.T) {
	valid := referenceAssumptions().CityPolicy
	tests := map[string]func(*CityPolicy){
		"missing city":   func(policy *CityPolicy) { policy.City = " " },
		"missing name":   func(policy *CityPolicy) { policy.PolicyName = "" },
		"missing source": func(policy *CityPolicy) { policy.Source = "" },
		"zero rate":      func(policy *CityPolicy) { policy.DownPaymentRate = 0 },
		"rate one":       func(policy *CityPolicy) { policy.DownPaymentRate = 1 },
		"invalid date":   func(policy *CityPolicy) { policy.EffectiveDate = "2026/07/14" },
		"future date":    func(policy *CityPolicy) { policy.EffectiveDate = "2026-07-15" },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			policy := valid
			mutate(&policy)
			if !errors.Is(policy.ValidateAt(referenceDate), ErrInvalidInput) {
				t.Fatalf("ValidateAt() error = %v, want ErrInvalidInput", policy.ValidateAt(referenceDate))
			}
		})
	}
}

func TestCityPolicyUsesTheCurrentDateInTheCallersTimeZone(t *testing.T) {
	shanghai := time.FixedZone("Asia/Shanghai", 8*60*60)
	asOf := time.Date(2026, 7, 14, 1, 0, 0, 0, shanghai)
	policy := referenceAssumptions().CityPolicy

	if err := policy.ValidateAt(asOf); err != nil {
		t.Fatalf("ValidateAt() error = %v, want policy effective on the local current date", err)
	}
}

func TestCalculateRejectsInvalidAssumptions(t *testing.T) {
	assumptions := referenceAssumptions()
	assumptions.EffectiveDate = "2026-07-15"

	_, err := Calculate(referenceInput(), assumptions, referenceDate)
	if !errors.Is(err, ErrInvalidAssumptions) {
		t.Fatalf("Calculate() error = %v, want ErrInvalidAssumptions", err)
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

	r549 := mustCalculate(t, at549, referenceAssumptions())
	r550 := mustCalculate(t, at550, referenceAssumptions())
	r551 := mustCalculate(t, at551, referenceAssumptions())
	if !(r549.DownPaymentGap <= r550.DownPaymentGap && r550.DownPaymentGap <= r551.DownPaymentGap) {
		t.Fatalf("downPaymentGap not monotonic: 549=%v 550=%v 551=%v", r549.DownPaymentGap, r550.DownPaymentGap, r551.DownPaymentGap)
	}
	if r550.SafeTotalPrice != r549.SafeTotalPrice {
		t.Fatalf("SafeTotalPrice differs across target prices: 549=%v 550=%v", r549.SafeTotalPrice, r550.SafeTotalPrice)
	}
}

func TestCalculateHousingCapacityClassifiesStrained(t *testing.T) {
	input := referenceInput()
	input.TargetTotalPrice = 500
	result := mustCalculate(t, input, referenceAssumptions())

	if result.NetOldHomeProceeds != 240 {
		t.Fatalf("NetOldHomeProceeds = %v, want 240", result.NetOldHomeProceeds)
	}
	if result.PressureLevel != PressureStrained {
		t.Fatalf("PressureLevel = %q, want strained", result.PressureLevel)
	}
	if result.SafeTotalPrice <= 500 || result.SafeTotalPrice >= result.StrainedTotalPrice {
		t.Fatalf("price bounds = safe %v strained %v", result.SafeTotalPrice, result.StrainedTotalPrice)
	}
	if result.Strategy != "先卖后买或同步推进" {
		t.Fatalf("Strategy = %q", result.Strategy)
	}
}

func TestCalculateHousingCapacityClassifiesDanger(t *testing.T) {
	input := HousingCapacityInput{
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
	}
	result := mustCalculate(t, input, referenceAssumptions())
	if result.PressureLevel != PressureDanger || result.DownPaymentGap <= 0 || result.Strategy != "暂缓改善" {
		t.Fatalf("danger result = %#v", result)
	}
}
