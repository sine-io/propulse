package neighborhood

import "testing"

func TestCalculateMetricChangeReturnsAbsoluteAndPercentageChange(t *testing.T) {
	got := CalculateMetricChange(15, 12)
	if got.Current != 15 || got.Baseline != 12 || got.AbsoluteChange != 3 || got.PercentageChange == nil || *got.PercentageChange != 25 || got.PercentageStatus != PercentageChangeAvailable {
		t.Fatalf("CalculateMetricChange() = %#v", got)
	}
}

func TestCalculateMetricChangePreservesAbsoluteChangeForZeroBaseline(t *testing.T) {
	got := CalculateMetricChange(3, 0)
	if got.AbsoluteChange != 3 || got.PercentageChange != nil || got.PercentageStatus != PercentageChangeZeroBaseline {
		t.Fatalf("CalculateMetricChange() = %#v", got)
	}
}
