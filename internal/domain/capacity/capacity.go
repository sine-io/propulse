package capacity

import (
	"errors"
	"math"
	"strings"
	"time"
)

var (
	ErrInvalidInput       = errors.New("invalid housing capacity input")
	ErrInvalidAssumptions = errors.New("invalid housing capacity assumptions")
)

type PressureLevel string

const (
	PressureSafe     PressureLevel = "safe"
	PressureStrained PressureLevel = "strained"
	PressureDanger   PressureLevel = "danger"
)

type TraceabilityStatus string

const (
	TraceabilityComplete          TraceabilityStatus = "complete"
	TraceabilityLegacyUnversioned TraceabilityStatus = "legacy_unversioned"
)

type AssumptionOrigin string

const (
	OriginConfiguredDefault AssumptionOrigin = "configured_default"
	OriginUserOverride      AssumptionOrigin = "user_override"
)

type RepaymentMethod string

const (
	RepaymentEqualInstallment RepaymentMethod = "equal_installment"
	RepaymentEqualPrincipal   RepaymentMethod = "equal_principal"
)

type LoanParams struct {
	AnnualInterestRate float64
	LoanTermMonths     int
	RepaymentMethod    RepaymentMethod
}

type CityPolicy struct {
	City            string
	PolicyName      string
	DownPaymentRate float64
	EffectiveDate   string
	Source          string
	Origin          AssumptionOrigin
}

type PressureThresholds struct {
	SafeRatio        float64
	StrainedRatio    float64
	DangerRatio      float64
	DangerMultiplier float64
}

// Assumptions is the complete, reproducible rule set injected into a
// calculation. Runtime defaults are assembled outside the domain layer.
type Assumptions struct {
	RuleVersion           string
	EffectiveDate         string
	RuleSource            string
	Loan                  LoanParams
	LoanSource            string
	LoanOrigin            AssumptionOrigin
	CityPolicy            CityPolicy
	ReserveMonths         float64
	PressureThresholds    PressureThresholds
	OldHomeShareThreshold float64
}

type HousingCapacityInput struct {
	CashOnHand                float64
	OldHomeValue              float64
	OldLoanBalance            float64
	MonthlyIncome             float64
	CurrentMonthlyMortgage    float64
	AcceptableMonthlyMortgage float64
	TargetTotalPrice          float64
	RenovationBudget          float64
	TransactionCosts          float64
	TransitionRentCost        float64
	TransactionScenario       *TransactionScenario
	LoanPlan                  *LoanPlan
	ManualOverrides           *CalculationOverrides
	LoanOverride              *LoanParams
	CityPolicyOverride        *CityPolicy
}

type HousingCapacityResult struct {
	NetOldHomeProceeds          float64
	DeployableCash              float64
	SafeTotalPrice              float64
	StrainedTotalPrice          float64
	DangerTotalPrice            float64
	DownPaymentGap              float64
	MonthlyPayment              float64
	MonthlyPaymentRatio         float64
	PressureLevel               PressureLevel
	MinimumSafeOldHomeSalePrice float64
	Strategy                    string
	Reasons                     []string
	RuleVersion                 string
	EffectiveDate               string
	TraceabilityStatus          TraceabilityStatus
	AppliedAssumptions          *Assumptions
	RecommendedDownPaymentRate  float64
	RecommendedDownPayment      float64
	LoanBreakdown               *LoanBreakdown
	TaxBreakdown                *TaxBreakdown
	PolicyVersion               *PolicyVersionReference
	Sources                     []PolicySource
	ManualOverrides             []AppliedManualOverride
	Disclaimer                  string
}

func (input HousingCapacityInput) ValidateAt(asOf time.Time) error {
	values := []float64{
		input.CashOnHand,
		input.OldHomeValue,
		input.OldLoanBalance,
		input.MonthlyIncome,
		input.CurrentMonthlyMortgage,
		input.AcceptableMonthlyMortgage,
		input.TargetTotalPrice,
		input.RenovationBudget,
		input.TransactionCosts,
		input.TransitionRentCost,
	}
	for _, value := range values {
		if !isFinite(value) || value < 0 {
			return ErrInvalidInput
		}
	}
	if input.MonthlyIncome <= 0 || input.TargetTotalPrice <= 0 {
		return ErrInvalidInput
	}
	if input.LoanOverride != nil {
		if err := input.LoanOverride.Validate(); err != nil {
			return ErrInvalidInput
		}
	}
	if input.CityPolicyOverride != nil {
		if err := input.CityPolicyOverride.ValidateAt(asOf); err != nil {
			return ErrInvalidInput
		}
	}
	return nil
}

func (loan LoanParams) Validate() error {
	if !isFinite(loan.AnnualInterestRate) || loan.AnnualInterestRate < 0 || loan.AnnualInterestRate > 1 {
		return ErrInvalidInput
	}
	if loan.LoanTermMonths < 1 || loan.LoanTermMonths > 600 {
		return ErrInvalidInput
	}
	if loan.RepaymentMethod != RepaymentEqualInstallment && loan.RepaymentMethod != RepaymentEqualPrincipal {
		return ErrInvalidInput
	}
	return nil
}

func (policy CityPolicy) ValidateAt(asOf time.Time) error {
	if strings.TrimSpace(policy.City) == "" || strings.TrimSpace(policy.PolicyName) == "" || strings.TrimSpace(policy.Source) == "" {
		return ErrInvalidInput
	}
	if !isFinite(policy.DownPaymentRate) || policy.DownPaymentRate <= 0 || policy.DownPaymentRate >= 1 {
		return ErrInvalidInput
	}
	if policy.Origin != "" && policy.Origin != OriginConfiguredDefault && policy.Origin != OriginUserOverride {
		return ErrInvalidInput
	}
	if !isEffectiveDateValid(policy.EffectiveDate, asOf) {
		return ErrInvalidInput
	}
	return nil
}

func (assumptions Assumptions) ValidateAt(asOf time.Time) error {
	if strings.TrimSpace(assumptions.RuleVersion) == "" || strings.TrimSpace(assumptions.RuleSource) == "" ||
		strings.TrimSpace(assumptions.LoanSource) == "" {
		return ErrInvalidAssumptions
	}
	if !isEffectiveDateValid(assumptions.EffectiveDate, asOf) {
		return ErrInvalidAssumptions
	}
	if err := assumptions.Loan.Validate(); err != nil {
		return ErrInvalidAssumptions
	}
	if assumptions.LoanOrigin != OriginConfiguredDefault && assumptions.LoanOrigin != OriginUserOverride {
		return ErrInvalidAssumptions
	}
	if err := assumptions.CityPolicy.ValidateAt(asOf); err != nil ||
		(assumptions.CityPolicy.Origin != OriginConfiguredDefault && assumptions.CityPolicy.Origin != OriginUserOverride) {
		return ErrInvalidAssumptions
	}
	if !isFinite(assumptions.ReserveMonths) || assumptions.ReserveMonths < 0 {
		return ErrInvalidAssumptions
	}
	thresholds := assumptions.PressureThresholds
	if !isFinite(thresholds.SafeRatio) || !isFinite(thresholds.StrainedRatio) ||
		!isFinite(thresholds.DangerRatio) || !isFinite(thresholds.DangerMultiplier) ||
		thresholds.SafeRatio <= 0 || thresholds.SafeRatio >= thresholds.StrainedRatio ||
		thresholds.StrainedRatio >= thresholds.DangerRatio || thresholds.DangerRatio >= 1 ||
		thresholds.DangerMultiplier <= 0 {
		return ErrInvalidAssumptions
	}
	if !isFinite(assumptions.OldHomeShareThreshold) || assumptions.OldHomeShareThreshold < 0 || assumptions.OldHomeShareThreshold > 1 {
		return ErrInvalidAssumptions
	}
	return nil
}

// ApplyOverrides resolves the exact assumptions used for one calculation.
// Submitting unchanged defaults preserves configured_default provenance.
func (assumptions Assumptions) ApplyOverrides(input HousingCapacityInput) Assumptions {
	if input.LoanOverride != nil && *input.LoanOverride != assumptions.Loan {
		assumptions.Loan = *input.LoanOverride
		assumptions.LoanSource = "user_input"
		assumptions.LoanOrigin = OriginUserOverride
	}
	if input.CityPolicyOverride != nil {
		override := normalizeCityPolicy(*input.CityPolicyOverride)
		if !sameCityPolicyValues(override, assumptions.CityPolicy) {
			override.Origin = OriginUserOverride
			assumptions.CityPolicy = override
		}
	}
	return assumptions
}

func Calculate(input HousingCapacityInput, assumptions Assumptions, asOf time.Time) (HousingCapacityResult, error) {
	if err := input.ValidateAt(asOf); err != nil {
		return HousingCapacityResult{}, err
	}
	applied := assumptions.ApplyOverrides(input)
	if err := applied.ValidateAt(asOf); err != nil {
		return HousingCapacityResult{}, err
	}

	coefficient := applied.monthlyPaymentCoefficient()
	netOldHomeProceeds := math.Max(input.OldHomeValue-input.OldLoanBalance, 0)
	reserve := input.MonthlyIncome * applied.ReserveMonths
	requiredCosts := input.RenovationBudget + input.TransactionCosts + input.TransitionRentCost
	deployableCash := math.Max(input.CashOnHand+netOldHomeProceeds-requiredCosts-reserve, 0)

	monthlyCapacityToTotalPrice := func(ratio float64) float64 {
		availableMonthlyPayment := math.Max(math.Min(
			input.AcceptableMonthlyMortgage,
			input.MonthlyIncome*ratio-input.CurrentMonthlyMortgage,
		), 0)
		return deployableCash + availableMonthlyPayment/coefficient
	}

	thresholds := applied.PressureThresholds
	safeTotalPrice := round(monthlyCapacityToTotalPrice(thresholds.SafeRatio), 1)
	strainedTotalPrice := round(monthlyCapacityToTotalPrice(thresholds.StrainedRatio), 1)
	dangerTotalPrice := round(
		deployableCash+math.Max(
			input.MonthlyIncome*thresholds.DangerRatio-input.CurrentMonthlyMortgage,
			input.AcceptableMonthlyMortgage*thresholds.DangerMultiplier,
		)/coefficient,
		1,
	)

	requiredUpfront := input.TargetTotalPrice*applied.CityPolicy.DownPaymentRate + requiredCosts + reserve
	downPaymentGap := round(math.Max(requiredUpfront-input.CashOnHand-netOldHomeProceeds, 0), 1)
	monthlyPayment := round(input.TargetTotalPrice*coefficient, 2)
	monthlyPaymentRatio := round((monthlyPayment+input.CurrentMonthlyMortgage)/input.MonthlyIncome, 3)

	pressureLevel := PressureSafe
	if monthlyPaymentRatio > thresholds.StrainedRatio {
		pressureLevel = PressureDanger
	} else if monthlyPaymentRatio > thresholds.SafeRatio {
		pressureLevel = PressureStrained
	}

	oldHomeProceedsShare := netOldHomeProceeds / math.Max(input.CashOnHand+netOldHomeProceeds, 1)
	reasons := make([]string, 0, 3)
	if oldHomeProceedsShare > applied.OldHomeShareThreshold {
		reasons = append(reasons, "旧房净回款占首付能力比重较高，未锁定成交前不宜贸然下定。")
	}
	if downPaymentGap > 0 {
		reasons = append(reasons, "目标总价下存在首付或过渡资金缺口，需要先补足安全垫。")
	}
	switch pressureLevel {
	case PressureDanger:
		reasons = append(reasons, "目标总价对应的月供收入比超过危险线，现金流缓冲不足。")
	case PressureStrained:
		reasons = append(reasons, "目标总价已高于安全月供线，适合通过砍价或降低总价回到安全区。")
	default:
		reasons = append(reasons, "目标总价对应月供仍在安全线内，可以继续推进看房。")
	}

	minimumSafeOldHomeSalePrice := round(
		input.OldLoanBalance+math.Max(requiredUpfront-input.CashOnHand, 0),
		1,
	)
	strategy := "可以同步推进"
	if pressureLevel == PressureDanger || downPaymentGap > 0 {
		strategy = "暂缓改善"
	} else if oldHomeProceedsShare > applied.OldHomeShareThreshold {
		strategy = "先卖后买或同步推进"
	}

	appliedCopy := applied
	return HousingCapacityResult{
		NetOldHomeProceeds:          round(netOldHomeProceeds, 1),
		DeployableCash:              round(deployableCash, 1),
		SafeTotalPrice:              safeTotalPrice,
		StrainedTotalPrice:          strainedTotalPrice,
		DangerTotalPrice:            dangerTotalPrice,
		DownPaymentGap:              downPaymentGap,
		MonthlyPayment:              monthlyPayment,
		MonthlyPaymentRatio:         monthlyPaymentRatio,
		PressureLevel:               pressureLevel,
		MinimumSafeOldHomeSalePrice: minimumSafeOldHomeSalePrice,
		Strategy:                    strategy,
		Reasons:                     reasons,
		RuleVersion:                 applied.RuleVersion,
		EffectiveDate:               applied.EffectiveDate,
		TraceabilityStatus:          TraceabilityComplete,
		AppliedAssumptions:          &appliedCopy,
	}, nil
}

func (assumptions Assumptions) monthlyPaymentCoefficient() float64 {
	return (1 - assumptions.CityPolicy.DownPaymentRate) * assumptions.Loan.perPrincipalMonthlyFactor()
}

func (loan LoanParams) perPrincipalMonthlyFactor() float64 {
	n := loan.LoanTermMonths
	if n <= 0 {
		return 0
	}
	rate := loan.AnnualInterestRate / 12
	if rate <= 0 {
		return 1 / float64(n)
	}
	if loan.RepaymentMethod == RepaymentEqualPrincipal {
		return 1/float64(n) + rate
	}
	power := math.Pow(1+rate, float64(n))
	return rate * power / (power - 1)
}

func normalizeCityPolicy(policy CityPolicy) CityPolicy {
	policy.City = strings.TrimSpace(policy.City)
	policy.PolicyName = strings.TrimSpace(policy.PolicyName)
	policy.EffectiveDate = strings.TrimSpace(policy.EffectiveDate)
	policy.Source = strings.TrimSpace(policy.Source)
	policy.Origin = ""
	return policy
}

func sameCityPolicyValues(left, right CityPolicy) bool {
	left = normalizeCityPolicy(left)
	right = normalizeCityPolicy(right)
	return left.City == right.City && left.PolicyName == right.PolicyName &&
		left.DownPaymentRate == right.DownPaymentRate && left.EffectiveDate == right.EffectiveDate &&
		left.Source == right.Source
}

func isEffectiveDateValid(value string, asOf time.Time) bool {
	value = strings.TrimSpace(value)
	if asOf.IsZero() {
		return false
	}
	if _, err := time.Parse(time.DateOnly, value); err != nil {
		return false
	}
	return value <= asOf.Format(time.DateOnly)
}

func isFinite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

func round(value float64, digits int) float64 {
	factor := math.Pow(10, float64(digits))
	return math.Round(value*factor) / factor
}
