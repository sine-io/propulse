package decision

import (
	"fmt"
	"strconv"
	"time"

	appcapacity "github.com/sine-io/propulse/internal/application/capacity"
	appneighborhood "github.com/sine-io/propulse/internal/application/neighborhood"
	domaincapacity "github.com/sine-io/propulse/internal/domain/capacity"
	domaindecision "github.com/sine-io/propulse/internal/domain/decision"
	domainneighborhood "github.com/sine-io/propulse/internal/domain/neighborhood"
)

func newActionWindowResult(
	calculation appcapacity.CalculationRecord,
	target appneighborhood.Neighborhood,
	metric appneighborhood.MetricWithSignal,
	recommendation domaindecision.ActionWindowResult,
) ActionWindowResult {
	return ActionWindowResult{
		Action:            recommendation.Action,
		Confidence:        recommendation.Confidence,
		ConfidenceReasons: append([]string(nil), recommendation.ConfidenceReasons...),
		Summary:           recommendation.Summary,
		Target: ActionWindowTarget{
			NeighborhoodID: target.ID,
			Name:           target.Name,
			Area:           target.Area,
			TargetLayout:   target.TargetLayout,
		},
		CapacityCalculation: CapacityCalculationReference{
			ID:                 calculation.ID,
			CreatedAt:          calculation.CreatedAt,
			RuleVersion:        calculation.Result.RuleVersion,
			TraceabilityStatus: calculation.Result.TraceabilityStatus,
		},
		Metric: DecisionMetricReference{
			ID:                     metric.Metric.ID,
			CollectionRunID:        metric.Metric.CollectionRunID,
			AlgorithmVersion:       metric.Metric.AlgorithmVersion,
			CollectedAt:            metric.Metric.CollectedAt,
			CalculatedAt:           metric.Metric.CalculatedAt,
			SourceIDs:              append([]string(nil), metric.Metric.SourceIDs...),
			ListingSampleCount:     metric.Metric.ListingSampleCount,
			TransactionSampleCount: metric.Metric.TransactionSampleCount,
			Coverage:               metric.Metric.Coverage,
			Freshness:              metric.Metric.Freshness,
			QualityState:           metric.Metric.QualityState,
			QualityWarnings:        append([]domainneighborhood.QualityWarning(nil), metric.Metric.QualityWarnings...),
		},
		Factors:   buildDecisionFactors(calculation, metric),
		Checklist: append([]string(nil), recommendation.Checklist...),
		Risks:     append([]string(nil), recommendation.Risks...),
	}
}

func buildDecisionFactors(calculation appcapacity.CalculationRecord, metric appneighborhood.MetricWithSignal) []DecisionFactor {
	capacitySource := &DecisionFactorSource{
		Type:       FactorSourceCapacityCalculation,
		ID:         calculation.ID,
		ObservedAt: calculation.CreatedAt,
	}
	metricSource := &DecisionFactorSource{
		Type:       FactorSourceNeighborhoodMetric,
		ID:         metric.Metric.ID,
		ObservedAt: metric.Metric.CollectedAt,
	}

	transactionEvidence := []DecisionFactorEvidence{
		textEvidence("momentum", "成交动量", string(metric.Metric.TransactionMomentum)),
		numberEvidence("sample_count", "成交样本", float64(metric.Metric.TransactionSampleCount), "笔"),
	}
	if evidence := metric.Metric.TransactionEvidence; evidence != nil {
		transactionEvidence = append(transactionEvidence,
			textEvidence("window_start", "统计窗口起点", evidence.WindowStart.UTC().Format(time.RFC3339)),
			textEvidence("window_end", "统计窗口终点", evidence.WindowEnd.UTC().Format(time.RFC3339)),
			numberEvidence("recent_30_day_count", "近 30 天成交", float64(evidence.RecentThirtyDayCount), "笔"),
			numberEvidence("preceding_60_day_count", "此前 60 天成交", float64(evidence.PrecedingSixtyDayCount), "笔"),
			numberEvidence("recent_30_day_monthly_frequency", "近 30 天月频", evidence.RecentThirtyDayMonthlyFrequency, "笔/月"),
			numberEvidence("preceding_60_day_monthly_frequency", "此前 60 天月频", evidence.PrecedingSixtyDayMonthlyFrequency, "笔/月"),
		)
	}

	return []DecisionFactor{
		{
			Key:     FactorBudgetPressure,
			Status:  budgetPressureStatus(calculation.Result.PressureLevel),
			Summary: budgetPressureSummary(calculation.Result.PressureLevel),
			Source:  capacitySource,
			Evidence: []DecisionFactorEvidence{
				textEvidence("pressure_level", "资金压力", string(calculation.Result.PressureLevel)),
				numberEvidence("target_total_price", "目标总价", calculation.Input.TargetTotalPrice, "万元"),
				numberEvidence("safe_total_price", "安全总价", calculation.Result.SafeTotalPrice, "万元"),
				numberEvidence("monthly_payment_ratio", "月供收入比", calculation.Result.MonthlyPaymentRatio*100, "%"),
			},
		},
		{
			Key:     FactorDownPaymentGap,
			Status:  downPaymentGapStatus(calculation.Result.DownPaymentGap),
			Summary: downPaymentGapSummary(calculation.Result.DownPaymentGap),
			Source:  capacitySource,
			Evidence: []DecisionFactorEvidence{
				numberEvidence("down_payment_gap", "首付缺口", calculation.Result.DownPaymentGap, "万元"),
				booleanEvidence("has_down_payment_gap", "存在首付缺口", calculation.Result.DownPaymentGap > 0),
				numberEvidence("deployable_cash", "可动用现金", calculation.Result.DeployableCash, "万元"),
				numberEvidence("net_old_home_proceeds", "旧房净回款", calculation.Result.NetOldHomeProceeds, "万元"),
			},
		},
		{
			Key:     FactorMarketSignal,
			Status:  marketSignalStatus(metric.Signal.Status),
			Summary: fmt.Sprintf("目标小区信号为“%s”。%s", metric.Signal.Status, metric.Signal.NextAction),
			Source:  metricSource,
			Evidence: []DecisionFactorEvidence{
				textEvidence("neighborhood_status", "小区信号", string(metric.Signal.Status)),
				textEvidence("supply_pressure", "供应压力", string(metric.Signal.SupplyPressure)),
				numberEvidence("listed_homes", "挂牌房源", float64(metric.Metric.ListedHomes), "套"),
				numberEvidence("price_cut_homes", "降价房源", float64(metric.Metric.PriceCutHomes), "套"),
				numberEvidence("price_cut_share", "降价占比", metric.Signal.PriceCutShare*100, "%"),
				numberEvidence("price_gap", "挂牌成交价差", metric.Signal.PriceGapPct*100, "%"),
				textEvidence("quality_state", "数据质量", string(metric.Metric.QualityState)),
			},
		},
		{
			Key:      FactorTransactionMomentum,
			Status:   transactionMomentumStatus(metric.Metric.TransactionMomentum),
			Summary:  transactionMomentumSummary(metric.Metric.TransactionMomentum),
			Source:   metricSource,
			Evidence: transactionEvidence,
		},
		{
			Key:     FactorTargetLayoutSupply,
			Status:  targetLayoutSupplyStatus(metric.Signal.TargetLayoutScarcity),
			Summary: fmt.Sprintf("目标户型当前供给 %d 套，稀缺度为%s。", metric.Metric.TargetLayoutSupply, scarcityLabel(metric.Signal.TargetLayoutScarcity)),
			Source:  metricSource,
			Evidence: []DecisionFactorEvidence{
				numberEvidence("target_layout_supply", "目标户型供给", float64(metric.Metric.TargetLayoutSupply), "套"),
				textEvidence("target_layout_scarcity", "目标户型稀缺度", string(metric.Signal.TargetLayoutScarcity)),
				numberEvidence("listing_sample_count", "挂牌样本", float64(metric.Metric.ListingSampleCount), "套"),
			},
		},
		{
			Key:      FactorAlternatives,
			Status:   FactorStatusUnknown,
			Summary:  "尚未执行可比备选评估，本次置信度不使用备选加分。",
			Source:   nil,
			Evidence: []DecisionFactorEvidence{},
		},
	}
}

func budgetPressureStatus(pressure domaincapacity.PressureLevel) FactorStatus {
	switch pressure {
	case domaincapacity.PressureSafe:
		return FactorStatusPositive
	case domaincapacity.PressureStrained:
		return FactorStatusCaution
	case domaincapacity.PressureDanger:
		return FactorStatusNegative
	default:
		return FactorStatusUnknown
	}
}

func budgetPressureSummary(pressure domaincapacity.PressureLevel) string {
	switch pressure {
	case domaincapacity.PressureSafe:
		return "资金压力处于安全区。"
	case domaincapacity.PressureStrained:
		return "资金压力接近承压区，出价需要严格受安全总价约束。"
	case domaincapacity.PressureDanger:
		return "资金压力处于危险区，不支持新增购房承诺。"
	default:
		return "资金压力状态未知。"
	}
}

func downPaymentGapStatus(gap float64) FactorStatus {
	if gap > 0 {
		return FactorStatusNegative
	}
	return FactorStatusPositive
}

func downPaymentGapSummary(gap float64) string {
	if gap > 0 {
		return "当前测算仍有 " + formatDecisionNumber(gap) + " 万元首付缺口。"
	}
	return "当前测算没有首付缺口。"
}

func marketSignalStatus(status domainneighborhood.NeighborhoodStatus) FactorStatus {
	switch status {
	case domainneighborhood.NeighborhoodStatusBargain, domainneighborhood.NeighborhoodStatusFocus:
		return FactorStatusPositive
	case domainneighborhood.NeighborhoodStatusObserve:
		return FactorStatusNeutral
	case domainneighborhood.NeighborhoodStatusPriceHard:
		return FactorStatusCaution
	case domainneighborhood.NeighborhoodStatusNotSuggest:
		return FactorStatusNegative
	default:
		return FactorStatusUnknown
	}
}

func transactionMomentumStatus(momentum domainneighborhood.TransactionMomentum) FactorStatus {
	switch momentum {
	case domainneighborhood.TransactionMomentumWeak:
		return FactorStatusPositive
	case domainneighborhood.TransactionMomentumStable:
		return FactorStatusNeutral
	case domainneighborhood.TransactionMomentumStrong:
		return FactorStatusCaution
	default:
		return FactorStatusUnknown
	}
}

func transactionMomentumSummary(momentum domainneighborhood.TransactionMomentum) string {
	switch momentum {
	case domainneighborhood.TransactionMomentumWeak:
		return "真实成交动量偏弱，买方议价条件相对有利。"
	case domainneighborhood.TransactionMomentumStable:
		return "真实成交动量平稳，暂未形成明显方向。"
	case domainneighborhood.TransactionMomentumStrong:
		return "真实成交动量活跃，买方议价空间可能收窄。"
	default:
		return "成交证据不足，无法判断成交动量。"
	}
}

func targetLayoutSupplyStatus(scarcity domainneighborhood.Scarcity) FactorStatus {
	switch scarcity {
	case domainneighborhood.ScarcityLow:
		return FactorStatusPositive
	case domainneighborhood.ScarcityMedium:
		return FactorStatusNeutral
	case domainneighborhood.ScarcityHigh:
		return FactorStatusCaution
	default:
		return FactorStatusUnknown
	}
}

func scarcityLabel(scarcity domainneighborhood.Scarcity) string {
	switch scarcity {
	case domainneighborhood.ScarcityLow:
		return "低"
	case domainneighborhood.ScarcityMedium:
		return "中"
	case domainneighborhood.ScarcityHigh:
		return "高"
	default:
		return "未知"
	}
}

func textEvidence(key, label, value string) DecisionFactorEvidence {
	return DecisionFactorEvidence{Key: key, Label: label, ValueType: EvidenceValueText, TextValue: &value}
}

func numberEvidence(key, label string, value float64, unit string) DecisionFactorEvidence {
	return DecisionFactorEvidence{Key: key, Label: label, ValueType: EvidenceValueNumber, NumberValue: &value, Unit: unit}
}

func booleanEvidence(key, label string, value bool) DecisionFactorEvidence {
	return DecisionFactorEvidence{Key: key, Label: label, ValueType: EvidenceValueBoolean, BooleanValue: &value}
}

func formatDecisionNumber(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}
