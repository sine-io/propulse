package capacity

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

const BudgetEstimateDisclaimer = "本结果仅供换房预算估算，不构成贷款审批、税务申报或完税承诺；实际金额以贷款机构和主管税务机关核定为准。"

type HomePurchaseOrder string

const (
	HomeFirst  HomePurchaseOrder = "first"
	HomeSecond HomePurchaseOrder = "second"
)

type TargetHomeType string

const (
	TargetHomeNew    TargetHomeType = "new"
	TargetHomeResale TargetHomeType = "resale"
)

type LoanType string

const (
	LoanCommercial    LoanType = "commercial"
	LoanProvidentFund LoanType = "provident_fund"
	LoanCombined      LoanType = "combined"
)

type TaxBurdenMode string

const (
	TaxBurdenStatutory TaxBurdenMode = "statutory"
	TaxBurdenBuyerAll  TaxBurdenMode = "buyer_all"
)

type TransactionScenario struct {
	City                  string
	HomePurchaseOrder     HomePurchaseOrder
	TargetHomeType        TargetHomeType
	TargetHomeAreaSQM     float64
	OldHomeHoldingYears   int
	OldHomeOnlyFamilyHome bool
	OldHomeOriginalPrice  float64
	TaxBurdenMode         TaxBurdenMode
}

type LoanPlan struct {
	Type                    LoanType
	TotalLoanAmount         float64
	CommercialLoanAmount    float64
	ProvidentFundLoanAmount float64
	LoanTermMonths          int
	RepaymentMethod         RepaymentMethod
}

type CalculationOverrides struct {
	CommercialAnnualInterestRate *float64
	ProvidentAnnualInterestRate  *float64
	DownPaymentRate              *float64
	TaxAmounts                   map[string]float64
}

type PolicySource struct {
	Code          string
	Title         string
	Issuer        string
	URL           string
	EffectiveDate string
}

type DownPaymentRules struct {
	CommercialFirst  float64
	CommercialSecond float64
	ProvidentFirst   float64
	ProvidentSecond  float64
	CombinedFirst    float64
	CombinedSecond   float64
}

type InterestRateRules struct {
	CommercialFirst              float64
	CommercialSecond             float64
	ProvidentFirstUpToFiveYears  float64
	ProvidentFirstOverFiveYears  float64
	ProvidentSecondUpToFiveYears float64
	ProvidentSecondOverFiveYears float64
}

type TaxRules struct {
	DeedFirstUpToAreaRate       float64
	DeedFirstOverAreaRate       float64
	DeedSecondUpToAreaRate      float64
	DeedSecondOverAreaRate      float64
	DeedAreaThresholdSQM        float64
	VATRate                     float64
	VATExemptHoldingYears       int
	VATSurchargeRate            float64
	IncomeTaxGainRate           float64
	IncomeTaxAssessedRate       float64
	IncomeTaxExemptHoldingYears int
}

type HousingPolicyRules struct {
	DownPayment DownPaymentRules
	Interest    InterestRateRules
	Tax         TaxRules
}

type HousingPolicyVersion struct {
	ID            string
	City          string
	Version       string
	Name          string
	EffectiveFrom string
	EffectiveTo   *string
	Enabled       bool
	Rules         HousingPolicyRules
	Sources       []PolicySource
	CreatedAt     time.Time
}

type PolicyVersionReference struct {
	ID            string
	City          string
	Version       string
	Name          string
	EffectiveFrom string
	EffectiveTo   *string
}

type LoanComponent struct {
	Type               LoanType
	Principal          float64
	AnnualInterestRate float64
	MonthlyPayment     float64
	SourceCode         string
	ManualOverride     bool
}

type LoanBreakdown struct {
	Type            LoanType
	TotalPrincipal  float64
	LoanTermMonths  int
	RepaymentMethod RepaymentMethod
	Components      []LoanComponent
	MonthlyPayment  float64
}

type TaxSide string

const (
	TaxSideBuyer  TaxSide = "buyer"
	TaxSideSeller TaxSide = "seller"
)

type TaxItem struct {
	Code            string
	Name            string
	StatutorySide   TaxSide
	PaidBy          TaxSide
	TaxBase         float64
	Rate            float64
	AutomaticAmount float64
	Amount          float64
	Formula         string
	Exempt          bool
	SourceCode      string
	ManualOverride  bool
}

type TaxBreakdown struct {
	Items       []TaxItem
	BuyerTotal  float64
	SellerTotal float64
	Total       float64
}

type AppliedManualOverride struct {
	Field          string
	AutomaticValue float64
	AppliedValue   float64
}

func (policy HousingPolicyVersion) Validate() error {
	if strings.TrimSpace(policy.City) == "" || strings.TrimSpace(policy.Version) == "" || strings.TrimSpace(policy.Name) == "" {
		return ErrInvalidAssumptions
	}
	from, err := time.Parse(time.DateOnly, strings.TrimSpace(policy.EffectiveFrom))
	if err != nil {
		return ErrInvalidAssumptions
	}
	if policy.EffectiveTo != nil {
		to, parseErr := time.Parse(time.DateOnly, strings.TrimSpace(*policy.EffectiveTo))
		if parseErr != nil || !to.After(from) {
			return ErrInvalidAssumptions
		}
	}
	if len(policy.Sources) == 0 {
		return ErrInvalidAssumptions
	}
	seenSources := make(map[string]struct{}, len(policy.Sources))
	for _, source := range policy.Sources {
		code := strings.TrimSpace(source.Code)
		if code == "" || strings.TrimSpace(source.Title) == "" || strings.TrimSpace(source.Issuer) == "" || strings.TrimSpace(source.URL) == "" {
			return ErrInvalidAssumptions
		}
		if _, exists := seenSources[code]; exists {
			return ErrInvalidAssumptions
		}
		seenSources[code] = struct{}{}
		if _, parseErr := time.Parse(time.DateOnly, strings.TrimSpace(source.EffectiveDate)); parseErr != nil {
			return ErrInvalidAssumptions
		}
	}
	if !policy.Rules.valid() {
		return ErrInvalidAssumptions
	}
	return nil
}

func (policy HousingPolicyVersion) EffectiveAt(asOf time.Time) bool {
	if !policy.Enabled || asOf.IsZero() || policy.Validate() != nil {
		return false
	}
	date := asOf.Format(time.DateOnly)
	if date < policy.EffectiveFrom {
		return false
	}
	return policy.EffectiveTo == nil || date < *policy.EffectiveTo
}

func (rules HousingPolicyRules) valid() bool {
	percentages := []float64{
		rules.DownPayment.CommercialFirst, rules.DownPayment.CommercialSecond,
		rules.DownPayment.ProvidentFirst, rules.DownPayment.ProvidentSecond,
		rules.DownPayment.CombinedFirst, rules.DownPayment.CombinedSecond,
		rules.Interest.CommercialFirst, rules.Interest.CommercialSecond,
		rules.Interest.ProvidentFirstUpToFiveYears, rules.Interest.ProvidentFirstOverFiveYears,
		rules.Interest.ProvidentSecondUpToFiveYears, rules.Interest.ProvidentSecondOverFiveYears,
		rules.Tax.DeedFirstUpToAreaRate, rules.Tax.DeedFirstOverAreaRate,
		rules.Tax.DeedSecondUpToAreaRate, rules.Tax.DeedSecondOverAreaRate,
		rules.Tax.VATRate, rules.Tax.VATSurchargeRate,
		rules.Tax.IncomeTaxGainRate, rules.Tax.IncomeTaxAssessedRate,
	}
	for _, value := range percentages {
		if !isFinite(value) || value < 0 || value >= 1 {
			return false
		}
	}
	return rules.DownPayment.CommercialFirst > 0 && rules.DownPayment.CommercialSecond > 0 &&
		rules.DownPayment.ProvidentFirst > 0 && rules.DownPayment.ProvidentSecond > 0 &&
		rules.DownPayment.CombinedFirst > 0 && rules.DownPayment.CombinedSecond > 0 &&
		rules.Interest.CommercialFirst > 0 && rules.Interest.CommercialSecond > 0 &&
		rules.Interest.ProvidentFirstUpToFiveYears > 0 && rules.Interest.ProvidentFirstOverFiveYears > 0 &&
		rules.Interest.ProvidentSecondUpToFiveYears > 0 && rules.Interest.ProvidentSecondOverFiveYears > 0 &&
		rules.Tax.DeedAreaThresholdSQM > 0 && rules.Tax.VATExemptHoldingYears >= 0 &&
		rules.Tax.IncomeTaxExemptHoldingYears >= 0
}

func (input HousingCapacityInput) ValidatePolicyInput(policy HousingPolicyVersion, asOf time.Time) error {
	if err := input.ValidateAt(asOf); err != nil || input.TransactionScenario == nil || input.LoanPlan == nil {
		return ErrInvalidInput
	}
	scenario := input.TransactionScenario
	if strings.TrimSpace(scenario.City) == "" || !strings.EqualFold(strings.TrimSpace(scenario.City), strings.TrimSpace(policy.City)) ||
		(scenario.HomePurchaseOrder != HomeFirst && scenario.HomePurchaseOrder != HomeSecond) ||
		(scenario.TargetHomeType != TargetHomeNew && scenario.TargetHomeType != TargetHomeResale) ||
		!isFinite(scenario.TargetHomeAreaSQM) || scenario.TargetHomeAreaSQM <= 0 ||
		scenario.OldHomeHoldingYears < 0 || scenario.OldHomeHoldingYears > 100 ||
		!isFinite(scenario.OldHomeOriginalPrice) || scenario.OldHomeOriginalPrice < 0 ||
		(scenario.TaxBurdenMode != TaxBurdenStatutory && scenario.TaxBurdenMode != TaxBurdenBuyerAll) {
		return ErrInvalidInput
	}
	plan := input.LoanPlan
	if (plan.Type != LoanCommercial && plan.Type != LoanProvidentFund && plan.Type != LoanCombined) ||
		!isFinite(plan.TotalLoanAmount) || plan.TotalLoanAmount <= 0 || plan.TotalLoanAmount >= input.TargetTotalPrice ||
		plan.LoanTermMonths < 1 || plan.LoanTermMonths > 360 ||
		(plan.RepaymentMethod != RepaymentEqualInstallment && plan.RepaymentMethod != RepaymentEqualPrincipal) {
		return ErrInvalidInput
	}
	amounts := []float64{plan.CommercialLoanAmount, plan.ProvidentFundLoanAmount}
	for _, amount := range amounts {
		if !isFinite(amount) || amount < 0 {
			return ErrInvalidInput
		}
	}
	if plan.Type == LoanCombined {
		if plan.CommercialLoanAmount <= 0 || plan.ProvidentFundLoanAmount <= 0 ||
			math.Abs(plan.CommercialLoanAmount+plan.ProvidentFundLoanAmount-plan.TotalLoanAmount) > 0.01 {
			return ErrInvalidInput
		}
	} else if plan.Type == LoanCommercial && plan.ProvidentFundLoanAmount != 0 {
		return ErrInvalidInput
	} else if plan.Type == LoanProvidentFund && plan.CommercialLoanAmount != 0 {
		return ErrInvalidInput
	}
	if overrides := input.ManualOverrides; overrides != nil {
		for _, rate := range []*float64{overrides.CommercialAnnualInterestRate, overrides.ProvidentAnnualInterestRate, overrides.DownPaymentRate} {
			if rate != nil && (!isFinite(*rate) || *rate <= 0 || *rate >= 1) {
				return ErrInvalidInput
			}
		}
		validTaxes := map[string]struct{}{"deed_tax": {}, "value_added_tax": {}, "vat_surcharges": {}, "individual_income_tax": {}}
		for code, amount := range overrides.TaxAmounts {
			if _, ok := validTaxes[code]; !ok || !isFinite(amount) || amount < 0 {
				return ErrInvalidInput
			}
		}
	}
	return nil
}

func CalculateWithPolicy(input HousingCapacityInput, assumptions Assumptions, policy HousingPolicyVersion, asOf time.Time) (HousingCapacityResult, error) {
	if !policy.EffectiveAt(asOf) {
		return HousingCapacityResult{}, ErrInvalidAssumptions
	}
	if err := input.ValidatePolicyInput(policy, asOf); err != nil {
		return HousingCapacityResult{}, err
	}
	if err := assumptions.ValidateAt(asOf); err != nil {
		return HousingCapacityResult{}, err
	}

	automaticDownRate := policy.downPaymentRate(input.TransactionScenario.HomePurchaseOrder, input.LoanPlan.Type)
	downRate := automaticDownRate
	overrides := make([]AppliedManualOverride, 0, 4)
	if input.ManualOverrides != nil && input.ManualOverrides.DownPaymentRate != nil {
		downRate = *input.ManualOverrides.DownPaymentRate
		if downRate != automaticDownRate {
			overrides = append(overrides, AppliedManualOverride{Field: "downPaymentRate", AutomaticValue: automaticDownRate, AppliedValue: downRate})
		}
	}
	maximumLoan := input.TargetTotalPrice * (1 - downRate)
	if input.LoanPlan.TotalLoanAmount-maximumLoan > 0.01 {
		return HousingCapacityResult{}, ErrInvalidInput
	}

	loan, loanOverrides := policy.loanBreakdown(*input.TransactionScenario, *input.LoanPlan, input.ManualOverrides)
	overrides = append(overrides, loanOverrides...)
	taxes := policy.taxBreakdown(input, input.ManualOverrides)
	for _, item := range taxes.Items {
		if item.ManualOverride {
			overrides = append(overrides, AppliedManualOverride{Field: "taxAmounts." + item.Code, AutomaticValue: item.AutomaticAmount, AppliedValue: item.Amount})
		}
	}
	sort.Slice(overrides, func(i, j int) bool { return overrides[i].Field < overrides[j].Field })

	buyerCosts := taxes.BuyerTotal
	sellerCosts := taxes.SellerTotal
	netOldHomeProceeds := math.Max(input.OldHomeValue-input.OldLoanBalance-sellerCosts, 0)
	reserve := input.MonthlyIncome * assumptions.ReserveMonths
	requiredCosts := input.RenovationBudget + buyerCosts + input.TransitionRentCost
	deployableCash := math.Max(input.CashOnHand+netOldHomeProceeds-requiredCosts-reserve, 0)
	coefficient := loan.MonthlyPayment / input.TargetTotalPrice
	if coefficient <= 0 {
		return HousingCapacityResult{}, ErrInvalidInput
	}

	monthlyCapacityToTotalPrice := func(ratio float64) float64 {
		availableMonthlyPayment := math.Max(math.Min(
			input.AcceptableMonthlyMortgage,
			input.MonthlyIncome*ratio-input.CurrentMonthlyMortgage,
		), 0)
		return deployableCash + availableMonthlyPayment/coefficient
	}
	thresholds := assumptions.PressureThresholds
	safeTotalPrice := round(monthlyCapacityToTotalPrice(thresholds.SafeRatio), 1)
	strainedTotalPrice := round(monthlyCapacityToTotalPrice(thresholds.StrainedRatio), 1)
	dangerTotalPrice := round(deployableCash+math.Max(
		input.MonthlyIncome*thresholds.DangerRatio-input.CurrentMonthlyMortgage,
		input.AcceptableMonthlyMortgage*thresholds.DangerMultiplier,
	)/coefficient, 1)

	actualDownPayment := input.TargetTotalPrice - input.LoanPlan.TotalLoanAmount
	requiredUpfront := actualDownPayment + requiredCosts + reserve
	downPaymentGap := round(math.Max(requiredUpfront-input.CashOnHand-netOldHomeProceeds, 0), 1)
	monthlyPayment := loan.MonthlyPayment
	monthlyPaymentRatio := round((monthlyPayment+input.CurrentMonthlyMortgage)/input.MonthlyIncome, 3)
	pressureLevel := PressureSafe
	if monthlyPaymentRatio > thresholds.StrainedRatio {
		pressureLevel = PressureDanger
	} else if monthlyPaymentRatio > thresholds.SafeRatio {
		pressureLevel = PressureStrained
	}

	oldHomeProceedsShare := netOldHomeProceeds / math.Max(input.CashOnHand+netOldHomeProceeds, 1)
	reasons := make([]string, 0, 4)
	if oldHomeProceedsShare > assumptions.OldHomeShareThreshold {
		reasons = append(reasons, "旧房净回款占首付能力比重较高，未锁定成交前不宜贸然下定。")
	}
	if downPaymentGap > 0 {
		reasons = append(reasons, "目标总价下存在首付、税费或过渡资金缺口，需要先补足安全垫。")
	}
	switch pressureLevel {
	case PressureDanger:
		reasons = append(reasons, "目标贷款方案的月供收入比超过危险线，现金流缓冲不足。")
	case PressureStrained:
		reasons = append(reasons, "目标贷款方案已高于安全月供线，适合通过砍价或降低总价回到安全区。")
	default:
		reasons = append(reasons, "目标贷款方案的月供仍在安全线内，可以继续推进看房。")
	}

	strategy := "可以同步推进"
	if pressureLevel == PressureDanger || downPaymentGap > 0 {
		strategy = "暂缓改善"
	} else if oldHomeProceedsShare > assumptions.OldHomeShareThreshold {
		strategy = "先卖后买或同步推进"
	}
	applied := assumptions
	applied.RuleVersion = policy.Version
	applied.EffectiveDate = policy.EffectiveFrom
	applied.RuleSource = policy.Name
	applied.CityPolicy = CityPolicy{
		City: policy.City, PolicyName: policy.Name, DownPaymentRate: downRate,
		EffectiveDate: policy.EffectiveFrom, Source: policy.primarySourceURL(), Origin: OriginConfiguredDefault,
	}
	if downRate != automaticDownRate {
		applied.CityPolicy.Origin = OriginUserOverride
	}
	applied.Loan = LoanParams{
		AnnualInterestRate: loan.weightedRate(), LoanTermMonths: loan.LoanTermMonths, RepaymentMethod: loan.RepaymentMethod,
	}
	applied.LoanSource = policy.primarySourceURL()
	applied.LoanOrigin = OriginConfiguredDefault
	if len(loanOverrides) > 0 {
		applied.LoanOrigin = OriginUserOverride
	}

	policyRef := PolicyVersionReference{
		ID: policy.ID, City: policy.City, Version: policy.Version, Name: policy.Name,
		EffectiveFrom: policy.EffectiveFrom, EffectiveTo: copyStringPointer(policy.EffectiveTo),
	}
	return HousingCapacityResult{
		NetOldHomeProceeds: round(netOldHomeProceeds, 1), DeployableCash: round(deployableCash, 1),
		SafeTotalPrice: safeTotalPrice, StrainedTotalPrice: strainedTotalPrice, DangerTotalPrice: dangerTotalPrice,
		DownPaymentGap: downPaymentGap, MonthlyPayment: monthlyPayment, MonthlyPaymentRatio: monthlyPaymentRatio,
		PressureLevel:               pressureLevel,
		MinimumSafeOldHomeSalePrice: round(input.OldLoanBalance+sellerCosts+math.Max(requiredUpfront-input.CashOnHand, 0), 1),
		Strategy:                    strategy, Reasons: reasons, RuleVersion: policy.Version, EffectiveDate: policy.EffectiveFrom,
		TraceabilityStatus: TraceabilityComplete, AppliedAssumptions: &applied,
		RecommendedDownPaymentRate: downRate, RecommendedDownPayment: round(input.TargetTotalPrice*downRate, 2),
		LoanBreakdown: &loan, TaxBreakdown: &taxes, PolicyVersion: &policyRef,
		Sources: append([]PolicySource(nil), policy.Sources...), ManualOverrides: overrides,
		Disclaimer: BudgetEstimateDisclaimer,
	}, nil
}

func (policy HousingPolicyVersion) downPaymentRate(order HomePurchaseOrder, loanType LoanType) float64 {
	rules := policy.Rules.DownPayment
	if order == HomeSecond {
		switch loanType {
		case LoanProvidentFund:
			return rules.ProvidentSecond
		case LoanCombined:
			return rules.CombinedSecond
		default:
			return rules.CommercialSecond
		}
	}
	switch loanType {
	case LoanProvidentFund:
		return rules.ProvidentFirst
	case LoanCombined:
		return rules.CombinedFirst
	default:
		return rules.CommercialFirst
	}
}

func (policy HousingPolicyVersion) loanBreakdown(scenario TransactionScenario, plan LoanPlan, manual *CalculationOverrides) (LoanBreakdown, []AppliedManualOverride) {
	commercialRate := policy.Rules.Interest.CommercialFirst
	if scenario.HomePurchaseOrder == HomeSecond {
		commercialRate = policy.Rules.Interest.CommercialSecond
	}
	providentRate := policy.Rules.Interest.ProvidentFirstOverFiveYears
	if plan.LoanTermMonths <= 60 {
		providentRate = policy.Rules.Interest.ProvidentFirstUpToFiveYears
	}
	if scenario.HomePurchaseOrder == HomeSecond {
		providentRate = policy.Rules.Interest.ProvidentSecondOverFiveYears
		if plan.LoanTermMonths <= 60 {
			providentRate = policy.Rules.Interest.ProvidentSecondUpToFiveYears
		}
	}
	automaticCommercial, automaticProvident := commercialRate, providentRate
	commercialOverridden, providentOverridden := false, false
	overrides := make([]AppliedManualOverride, 0, 2)
	if manual != nil && manual.CommercialAnnualInterestRate != nil {
		commercialRate = *manual.CommercialAnnualInterestRate
		commercialOverridden = commercialRate != automaticCommercial
		if commercialOverridden {
			overrides = append(overrides, AppliedManualOverride{Field: "commercialAnnualInterestRate", AutomaticValue: automaticCommercial, AppliedValue: commercialRate})
		}
	}
	if manual != nil && manual.ProvidentAnnualInterestRate != nil {
		providentRate = *manual.ProvidentAnnualInterestRate
		providentOverridden = providentRate != automaticProvident
		if providentOverridden {
			overrides = append(overrides, AppliedManualOverride{Field: "providentAnnualInterestRate", AutomaticValue: automaticProvident, AppliedValue: providentRate})
		}
	}

	commercialPrincipal := plan.CommercialLoanAmount
	providentPrincipal := plan.ProvidentFundLoanAmount
	if plan.Type == LoanCommercial {
		commercialPrincipal = plan.TotalLoanAmount
	}
	if plan.Type == LoanProvidentFund {
		providentPrincipal = plan.TotalLoanAmount
	}
	components := make([]LoanComponent, 0, 2)
	if commercialPrincipal > 0 {
		payment := round(commercialPrincipal*(LoanParams{AnnualInterestRate: commercialRate, LoanTermMonths: plan.LoanTermMonths, RepaymentMethod: plan.RepaymentMethod}).perPrincipalMonthlyFactor(), 4)
		components = append(components, LoanComponent{Type: LoanCommercial, Principal: round(commercialPrincipal, 2), AnnualInterestRate: commercialRate, MonthlyPayment: payment, SourceCode: "commercial_rate", ManualOverride: commercialOverridden})
	}
	if providentPrincipal > 0 {
		payment := round(providentPrincipal*(LoanParams{AnnualInterestRate: providentRate, LoanTermMonths: plan.LoanTermMonths, RepaymentMethod: plan.RepaymentMethod}).perPrincipalMonthlyFactor(), 4)
		components = append(components, LoanComponent{Type: LoanProvidentFund, Principal: round(providentPrincipal, 2), AnnualInterestRate: providentRate, MonthlyPayment: payment, SourceCode: "provident_fund", ManualOverride: providentOverridden})
	}
	monthly := 0.0
	for _, component := range components {
		monthly += component.MonthlyPayment
	}
	return LoanBreakdown{Type: plan.Type, TotalPrincipal: round(plan.TotalLoanAmount, 2), LoanTermMonths: plan.LoanTermMonths, RepaymentMethod: plan.RepaymentMethod, Components: components, MonthlyPayment: round(monthly, 4)}, overrides
}

func (policy HousingPolicyVersion) taxBreakdown(input HousingCapacityInput, manual *CalculationOverrides) TaxBreakdown {
	scenario := *input.TransactionScenario
	rules := policy.Rules.Tax
	deedRate := rules.DeedFirstOverAreaRate
	if scenario.HomePurchaseOrder == HomeFirst && scenario.TargetHomeAreaSQM <= rules.DeedAreaThresholdSQM {
		deedRate = rules.DeedFirstUpToAreaRate
	} else if scenario.HomePurchaseOrder == HomeSecond && scenario.TargetHomeAreaSQM <= rules.DeedAreaThresholdSQM {
		deedRate = rules.DeedSecondUpToAreaRate
	} else if scenario.HomePurchaseOrder == HomeSecond {
		deedRate = rules.DeedSecondOverAreaRate
	}
	items := []TaxItem{{
		Code: "deed_tax", Name: "契税", StatutorySide: TaxSideBuyer, PaidBy: TaxSideBuyer,
		TaxBase: input.TargetTotalPrice, Rate: deedRate, AutomaticAmount: input.TargetTotalPrice * deedRate,
		Formula: fmt.Sprintf("目标房总价 x %.2f%%", deedRate*100), SourceCode: "deed_tax",
	}}

	vatAmount := 0.0
	vatExempt := input.OldHomeValue <= 0 || scenario.OldHomeHoldingYears >= rules.VATExemptHoldingYears
	if !vatExempt {
		vatAmount = input.OldHomeValue / (1 + rules.VATRate) * rules.VATRate
	}
	items = append(items, TaxItem{
		Code: "value_added_tax", Name: "增值税", StatutorySide: TaxSideSeller, PaidBy: TaxSideSeller,
		TaxBase: input.OldHomeValue, Rate: rules.VATRate, AutomaticAmount: vatAmount,
		Formula: fmt.Sprintf("旧房售价/(1+%.2f%%) x %.2f%%；持有满%d年免征", rules.VATRate*100, rules.VATRate*100, rules.VATExemptHoldingYears),
		Exempt:  vatExempt, SourceCode: "housing_vat",
	})
	surcharge := vatAmount * rules.VATSurchargeRate
	items = append(items, TaxItem{
		Code: "vat_surcharges", Name: "增值税附加", StatutorySide: TaxSideSeller, PaidBy: TaxSideSeller,
		TaxBase: vatAmount, Rate: rules.VATSurchargeRate, AutomaticAmount: surcharge,
		Formula: fmt.Sprintf("增值税 x %.2f%%", rules.VATSurchargeRate*100), Exempt: vatAmount == 0, SourceCode: "tax_surcharges",
	})

	incomeTax := 0.0
	incomeExempt := input.OldHomeValue <= 0 || (scenario.OldHomeHoldingYears >= rules.IncomeTaxExemptHoldingYears && scenario.OldHomeOnlyFamilyHome)
	incomeFormula := fmt.Sprintf("旧房售价 x %.2f%%（无法提供原值时的预算口径）", rules.IncomeTaxAssessedRate*100)
	incomeBase, incomeRate := input.OldHomeValue, rules.IncomeTaxAssessedRate
	if !incomeExempt && scenario.OldHomeOriginalPrice > 0 {
		incomeBase = math.Max(input.OldHomeValue-scenario.OldHomeOriginalPrice, 0)
		incomeRate = rules.IncomeTaxGainRate
		incomeFormula = fmt.Sprintf("旧房售价与原购入价正差额 x %.2f%%（未计可扣除费用）", rules.IncomeTaxGainRate*100)
	}
	if !incomeExempt {
		incomeTax = incomeBase * incomeRate
	}
	items = append(items, TaxItem{
		Code: "individual_income_tax", Name: "个人所得税", StatutorySide: TaxSideSeller, PaidBy: TaxSideSeller,
		TaxBase: incomeBase, Rate: incomeRate, AutomaticAmount: incomeTax, Formula: incomeFormula,
		Exempt: incomeExempt, SourceCode: "individual_income_tax",
	})

	buyerAll := scenario.TaxBurdenMode == TaxBurdenBuyerAll
	for i := range items {
		items[i].AutomaticAmount = round(items[i].AutomaticAmount, 4)
		items[i].Amount = items[i].AutomaticAmount
		if buyerAll {
			items[i].PaidBy = TaxSideBuyer
		}
		if manual != nil {
			if amount, ok := manual.TaxAmounts[items[i].Code]; ok {
				items[i].Amount = round(amount, 4)
				items[i].ManualOverride = items[i].Amount != items[i].AutomaticAmount
				items[i].Exempt = items[i].Amount == 0
			}
		}
	}
	result := TaxBreakdown{Items: items}
	for _, item := range items {
		if item.PaidBy == TaxSideBuyer {
			result.BuyerTotal += item.Amount
		} else {
			result.SellerTotal += item.Amount
		}
		result.Total += item.Amount
	}
	result.BuyerTotal = round(result.BuyerTotal, 4)
	result.SellerTotal = round(result.SellerTotal, 4)
	result.Total = round(result.Total, 4)
	return result
}

func (policy HousingPolicyVersion) primarySourceURL() string {
	if len(policy.Sources) == 0 {
		return "policy_repository"
	}
	return policy.Sources[0].URL
}

func (loan LoanBreakdown) weightedRate() float64 {
	if loan.TotalPrincipal <= 0 {
		return 0
	}
	weighted := 0.0
	for _, component := range loan.Components {
		weighted += component.Principal * component.AnnualInterestRate
	}
	return weighted / loan.TotalPrincipal
}

func copyStringPointer(value *string) *string {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}
