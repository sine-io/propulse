package neighborhood

import (
	"testing"
	"time"
)

func TestNewTransactionMomentumEvidenceUsesInclusiveNinetyDayUTCWindow(t *testing.T) {
	collectedAt := time.Date(2026, 7, 14, 1, 30, 0, 0, time.FixedZone("UTC+8", 8*60*60))
	got := NewTransactionMomentumEvidence(collectedAt, 3, 5)

	wantEnd := time.Date(2026, 7, 13, 0, 0, 0, 0, time.UTC)
	wantStart := time.Date(2026, 4, 14, 0, 0, 0, 0, time.UTC)
	if !got.WindowStart.Equal(wantStart) || !got.WindowEnd.Equal(wantEnd) {
		t.Fatalf("window = %s through %s, want %s through %s", got.WindowStart, got.WindowEnd, wantStart, wantEnd)
	}
	if got.SampleCount != 8 || got.RecentThirtyDayMonthlyFrequency != 3 || got.PrecedingSixtyDayMonthlyFrequency != 2.5 {
		t.Fatalf("evidence = %#v", got)
	}
}

func TestCalculateTransactionMomentumUsesOnlyTransactionEvidence(t *testing.T) {
	tests := []struct {
		name      string
		recent    int
		preceding int
		want      TransactionMomentum
	}{
		{name: "zero transactions", recent: 0, preceding: 0, want: TransactionMomentumUnknown},
		{name: "fewer than three samples", recent: 2, preceding: 0, want: TransactionMomentumUnknown},
		{name: "strong above 1.2 ratio", recent: 4, preceding: 6, want: TransactionMomentumStrong},
		{name: "stable at 1.2 ratio", recent: 3, preceding: 5, want: TransactionMomentumStable},
		{name: "stable at 0.8 ratio", recent: 2, preceding: 5, want: TransactionMomentumStable},
		{name: "weak below 0.8 ratio", recent: 1, preceding: 4, want: TransactionMomentumWeak},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			evidence := NewTransactionMomentumEvidence(time.Date(2026, 7, 14, 0, 0, 0, 0, time.UTC), test.recent, test.preceding)
			if got := CalculateTransactionMomentum(evidence); got != test.want {
				t.Fatalf("CalculateTransactionMomentum() = %q, want %q", got, test.want)
			}
		})
	}
}
