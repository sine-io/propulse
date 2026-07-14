package decision

import (
	"time"

	domaincapacity "github.com/sine-io/propulse/internal/domain/capacity"
	domaindecision "github.com/sine-io/propulse/internal/domain/decision"
	domainneighborhood "github.com/sine-io/propulse/internal/domain/neighborhood"
)

type FactorKey string

const (
	FactorBudgetPressure      FactorKey = "budget_pressure"
	FactorDownPaymentGap      FactorKey = "down_payment_gap"
	FactorMarketSignal        FactorKey = "market_signal"
	FactorTransactionMomentum FactorKey = "transaction_momentum"
	FactorTargetLayoutSupply  FactorKey = "target_layout_supply"
	FactorAlternatives        FactorKey = "alternatives"
)

type FactorStatus string

const (
	FactorStatusPositive FactorStatus = "positive"
	FactorStatusNeutral  FactorStatus = "neutral"
	FactorStatusCaution  FactorStatus = "caution"
	FactorStatusNegative FactorStatus = "negative"
	FactorStatusUnknown  FactorStatus = "unknown"
)

type FactorSourceType string

const (
	FactorSourceCapacityCalculation   FactorSourceType = "capacity_calculation"
	FactorSourceNeighborhoodMetric    FactorSourceType = "neighborhood_metric"
	FactorSourceAlternativeComparison FactorSourceType = "alternative_comparison"
)

type EvidenceValueType string

const (
	EvidenceValueText    EvidenceValueType = "text"
	EvidenceValueNumber  EvidenceValueType = "number"
	EvidenceValueBoolean EvidenceValueType = "boolean"
)

type ActionWindowResult struct {
	Action              domaindecision.ActionWindow
	Confidence          domaindecision.Confidence
	ConfidenceReasons   []string
	Summary             string
	Target              ActionWindowTarget
	CapacityCalculation CapacityCalculationReference
	Metric              DecisionMetricReference
	Factors             []DecisionFactor
	Checklist           []string
	Risks               []string
}

type ActionWindowTarget struct {
	NeighborhoodID string
	Name           string
	Area           string
	TargetLayout   string
}

type CapacityCalculationReference struct {
	ID                 string
	CreatedAt          time.Time
	RuleVersion        string
	TraceabilityStatus domaincapacity.TraceabilityStatus
}

type DecisionMetricReference struct {
	ID                     string
	CollectionRunID        string
	AlgorithmVersion       string
	CollectedAt            time.Time
	CalculatedAt           time.Time
	SourceIDs              []string
	ListingSampleCount     int
	TransactionSampleCount int
	Coverage               domainneighborhood.Coverage
	Freshness              domainneighborhood.Freshness
	QualityState           domainneighborhood.MarketQualityState
	QualityWarnings        []domainneighborhood.QualityWarning
}

type DecisionFactor struct {
	Key      FactorKey
	Status   FactorStatus
	Summary  string
	Source   *DecisionFactorSource
	Evidence []DecisionFactorEvidence
}

type DecisionFactorSource struct {
	Type       FactorSourceType
	ID         string
	ObservedAt time.Time
}

type DecisionFactorEvidence struct {
	Key          string
	Label        string
	ValueType    EvidenceValueType
	TextValue    *string
	NumberValue  *float64
	BooleanValue *bool
	Unit         string
}
