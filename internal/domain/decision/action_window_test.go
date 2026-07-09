package decision

import (
	"testing"

	domaincapacity "github.com/sine-io/propulse/internal/domain/capacity"
	domainneighborhood "github.com/sine-io/propulse/internal/domain/neighborhood"
)

func TestRecommendActionWindowBargainsWhenBudgetServiceableAndBuyerWindowOpen(t *testing.T) {
	result := RecommendActionWindow(ActionWindowInput{
		BudgetPressure:       domaincapacity.PressureStrained,
		HasDownPaymentGap:    false,
		NeighborhoodStatus:   domainneighborhood.NeighborhoodStatusBargain,
		TargetLayoutScarcity: domainneighborhood.ScarcityMedium,
		AlternativesBetter:   true,
	})

	if result.Action != ActionBargain {
		t.Fatalf("Action = %q, want %q", result.Action, ActionBargain)
	}
	if result.Confidence != ConfidenceHigh {
		t.Fatalf("Confidence = %q, want %q", result.Confidence, ConfidenceHigh)
	}
	if len(result.Checklist) == 0 || result.Checklist[0] != "约看 3 套成交区间附近、挂牌超过 60 天的目标户型。" {
		t.Fatalf("Checklist = %#v", result.Checklist)
	}
	if len(result.Risks) != 1 || result.Risks[0] != "预算不是完全宽松，砍价失败时不要上调总价硬追。" {
		t.Fatalf("Risks = %#v", result.Risks)
	}
}

func TestRecommendActionWindowWaitsWhenBudgetDangerousAndGapExists(t *testing.T) {
	result := RecommendActionWindow(ActionWindowInput{
		BudgetPressure:       domaincapacity.PressureDanger,
		HasDownPaymentGap:    true,
		NeighborhoodStatus:   domainneighborhood.NeighborhoodStatusBargain,
		TargetLayoutScarcity: domainneighborhood.ScarcityLow,
		AlternativesBetter:   false,
	})

	if result.Action != ActionWait {
		t.Fatalf("Action = %q, want %q", result.Action, ActionWait)
	}
	if result.Confidence != ConfidenceHigh {
		t.Fatalf("Confidence = %q, want %q", result.Confidence, ConfidenceHigh)
	}
	if result.Summary != "先处理预算与旧房回款，再进入看房或出价动作；否则容易把现金流压到危险区。" {
		t.Fatalf("Summary = %q", result.Summary)
	}
}

func TestRecommendActionWindowActsWhenBudgetSafeFocusAndLayoutScarce(t *testing.T) {
	result := RecommendActionWindow(ActionWindowInput{
		BudgetPressure:       domaincapacity.PressureSafe,
		HasDownPaymentGap:    false,
		NeighborhoodStatus:   domainneighborhood.NeighborhoodStatusFocus,
		TargetLayoutScarcity: domainneighborhood.ScarcityHigh,
		AlternativesBetter:   false,
	})

	if result.Action != ActionAct {
		t.Fatalf("Action = %q, want %q", result.Action, ActionAct)
	}
	if result.Confidence != ConfidenceMedium {
		t.Fatalf("Confidence = %q, want %q", result.Confidence, ConfidenceMedium)
	}
	if result.Summary != "预算安全、目标户型稀缺且价格进入可接受区间，可以进入出价准备。" {
		t.Fatalf("Summary = %q", result.Summary)
	}
}
