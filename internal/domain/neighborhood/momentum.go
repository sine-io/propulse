package neighborhood

import "time"

const (
	recentTransactionWindowDays    = 30
	transactionEvidenceWindowDays  = 90
	minimumTransactionSampleCount  = 3
	strongTransactionMomentumRatio = 1.2
	weakTransactionMomentumRatio   = 0.8
)

type TransactionMomentumEvidence struct {
	WindowStart                       time.Time
	WindowEnd                         time.Time
	SampleCount                       int
	RecentThirtyDayCount              int
	PrecedingSixtyDayCount            int
	RecentThirtyDayMonthlyFrequency   float64
	PrecedingSixtyDayMonthlyFrequency float64
}

func NewTransactionMomentumEvidence(collectedAt time.Time, recentThirtyDayCount, precedingSixtyDayCount int) TransactionMomentumEvidence {
	windowEnd := dateUTC(collectedAt)
	return TransactionMomentumEvidence{
		WindowStart:                       windowEnd.AddDate(0, 0, -transactionEvidenceWindowDays),
		WindowEnd:                         windowEnd,
		SampleCount:                       recentThirtyDayCount + precedingSixtyDayCount,
		RecentThirtyDayCount:              recentThirtyDayCount,
		PrecedingSixtyDayCount:            precedingSixtyDayCount,
		RecentThirtyDayMonthlyFrequency:   float64(recentThirtyDayCount),
		PrecedingSixtyDayMonthlyFrequency: float64(precedingSixtyDayCount) * float64(recentTransactionWindowDays) / 60,
	}
}

func CalculateTransactionMomentum(evidence TransactionMomentumEvidence) TransactionMomentum {
	if evidence.SampleCount < minimumTransactionSampleCount {
		return TransactionMomentumUnknown
	}

	recentFrequency := evidence.RecentThirtyDayMonthlyFrequency
	baselineFrequency := evidence.PrecedingSixtyDayMonthlyFrequency
	if recentFrequency > baselineFrequency*strongTransactionMomentumRatio {
		return TransactionMomentumStrong
	}
	if recentFrequency < baselineFrequency*weakTransactionMomentumRatio {
		return TransactionMomentumWeak
	}
	return TransactionMomentumStable
}

func dateUTC(value time.Time) time.Time {
	year, month, day := value.UTC().Date()
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}
