package capacity

import (
	"errors"
	"math"
	"testing"
	"time"
)

var policyReferenceDate = time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC)

func TestHousingPolicyVersionEffectiveAtUsesHalfOpenDateRange(t *testing.T) {
	policy := referencePolicy()
	to := "2027-01-01"
	policy.EffectiveTo = &to

	tests := map[string]struct {
		date string
		want bool
	}{
		"before":         {date: "2025-12-31", want: false},
		"effective from": {date: "2026-01-01", want: true},
		"last day":       {date: "2026-12-31", want: true},
		"effective to":   {date: "2027-01-01", want: false},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			date, err := time.Parse(time.DateOnly, test.date)
			if err != nil {
				t.Fatal(err)
			}
			if got := policy.EffectiveAt(date); got != test.want {
				t.Fatalf("EffectiveAt(%s) = %v, want %v", test.date, got, test.want)
			}
		})
	}
}

func TestCalculateWithPolicySelectsLoanRules(t *testing.T) {
	tests := map[string]struct {
		order        HomePurchaseOrder
		loanType     LoanType
		total        float64
		commercial   float64
		provident    float64
		wantDownRate float64
		wantRates    []float64
	}{
		"first commercial":  {HomeFirst, LoanCommercial, 400, 0, 0, 0.15, []float64{0.031}},
		"second commercial": {HomeSecond, LoanCommercial, 350, 0, 0, 0.25, []float64{0.041}},
		"first provident":   {HomeFirst, LoanProvidentFund, 400, 0, 0, 0.20, []float64{0.026}},
		"second provident":  {HomeSecond, LoanProvidentFund, 350, 0, 0, 0.30, []float64{0.03075}},
		"first combined":    {HomeFirst, LoanCombined, 410, 250, 160, 0.18, []float64{0.031, 0.026}},
		"second combined":   {HomeSecond, LoanCombined, 360, 220, 140, 0.28, []float64{0.041, 0.03075}},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			input := referencePolicyInput()
			input.TransactionScenario.HomePurchaseOrder = test.order
			input.LoanPlan.Type = test.loanType
			input.LoanPlan.TotalLoanAmount = test.total
			input.LoanPlan.CommercialLoanAmount = test.commercial
			input.LoanPlan.ProvidentFundLoanAmount = test.provident

			result := mustCalculateWithPolicy(t, input)
			if result.RecommendedDownPaymentRate != test.wantDownRate {
				t.Fatalf("RecommendedDownPaymentRate = %v, want %v", result.RecommendedDownPaymentRate, test.wantDownRate)
			}
			if result.LoanBreakdown == nil || len(result.LoanBreakdown.Components) != len(test.wantRates) {
				t.Fatalf("LoanBreakdown = %#v, want %d components", result.LoanBreakdown, len(test.wantRates))
			}
			for index, want := range test.wantRates {
				if result.LoanBreakdown.Components[index].AnnualInterestRate != want {
					t.Fatalf("component %d rate = %v, want %v", index, result.LoanBreakdown.Components[index].AnnualInterestRate, want)
				}
				if result.LoanBreakdown.Components[index].Type == LoanCommercial && result.LoanBreakdown.Components[index].SourceCode != "commercial_rate" {
					t.Fatalf("commercial source code = %q, want commercial_rate", result.LoanBreakdown.Components[index].SourceCode)
				}
			}
		})
	}
}

func TestCalculateWithPolicyUsesProvidentFiveYearBoundary(t *testing.T) {
	input := referencePolicyInput()
	input.LoanPlan.Type = LoanProvidentFund
	input.LoanPlan.TotalLoanAmount = 400
	input.LoanPlan.LoanTermMonths = 60
	result := mustCalculateWithPolicy(t, input)
	if got := result.LoanBreakdown.Components[0].AnnualInterestRate; got != 0.021 {
		t.Fatalf("60 month rate = %v, want 0.021", got)
	}

	input.LoanPlan.LoanTermMonths = 61
	result = mustCalculateWithPolicy(t, input)
	if got := result.LoanBreakdown.Components[0].AnnualInterestRate; got != 0.026 {
		t.Fatalf("61 month rate = %v, want 0.026", got)
	}
}

func TestCalculateWithPolicyCalculatesBuyerAndSellerTaxes(t *testing.T) {
	result := mustCalculateWithPolicy(t, referencePolicyInput())
	deed := taxByCode(t, result, "deed_tax")
	vat := taxByCode(t, result, "value_added_tax")
	surcharges := taxByCode(t, result, "vat_surcharges")
	income := taxByCode(t, result, "individual_income_tax")

	assertClose(t, deed.Amount, 5)
	assertClose(t, vat.Amount, 320/1.03*0.03)
	assertClose(t, surcharges.Amount, vat.Amount*0.06)
	assertClose(t, income.Amount, 24)
	assertClose(t, result.TaxBreakdown.BuyerTotal, deed.Amount)
	assertClose(t, result.TaxBreakdown.SellerTotal, vat.Amount+surcharges.Amount+income.Amount)
}

func TestCalculateWithPolicyAppliesTaxThresholdsAndBurdenMode(t *testing.T) {
	input := referencePolicyInput()
	input.TransactionScenario.HomePurchaseOrder = HomeSecond
	input.TransactionScenario.TargetHomeAreaSQM = 141
	input.TransactionScenario.OldHomeHoldingYears = 2
	input.TransactionScenario.TaxBurdenMode = TaxBurdenBuyerAll
	input.LoanPlan.TotalLoanAmount = 375
	result := mustCalculateWithPolicy(t, input)

	deed := taxByCode(t, result, "deed_tax")
	vat := taxByCode(t, result, "value_added_tax")
	if deed.Rate != 0.02 || deed.Amount != 10 {
		t.Fatalf("second-home deed tax = rate %v amount %v, want 0.02/10", deed.Rate, deed.Amount)
	}
	if !vat.Exempt || vat.Amount != 0 {
		t.Fatalf("VAT at two years = %#v, want exempt", vat)
	}
	if result.TaxBreakdown.SellerTotal != 0 || result.TaxBreakdown.BuyerTotal != result.TaxBreakdown.Total {
		t.Fatalf("buyer-all totals = %#v", result.TaxBreakdown)
	}
}

func TestCalculateWithPolicyExemptsFiveYearOnlyFamilyHomeIncomeTax(t *testing.T) {
	input := referencePolicyInput()
	input.TransactionScenario.OldHomeHoldingYears = 5
	input.TransactionScenario.OldHomeOnlyFamilyHome = true
	result := mustCalculateWithPolicy(t, input)
	income := taxByCode(t, result, "individual_income_tax")
	if !income.Exempt || income.Amount != 0 {
		t.Fatalf("income tax = %#v, want exempt", income)
	}
}

func TestCalculateWithPolicyRecordsManualOverrides(t *testing.T) {
	input := referencePolicyInput()
	downPayment := 0.20
	commercialRate := 0.049
	input.ManualOverrides = &CalculationOverrides{
		CommercialAnnualInterestRate: &commercialRate,
		DownPaymentRate:              &downPayment,
		TaxAmounts:                   map[string]float64{"deed_tax": 6},
	}
	input.LoanPlan.TotalLoanAmount = 400
	result := mustCalculateWithPolicy(t, input)

	if result.RecommendedDownPaymentRate != downPayment || !result.LoanBreakdown.Components[0].ManualOverride {
		t.Fatalf("manual loan/down-payment result = %#v", result)
	}
	if deed := taxByCode(t, result, "deed_tax"); deed.Amount != 6 || !deed.ManualOverride {
		t.Fatalf("manual deed tax = %#v", deed)
	}
	wantFields := []string{"commercialAnnualInterestRate", "downPaymentRate", "taxAmounts.deed_tax"}
	if len(result.ManualOverrides) != len(wantFields) {
		t.Fatalf("ManualOverrides = %#v", result.ManualOverrides)
	}
	for index, field := range wantFields {
		if result.ManualOverrides[index].Field != field {
			t.Fatalf("ManualOverrides[%d].Field = %q, want %q", index, result.ManualOverrides[index].Field, field)
		}
	}
}

func TestCalculateWithPolicyRejectsMismatchedCombinedLoan(t *testing.T) {
	input := referencePolicyInput()
	input.LoanPlan = &LoanPlan{
		Type: LoanCombined, TotalLoanAmount: 400, CommercialLoanAmount: 250,
		ProvidentFundLoanAmount: 100, LoanTermMonths: 360, RepaymentMethod: RepaymentEqualInstallment,
	}
	_, err := CalculateWithPolicy(input, referenceAssumptions(), referencePolicy(), policyReferenceDate)
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("CalculateWithPolicy() error = %v, want ErrInvalidInput", err)
	}
}

func referencePolicy() HousingPolicyVersion {
	return HousingPolicyVersion{
		ID: "8f297575-9418-4ea8-b901-c6467a639a01", City: "天津", Version: "tianjin-test",
		Name: "天津测试政策", EffectiveFrom: "2026-01-01", Enabled: true,
		Rules: HousingPolicyRules{
			DownPayment: DownPaymentRules{
				CommercialFirst: 0.15, CommercialSecond: 0.25,
				ProvidentFirst: 0.20, ProvidentSecond: 0.30,
				CombinedFirst: 0.18, CombinedSecond: 0.28,
			},
			Interest: InterestRateRules{
				CommercialFirst: 0.031, CommercialSecond: 0.041,
				ProvidentFirstUpToFiveYears: 0.021, ProvidentFirstOverFiveYears: 0.026,
				ProvidentSecondUpToFiveYears: 0.02525, ProvidentSecondOverFiveYears: 0.03075,
			},
			Tax: TaxRules{
				DeedFirstUpToAreaRate: 0.01, DeedFirstOverAreaRate: 0.015,
				DeedSecondUpToAreaRate: 0.01, DeedSecondOverAreaRate: 0.02,
				DeedAreaThresholdSQM: 140, VATRate: 0.03, VATExemptHoldingYears: 2,
				VATSurchargeRate: 0.06, IncomeTaxGainRate: 0.20, IncomeTaxAssessedRate: 0.01,
				IncomeTaxExemptHoldingYears: 5,
			},
		},
		Sources: []PolicySource{{
			Code: "test", Title: "测试来源", Issuer: "测试机构",
			URL: "https://example.com/policy", EffectiveDate: "2026-01-01",
		}},
	}
}

func referencePolicyInput() HousingCapacityInput {
	input := referenceInput()
	input.TargetTotalPrice = 500
	input.TransactionCosts = 0
	input.TransactionScenario = &TransactionScenario{
		City: "天津", HomePurchaseOrder: HomeFirst, TargetHomeType: TargetHomeResale,
		TargetHomeAreaSQM: 120, OldHomeHoldingYears: 1, OldHomeOriginalPrice: 200,
		TaxBurdenMode: TaxBurdenStatutory,
	}
	input.LoanPlan = &LoanPlan{
		Type: LoanCommercial, TotalLoanAmount: 400, LoanTermMonths: 360,
		RepaymentMethod: RepaymentEqualInstallment,
	}
	return input
}

func mustCalculateWithPolicy(t *testing.T, input HousingCapacityInput) HousingCapacityResult {
	t.Helper()
	result, err := CalculateWithPolicy(input, referenceAssumptions(), referencePolicy(), policyReferenceDate)
	if err != nil {
		t.Fatalf("CalculateWithPolicy() error = %v", err)
	}
	return result
}

func taxByCode(t *testing.T, result HousingCapacityResult, code string) TaxItem {
	t.Helper()
	if result.TaxBreakdown == nil {
		t.Fatal("TaxBreakdown = nil")
	}
	for _, item := range result.TaxBreakdown.Items {
		if item.Code == code {
			return item
		}
	}
	t.Fatalf("tax %q not found", code)
	return TaxItem{}
}

func assertClose(t *testing.T, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 0.00011 {
		t.Fatalf("got %v, want %v", got, want)
	}
}
