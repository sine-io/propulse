package decision

import (
	domaincapacity "github.com/sine-io/propulse/backend/internal/domain/capacity"
	domainneighborhood "github.com/sine-io/propulse/backend/internal/domain/neighborhood"
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
	BudgetPressure       domaincapacity.PressureLevel
	HasDownPaymentGap    bool
	NeighborhoodStatus   domainneighborhood.NeighborhoodStatus
	TargetLayoutScarcity domainneighborhood.Scarcity
	AlternativesBetter   bool
}

type ActionWindowResult struct {
	Action     ActionWindow `json:"action"`
	Confidence Confidence   `json:"confidence"`
	Summary    string       `json:"summary"`
	Checklist  []string     `json:"checklist"`
	Risks      []string     `json:"risks"`
}

func RecommendActionWindow(input ActionWindowInput) ActionWindowResult {
	if input.BudgetPressure == domaincapacity.PressureDanger || input.HasDownPaymentGap {
		return ActionWindowResult{
			Action:     ActionWait,
			Confidence: ConfidenceHigh,
			Summary:    "先处理预算与旧房回款，再进入看房或出价动作；否则容易把现金流压到危险区。",
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
		if input.AlternativesBetter {
			confidence = ConfidenceHigh
		}

		risks := []string{"单套低价房源可能存在硬伤，不要把个案当成整体价格。"}
		if input.BudgetPressure == domaincapacity.PressureStrained {
			risks = []string{"预算不是完全宽松，砍价失败时不要上调总价硬追。"}
		}

		return ActionWindowResult{
			Action:     ActionBargain,
			Confidence: confidence,
			Summary:    "预算仍可服务，且目标小区供应与降价信号支持买方试探底价。",
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
			Action:     ActionAct,
			Confidence: ConfidenceMedium,
			Summary:    "预算安全、目标户型稀缺且价格进入可接受区间，可以进入出价准备。",
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
			Action:     ActionWait,
			Confidence: ConfidenceMedium,
			Summary:    "小区价格仍偏硬，当前重点不是下定，而是等待新增供应或转向替代小区。",
			Checklist: []string{
				"观察未来两周新增挂牌和降价房源是否增加。",
				"把预算相近的替代小区加入观察池。",
			},
			Risks: []string{"追高会吞掉预算安全垫。"},
		}
	}

	return ActionWindowResult{
		Action:     ActionView,
		Confidence: ConfidenceMedium,
		Summary:    "可以开始实地看房，但还不需要急着报价或下定。",
		Checklist: []string{
			"每周记录挂牌、降价、成交和带看反馈。",
			"用看房记录筛掉硬伤房源，沉淀可谈价清单。",
		},
		Risks: []string{"数据仍不充分，单周变化不能当成明确趋势。"},
	}
}
