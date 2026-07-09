package neighborhood

import (
	"math"
	"strconv"
)

type TransactionMomentum string

const (
	TransactionMomentumWeak   TransactionMomentum = "weak"
	TransactionMomentumStable TransactionMomentum = "stable"
	TransactionMomentumStrong TransactionMomentum = "strong"
)

type SupplyPressure string

const (
	SupplyPressureLow    SupplyPressure = "low"
	SupplyPressureMedium SupplyPressure = "medium"
	SupplyPressureHigh   SupplyPressure = "high"
)

type NeighborhoodStatus string

const (
	NeighborhoodStatusFocus      NeighborhoodStatus = "重点看"
	NeighborhoodStatusObserve    NeighborhoodStatus = "继续观察"
	NeighborhoodStatusBargain    NeighborhoodStatus = "适合砍价"
	NeighborhoodStatusPriceHard  NeighborhoodStatus = "价格偏硬"
	NeighborhoodStatusNotSuggest NeighborhoodStatus = "暂不建议追"
)

type Scarcity string

const (
	ScarcityLow    Scarcity = "low"
	ScarcityMedium Scarcity = "medium"
	ScarcityHigh   Scarcity = "high"
)

type PriceRange struct {
	Min float64
	Max float64
}

type SignalInput struct {
	Name                  string
	ListingPriceRange     PriceRange
	TransactionPriceRange PriceRange
	ListedHomes           int
	ListedHomesChangePct  float64
	PriceCutHomes         int
	AvgDaysOnMarket       float64
	TransactionMomentum   TransactionMomentum
	TargetLayoutSupply    int
}

type SignalResult struct {
	Name                 string
	Status               NeighborhoodStatus
	SupplyPressure       SupplyPressure
	PriceCutShare        float64
	PriceGapPct          float64
	TargetLayoutScarcity Scarcity
	NextAction           string
	Reasons              []string
}

func EvaluateSignal(input SignalInput) SignalResult {
	priceCutShare := float64(input.PriceCutHomes) / math.Max(float64(input.ListedHomes), 1)
	listingMid := midpoint(input.ListingPriceRange)
	transactionMid := midpoint(input.TransactionPriceRange)
	priceGapPct := (listingMid - transactionMid) / math.Max(listingMid, 1)

	supplyPressure := SupplyPressureMedium
	if input.ListedHomesChangePct >= 12 || priceCutShare >= 0.2 || input.AvgDaysOnMarket >= 70 {
		supplyPressure = SupplyPressureHigh
	} else if input.ListedHomes < 20 &&
		input.ListedHomesChangePct <= 0 &&
		priceCutShare < 0.1 &&
		input.AvgDaysOnMarket < 45 {
		supplyPressure = SupplyPressureLow
	}

	targetLayoutScarcity := ScarcityLow
	if input.TargetLayoutSupply <= 4 {
		targetLayoutScarcity = ScarcityHigh
	} else if input.TargetLayoutSupply <= 10 {
		targetLayoutScarcity = ScarcityMedium
	}

	reasons := []string{}
	if input.ListedHomesChangePct >= 12 {
		reasons = append(reasons, "挂牌量明显增加，买方可选择空间扩大。")
	}
	if priceCutShare >= 0.2 {
		reasons = append(reasons, "降价房源占比超过 20%，房东预期开始松动。")
	}
	if input.TransactionMomentum == TransactionMomentumWeak {
		reasons = append(reasons, "成交偏弱，挂牌价缺少成交支撑。")
	}
	if supplyPressure == SupplyPressureLow {
		reasons = append(reasons, "目标户型供给偏少，成交对挂牌价仍有支撑。")
	}

	status := NeighborhoodStatusObserve
	nextAction := "继续每周记录挂牌、降价和成交变化，不急于下判断。"

	if supplyPressure == SupplyPressureHigh &&
		priceCutShare >= 0.2 &&
		input.TransactionMomentum == TransactionMomentumWeak {
		status = NeighborhoodStatusBargain
		nextAction = "重点看 " + formatWan(input.TransactionPriceRange.Min) + "-" + formatWan(input.TransactionPriceRange.Max) + " 万成交区间附近房源，对挂牌久、降价过的房源试探底价。"
	} else if supplyPressure == SupplyPressureLow &&
		input.TransactionMomentum == TransactionMomentumStrong {
		status = NeighborhoodStatusPriceHard
		nextAction = "不要用单套挂牌价追高，等待新增供应或转向替代小区。"
	} else if priceGapPct >= 0.05 && input.TransactionMomentum != TransactionMomentumStrong {
		status = NeighborhoodStatusFocus
		nextAction = "可以开始实地看房，记录缺陷并用成交区间校准报价。"
	} else if priceGapPct < 0.02 && input.TransactionMomentum == TransactionMomentumWeak {
		status = NeighborhoodStatusNotSuggest
		nextAction = "价格优势不明显且成交弱，先把预算和替代小区比较清楚。"
	}

	return SignalResult{
		Name:                 input.Name,
		Status:               status,
		SupplyPressure:       supplyPressure,
		PriceCutShare:        round(priceCutShare, 3),
		PriceGapPct:          round(priceGapPct, 3),
		TargetLayoutScarcity: targetLayoutScarcity,
		NextAction:           nextAction,
		Reasons:              reasons,
	}
}

func midpoint(priceRange PriceRange) float64 {
	return (priceRange.Min + priceRange.Max) / 2
}

func round(value float64, digits int) float64 {
	factor := math.Pow(10, float64(digits))
	return math.Round(value*factor) / factor
}

func formatWan(value float64) string {
	if value == math.Trunc(value) {
		return strconv.FormatInt(int64(value), 10)
	}
	return strconv.FormatFloat(value, 'f', -1, 64)
}
