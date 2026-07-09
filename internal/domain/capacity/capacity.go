package capacity

import "math"

type PressureLevel string

const (
	PressureSafe     PressureLevel = "safe"
	PressureStrained PressureLevel = "strained"
	PressureDanger   PressureLevel = "danger"
)

const (
	downPaymentRate                = 0.35
	monthlyPaymentPerTotalPriceWan = 0.65 * 0.0041
)

type HousingCapacityInput struct {
	CashOnHand                float64 `json:"cashOnHand"`
	OldHomeValue              float64 `json:"oldHomeValue"`
	OldLoanBalance            float64 `json:"oldLoanBalance"`
	MonthlyIncome             float64 `json:"monthlyIncome"`
	CurrentMonthlyMortgage    float64 `json:"currentMonthlyMortgage"`
	AcceptableMonthlyMortgage float64 `json:"acceptableMonthlyMortgage"`
	TargetTotalPrice          float64 `json:"targetTotalPrice"`
	RenovationBudget          float64 `json:"renovationBudget"`
	TransactionCosts          float64 `json:"transactionCosts"`
	TransitionRentCost        float64 `json:"transitionRentCost"`
}

type HousingCapacityResult struct {
	NetOldHomeProceeds          float64       `json:"netOldHomeProceeds"`
	DeployableCash              float64       `json:"deployableCash"`
	SafeTotalPrice              float64       `json:"safeTotalPrice"`
	StrainedTotalPrice          float64       `json:"strainedTotalPrice"`
	DangerTotalPrice            float64       `json:"dangerTotalPrice"`
	DownPaymentGap              float64       `json:"downPaymentGap"`
	MonthlyPayment              float64       `json:"monthlyPayment"`
	MonthlyPaymentRatio         float64       `json:"monthlyPaymentRatio"`
	PressureLevel               PressureLevel `json:"pressureLevel"`
	MinimumSafeOldHomeSalePrice float64       `json:"minimumSafeOldHomeSalePrice"`
	Strategy                    string        `json:"strategy"`
	Reasons                     []string      `json:"reasons"`
}

func Calculate(input HousingCapacityInput) HousingCapacityResult {
	netOldHomeProceeds := math.Max(input.OldHomeValue-input.OldLoanBalance, 0)
	reserve := input.MonthlyIncome * 6
	requiredCosts := input.RenovationBudget + input.TransactionCosts + input.TransitionRentCost
	deployableCash := math.Max(input.CashOnHand+netOldHomeProceeds-requiredCosts-reserve, 0)

	monthlyCapacityToTotalPrice := func(ratio float64) float64 {
		availableMonthlyPayment := math.Max(math.Min(
			input.AcceptableMonthlyMortgage,
			input.MonthlyIncome*ratio-input.CurrentMonthlyMortgage,
		), 0)

		return deployableCash + availableMonthlyPayment/monthlyPaymentPerTotalPriceWan
	}

	safeTotalPrice := round(monthlyCapacityToTotalPrice(0.35), 1)
	strainedTotalPrice := round(monthlyCapacityToTotalPrice(0.45), 1)
	dangerTotalPrice := round(
		deployableCash+math.Max(
			input.MonthlyIncome*0.55-input.CurrentMonthlyMortgage,
			input.AcceptableMonthlyMortgage*1.15,
		)/monthlyPaymentPerTotalPriceWan,
		1,
	)

	requiredUpfront := input.TargetTotalPrice*downPaymentRate + requiredCosts + reserve
	downPaymentGap := round(math.Max(requiredUpfront-input.CashOnHand-netOldHomeProceeds, 0), 1)
	monthlyPayment := round(input.TargetTotalPrice*monthlyPaymentPerTotalPriceWan, 2)
	monthlyPaymentRatio := round((monthlyPayment+input.CurrentMonthlyMortgage)/input.MonthlyIncome, 3)

	pressureLevel := PressureSafe
	if monthlyPaymentRatio > 0.45 {
		pressureLevel = PressureDanger
	} else if monthlyPaymentRatio > 0.35 {
		pressureLevel = PressureStrained
	}

	oldHomeProceedsShare := netOldHomeProceeds / math.Max(input.CashOnHand+netOldHomeProceeds, 1)
	reasons := make([]string, 0, 3)

	if oldHomeProceedsShare > 0.5 {
		reasons = append(reasons, "旧房净回款占首付能力比重较高，未锁定成交前不宜贸然下定。")
	}
	if downPaymentGap > 0 {
		reasons = append(reasons, "目标总价下存在首付或过渡资金缺口，需要先补足安全垫。")
	}
	if pressureLevel == PressureDanger {
		reasons = append(reasons, "目标总价对应的月供收入比超过危险线，现金流缓冲不足。")
	} else if pressureLevel == PressureStrained {
		reasons = append(reasons, "目标总价已高于安全月供线，适合通过砍价或降低总价回到安全区。")
	} else {
		reasons = append(reasons, "目标总价对应月供仍在安全线内，可以继续推进看房。")
	}

	minimumSafeOldHomeSalePrice := round(
		input.OldLoanBalance+math.Max(requiredUpfront-input.CashOnHand, 0),
		1,
	)

	strategy := "可以同步推进"
	if pressureLevel == PressureDanger || downPaymentGap > 0 {
		strategy = "暂缓改善"
	} else if oldHomeProceedsShare > 0.5 {
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
	}
}

func round(value float64, digits int) float64 {
	factor := math.Pow(10, float64(digits))
	return math.Round(value*factor) / factor
}
