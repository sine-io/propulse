package capacity

import (
	"errors"
	"math"
)

var ErrInvalidInput = errors.New("invalid housing capacity input")

type PressureLevel string

const (
	PressureSafe     PressureLevel = "safe"
	PressureStrained PressureLevel = "strained"
	PressureDanger   PressureLevel = "danger"
)

// Assumptions 收纳换房测算使用的全部规则参数，并带版本与生效日期，
// 使结果可追溯到一组明确、有出处的假设（CALC-006.1 / #66）。
type Assumptions struct {
	RuleVersion               string
	EffectiveDate             string // ISO 日期，如 2026-07-01
	DownPaymentRate           float64
	MonthlyPaymentCoefficient float64 // 每万总价对应月供（万）
	ReserveMonths             float64
	SafeRatio                 float64 // 安全月供收入比上限
	StrainedRatio             float64 // 偏高月供收入比上限
	DangerRatio               float64 // 危险线月供收入比
	DangerMultiplier          float64 // 危险总价对可接受月供的放大系数
	OldHomeShareThreshold     float64 // 旧房回款占首付能力的高占比阈值
}

// DefaultAssumptions 返回当前生效的规则参数。
// 调整参数或阈值时应递增 RuleVersion 并更新 EffectiveDate。
func DefaultAssumptions() Assumptions {
	return Assumptions{
		RuleVersion:               "2026.07",
		EffectiveDate:             "2026-07-01",
		DownPaymentRate:           0.35,
		MonthlyPaymentCoefficient: 0.65 * 0.0041,
		ReserveMonths:             6,
		SafeRatio:                 0.35,
		StrainedRatio:             0.45,
		DangerRatio:               0.55,
		DangerMultiplier:          1.15,
		OldHomeShareThreshold:     0.5,
	}
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
}

func (input HousingCapacityInput) Validate() error {
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
		if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 {
			return ErrInvalidInput
		}
	}
	if input.MonthlyIncome <= 0 || input.TargetTotalPrice <= 0 {
		return ErrInvalidInput
	}
	return nil
}

// Calculate 使用当前默认规则参数计算换房能力。
func Calculate(input HousingCapacityInput) HousingCapacityResult {
	return CalculateWith(input, DefaultAssumptions())
}

// CalculateWith 使用给定规则参数计算，结果回带规则版本与生效日期。
func CalculateWith(input HousingCapacityInput, a Assumptions) HousingCapacityResult {
	netOldHomeProceeds := math.Max(input.OldHomeValue-input.OldLoanBalance, 0)
	reserve := input.MonthlyIncome * a.ReserveMonths
	requiredCosts := input.RenovationBudget + input.TransactionCosts + input.TransitionRentCost
	deployableCash := math.Max(input.CashOnHand+netOldHomeProceeds-requiredCosts-reserve, 0)

	monthlyCapacityToTotalPrice := func(ratio float64) float64 {
		availableMonthlyPayment := math.Max(math.Min(
			input.AcceptableMonthlyMortgage,
			input.MonthlyIncome*ratio-input.CurrentMonthlyMortgage,
		), 0)

		return deployableCash + availableMonthlyPayment/a.MonthlyPaymentCoefficient
	}

	safeTotalPrice := round(monthlyCapacityToTotalPrice(a.SafeRatio), 1)
	strainedTotalPrice := round(monthlyCapacityToTotalPrice(a.StrainedRatio), 1)
	dangerTotalPrice := round(
		deployableCash+math.Max(
			input.MonthlyIncome*a.DangerRatio-input.CurrentMonthlyMortgage,
			input.AcceptableMonthlyMortgage*a.DangerMultiplier,
		)/a.MonthlyPaymentCoefficient,
		1,
	)

	requiredUpfront := input.TargetTotalPrice*a.DownPaymentRate + requiredCosts + reserve
	downPaymentGap := round(math.Max(requiredUpfront-input.CashOnHand-netOldHomeProceeds, 0), 1)
	monthlyPayment := round(input.TargetTotalPrice*a.MonthlyPaymentCoefficient, 2)
	monthlyPaymentRatio := round((monthlyPayment+input.CurrentMonthlyMortgage)/input.MonthlyIncome, 3)

	pressureLevel := PressureSafe
	if monthlyPaymentRatio > a.StrainedRatio {
		pressureLevel = PressureDanger
	} else if monthlyPaymentRatio > a.SafeRatio {
		pressureLevel = PressureStrained
	}

	oldHomeProceedsShare := netOldHomeProceeds / math.Max(input.CashOnHand+netOldHomeProceeds, 1)
	reasons := make([]string, 0, 3)

	if oldHomeProceedsShare > a.OldHomeShareThreshold {
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
	} else if oldHomeProceedsShare > a.OldHomeShareThreshold {
		strategy = "先卖后买或同步推进"
	}

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
		RuleVersion:                 a.RuleVersion,
		EffectiveDate:               a.EffectiveDate,
	}
}

func round(value float64, digits int) float64 {
	factor := math.Pow(10, float64(digits))
	return math.Round(value*factor) / factor
}
