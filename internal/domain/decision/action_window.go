package decision

import (
	domaincapacity "github.com/sine-io/propulse/internal/domain/capacity"
	domainneighborhood "github.com/sine-io/propulse/internal/domain/neighborhood"
)

type ActionWindow string

const (
	ActionView    ActionWindow = "看"
	ActionWait    ActionWindow = "等"
	ActionBargain ActionWindow = "砍价"
	ActionAct     ActionWindow = "出手"
)

type Confidence string

const (
	ConfidenceLow    Confidence = "低"
	ConfidenceMedium Confidence = "中"
	ConfidenceHigh   Confidence = "高"
)

type ActionWindowInput struct {
	BudgetPressure        domaincapacity.PressureLevel
	HasDownPaymentGap     bool
	NeighborhoodStatus    domainneighborhood.NeighborhoodStatus
	TargetLayoutScarcity  domainneighborhood.Scarcity
	AlternativeComparison AlternativeComparisonStatus
}

type ActionWindowResult struct {
	Action            ActionWindow
	Confidence        Confidence
	ConfidenceReasons []string
	Summary           string
	Checklist         []string
	Risks             []string
}

func RecommendActionWindow(input ActionWindowInput) ActionWindowResult {
	if input.BudgetPressure == domaincapacity.PressureDanger || input.HasDownPaymentGap {
		return ActionWindowResult{
			Action:            ActionWait,
			Confidence:        ConfidenceHigh,
			ConfidenceReasons: []string{"资金测算存在危险压力或首付缺口，属于直接阻断条件。"},
			Summary:           "先处理预算与旧房回款，再进入看房或出价动作；否则容易把现金流压到危险区。",
			Checklist: []string{
				"重新测算安全总价，优先消除首付缺口。",
				"推进旧房成交或降低目标总价。",
				"暂停对超预算房源的下定和追价。",
			},
			Risks: []string{"现金流安全垫不足时，即使小区出现砍价窗口也不宜贸然出手。"},
		}
	}

	if input.NeighborhoodStatus == domainneighborhood.NeighborhoodStatusBargain {
		confidence := ConfidenceMedium
		confidenceReason := "目标小区支持议价，但备选数据不足，不能据此提高置信度。"
		switch input.AlternativeComparison {
		case AlternativeComparisonBetterFound:
			confidence = ConfidenceHigh
			confidenceReason = "目标小区支持议价，且版本化比较发现至少一个预算内更优备选。"
		case AlternativeComparisonNone:
			confidenceReason = "目标小区支持议价，但备选比较没有发现满足规则的更优候选。"
		}
		risks := []string{"单套低价房源可能存在硬伤，不要把个案当成整体价格。"}
		if input.BudgetPressure == domaincapacity.PressureStrained {
			risks = []string{"预算不是完全宽松，砍价失败时不要上调总价硬追。"}
		}

		return ActionWindowResult{
			Action:            ActionBargain,
			Confidence:        confidence,
			ConfidenceReasons: []string{confidenceReason},
			Summary:           "预算仍可服务，且目标小区供应与降价信号支持买方试探底价。",
			Checklist: []string{
				"约看 3 套成交区间附近、挂牌超过 60 天的目标户型。",
				"用近期成交低位作为报价锚点，先试探 3%-8% 让价空间。",
				"同步比较替代小区，保留不成交也能退出的底气。",
			},
			Risks: risks,
		}
	}

	if input.BudgetPressure == domaincapacity.PressureSafe &&
		input.NeighborhoodStatus == domainneighborhood.NeighborhoodStatusFocus &&
		input.TargetLayoutScarcity == domainneighborhood.ScarcityHigh {
		return ActionWindowResult{
			Action:            ActionAct,
			Confidence:        ConfidenceMedium,
			ConfidenceReasons: []string{"预算、小区信号和户型供给方向一致，但结论仍基于单一目标小区。"},
			Summary:           "预算安全、目标户型稀缺且价格进入可接受区间，可以进入出价准备。",
			Checklist: []string{
				"核验房源硬伤、产权和税费后准备正式报价。",
				"把最高出价锁定在安全总价内。",
				"设置 24 小时冷静复盘，避免被单套稀缺性推着追价。",
			},
			Risks: []string{"稀缺房源容易造成竞价，必须提前写清最高价和退出条件。"},
		}
	}

	if input.NeighborhoodStatus == domainneighborhood.NeighborhoodStatusPriceHard {
		return ActionWindowResult{
			Action:            ActionWait,
			Confidence:        ConfidenceMedium,
			ConfidenceReasons: []string{"当前目标小区指标支持价格偏硬判断，但尚无可比备选证据。"},
			Summary:           "小区价格仍偏硬，当前重点不是下定，而是等待新增供应或转向替代小区。",
			Checklist: []string{
				"观察未来两周新增挂牌和降价房源是否增加。",
				"把预算相近的替代小区加入观察池。",
			},
			Risks: []string{"追高会吞掉预算安全垫。"},
		}
	}

	return ActionWindowResult{
		Action:            ActionView,
		Confidence:        ConfidenceMedium,
		ConfidenceReasons: []string{"当前证据足以支持继续看房，但尚未形成明确的等待、议价或出手组合。"},
		Summary:           "可以开始实地看房，但还不需要急着报价或下定。",
		Checklist: []string{
			"每周记录挂牌、降价、成交和带看反馈。",
			"用看房记录筛掉硬伤房源，沉淀可谈价清单。",
		},
		Risks: []string{"数据仍不充分，单周变化不能当成明确趋势。"},
	}
}
