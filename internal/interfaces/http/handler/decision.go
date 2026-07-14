package handler

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	appdecision "github.com/sine-io/propulse/internal/application/decision"
	domaincapacity "github.com/sine-io/propulse/internal/domain/capacity"
	domaindecision "github.com/sine-io/propulse/internal/domain/decision"
	domainneighborhood "github.com/sine-io/propulse/internal/domain/neighborhood"
)

type DecisionApplication interface {
	GetActionWindow(ctx context.Context, query appdecision.GetActionWindowQuery) (appdecision.ActionWindowResult, error)
}

type Decision struct {
	app DecisionApplication
}

type actionWindowResponse struct {
	Action                domaindecision.ActionWindow          `json:"action"`
	Confidence            domaindecision.Confidence            `json:"confidence"`
	ConfidenceReasons     []string                             `json:"confidenceReasons"`
	Summary               string                               `json:"summary"`
	Target                actionWindowTargetResponse           `json:"target"`
	CapacityCalculation   capacityCalculationReferenceResponse `json:"capacityCalculation"`
	Metric                decisionMetricReferenceResponse      `json:"metric"`
	AlternativeComparison alternativeComparisonResponse        `json:"alternativeComparison"`
	Factors               []decisionFactorResponse             `json:"factors"`
	Checklist             []string                             `json:"checklist"`
	Risks                 []string                             `json:"risks"`
}

type actionWindowTargetResponse struct {
	NeighborhoodID string `json:"neighborhoodId"`
	Name           string `json:"name"`
	Area           string `json:"area"`
	TargetLayout   string `json:"targetLayout"`
}

type capacityCalculationReferenceResponse struct {
	ID                 string                            `json:"id"`
	CreatedAt          string                            `json:"createdAt"`
	RuleVersion        string                            `json:"ruleVersion"`
	TraceabilityStatus domaincapacity.TraceabilityStatus `json:"traceabilityStatus"`
}

type decisionMetricReferenceResponse struct {
	ID                     string                                `json:"id"`
	CollectionRunID        string                                `json:"collectionRunId"`
	AlgorithmVersion       string                                `json:"algorithmVersion"`
	CollectedAt            string                                `json:"collectedAt"`
	CalculatedAt           string                                `json:"calculatedAt"`
	SourceIDs              []string                              `json:"sourceIds"`
	ListingSampleCount     int                                   `json:"listingSampleCount"`
	TransactionSampleCount int                                   `json:"transactionSampleCount"`
	Coverage               domainneighborhood.Coverage           `json:"coverage"`
	Freshness              domainneighborhood.Freshness          `json:"freshness"`
	QualityState           domainneighborhood.MarketQualityState `json:"qualityState"`
	QualityWarnings        []domainneighborhood.QualityWarning   `json:"qualityWarnings"`
}

type alternativeComparisonResponse struct {
	Status               domaindecision.AlternativeComparisonStatus `json:"status"`
	RuleVersion          string                                     `json:"ruleVersion"`
	ReferenceCollectedAt string                                     `json:"referenceCollectedAt"`
	SafeTotalPrice       float64                                    `json:"safeTotalPrice"`
	Candidates           []alternativeCandidateComparisonResponse   `json:"candidates"`
}

type alternativeCandidateComparisonResponse struct {
	NeighborhoodID                    string                                          `json:"neighborhoodId"`
	Name                              string                                          `json:"name"`
	Area                              string                                          `json:"area"`
	TargetLayout                      string                                          `json:"targetLayout"`
	Status                            domaindecision.AlternativeCandidateStatus       `json:"status"`
	Reasons                           []domaindecision.AlternativeComparisonReason    `json:"reasons"`
	Improvements                      []domaindecision.AlternativeComparisonDimension `json:"improvements"`
	Deteriorations                    []domaindecision.AlternativeComparisonDimension `json:"deteriorations"`
	WithinBudget                      *bool                                           `json:"withinBudget"`
	TargetTransactionPriceMidpoint    *float64                                        `json:"targetTransactionPriceMidpoint"`
	CandidateTransactionPriceMidpoint *float64                                        `json:"candidateTransactionPriceMidpoint"`
	PriceDifference                   *float64                                        `json:"priceDifference"`
	PriceDifferencePct                *float64                                        `json:"priceDifferencePct"`
	TargetSignal                      *domainneighborhood.NeighborhoodStatus          `json:"targetSignal"`
	CandidateSignal                   *domainneighborhood.NeighborhoodStatus          `json:"candidateSignal"`
	SignalRankDifference              *int                                            `json:"signalRankDifference"`
	TargetLayoutSupply                int                                             `json:"targetLayoutSupply"`
	CandidateTargetLayoutSupply       *int                                            `json:"candidateTargetLayoutSupply"`
	SupplyDifference                  *int                                            `json:"supplyDifference"`
	SupplyDifferencePct               *float64                                        `json:"supplyDifferencePct"`
	Metric                            *decisionMetricReferenceResponse                `json:"metric"`
}

type decisionFactorResponse struct {
	Key      appdecision.FactorKey            `json:"key"`
	Status   appdecision.FactorStatus         `json:"status"`
	Summary  string                           `json:"summary"`
	Source   *decisionFactorSourceResponse    `json:"source"`
	Evidence []decisionFactorEvidenceResponse `json:"evidence"`
}

type decisionFactorSourceResponse struct {
	Type       appdecision.FactorSourceType `json:"type"`
	ID         string                       `json:"id"`
	ObservedAt string                       `json:"observedAt"`
}

type decisionFactorEvidenceResponse struct {
	Key          string                        `json:"key"`
	Label        string                        `json:"label"`
	ValueType    appdecision.EvidenceValueType `json:"valueType"`
	TextValue    *string                       `json:"textValue,omitempty"`
	NumberValue  *float64                      `json:"numberValue,omitempty"`
	BooleanValue *bool                         `json:"booleanValue,omitempty"`
	Unit         string                        `json:"unit,omitempty"`
}

func NewDecision(app DecisionApplication) Decision {
	return Decision{app: app}
}

func (h Decision) GetActionWindow(c *gin.Context) {
	result, err := h.app.GetActionWindow(c.Request.Context(), appdecision.GetActionWindowQuery{
		NeighborhoodID: c.Query("neighborhoodId"),
	})
	if err != nil {
		if errors.Is(err, appdecision.ErrCapacityRequired) {
			writeError(c, http.StatusBadRequest, "capacity_required", "create a capacity calculation before requesting an action window")
			return
		}
		if errors.Is(err, appdecision.ErrWatchlistRequired) {
			writeError(c, http.StatusBadRequest, "watchlist_required", "add a neighborhood to the watchlist before requesting an action window")
			return
		}
		if errors.Is(err, appdecision.ErrInvalidNeighborhoodID) {
			writeError(c, http.StatusBadRequest, "invalid_neighborhood_id", "neighborhoodId must be a valid UUID")
			return
		}
		if errors.Is(err, appdecision.ErrMetricRequired) {
			writeError(c, http.StatusNotFound, "metric_required", "no neighborhood metric is available")
			return
		}
		if errors.Is(err, appdecision.ErrMetricStale) {
			writeError(c, http.StatusConflict, "metric_stale", "the latest neighborhood metric is stale")
			return
		}
		if errors.Is(err, appdecision.ErrMetricInsufficient) {
			writeError(c, http.StatusConflict, "metric_insufficient", "the latest neighborhood metric is insufficient")
			return
		}
		writeError(c, http.StatusInternalServerError, "internal_error", "internal server error")
		return
	}

	c.JSON(http.StatusOK, newActionWindowResponse(result))
}

func newActionWindowResponse(result appdecision.ActionWindowResult) actionWindowResponse {
	factors := make([]decisionFactorResponse, 0, len(result.Factors))
	for _, factor := range result.Factors {
		var source *decisionFactorSourceResponse
		if factor.Source != nil {
			source = &decisionFactorSourceResponse{
				Type:       factor.Source.Type,
				ID:         factor.Source.ID,
				ObservedAt: formatDecisionTime(factor.Source.ObservedAt),
			}
		}
		evidence := make([]decisionFactorEvidenceResponse, 0, len(factor.Evidence))
		for _, value := range factor.Evidence {
			evidence = append(evidence, decisionFactorEvidenceResponse{
				Key:          value.Key,
				Label:        value.Label,
				ValueType:    value.ValueType,
				TextValue:    value.TextValue,
				NumberValue:  value.NumberValue,
				BooleanValue: value.BooleanValue,
				Unit:         value.Unit,
			})
		}
		factors = append(factors, decisionFactorResponse{
			Key: factor.Key, Status: factor.Status, Summary: factor.Summary,
			Source: source, Evidence: evidence,
		})
	}

	return actionWindowResponse{
		Action:            result.Action,
		Confidence:        result.Confidence,
		ConfidenceReasons: append([]string{}, result.ConfidenceReasons...),
		Summary:           result.Summary,
		Target: actionWindowTargetResponse{
			NeighborhoodID: result.Target.NeighborhoodID,
			Name:           result.Target.Name,
			Area:           result.Target.Area,
			TargetLayout:   result.Target.TargetLayout,
		},
		CapacityCalculation: capacityCalculationReferenceResponse{
			ID:                 result.CapacityCalculation.ID,
			CreatedAt:          formatDecisionTime(result.CapacityCalculation.CreatedAt),
			RuleVersion:        result.CapacityCalculation.RuleVersion,
			TraceabilityStatus: result.CapacityCalculation.TraceabilityStatus,
		},
		Metric:                newDecisionMetricReferenceResponse(result.Metric),
		AlternativeComparison: newAlternativeComparisonResponse(result.AlternativeComparison),
		Factors:               factors,
		Checklist:             append([]string{}, result.Checklist...),
		Risks:                 append([]string{}, result.Risks...),
	}
}

func newDecisionMetricReferenceResponse(metric appdecision.DecisionMetricReference) decisionMetricReferenceResponse {
	return decisionMetricReferenceResponse{
		ID:                     metric.ID,
		CollectionRunID:        metric.CollectionRunID,
		AlgorithmVersion:       metric.AlgorithmVersion,
		CollectedAt:            formatDecisionTime(metric.CollectedAt),
		CalculatedAt:           formatDecisionTime(metric.CalculatedAt),
		SourceIDs:              append([]string{}, metric.SourceIDs...),
		ListingSampleCount:     metric.ListingSampleCount,
		TransactionSampleCount: metric.TransactionSampleCount,
		Coverage:               metric.Coverage,
		Freshness:              metric.Freshness,
		QualityState:           metric.QualityState,
		QualityWarnings:        append([]domainneighborhood.QualityWarning{}, metric.QualityWarnings...),
	}
}

func newAlternativeComparisonResponse(comparison appdecision.AlternativeComparisonResult) alternativeComparisonResponse {
	candidates := make([]alternativeCandidateComparisonResponse, 0, len(comparison.Candidates))
	for _, candidate := range comparison.Candidates {
		var metric *decisionMetricReferenceResponse
		if candidate.Metric != nil {
			mapped := newDecisionMetricReferenceResponse(*candidate.Metric)
			metric = &mapped
		}
		candidates = append(candidates, alternativeCandidateComparisonResponse{
			NeighborhoodID:                    candidate.NeighborhoodID,
			Name:                              candidate.Name,
			Area:                              candidate.Area,
			TargetLayout:                      candidate.TargetLayout,
			Status:                            candidate.Status,
			Reasons:                           append([]domaindecision.AlternativeComparisonReason{}, candidate.Reasons...),
			Improvements:                      append([]domaindecision.AlternativeComparisonDimension{}, candidate.Improvements...),
			Deteriorations:                    append([]domaindecision.AlternativeComparisonDimension{}, candidate.Deteriorations...),
			WithinBudget:                      candidate.WithinBudget,
			TargetTransactionPriceMidpoint:    candidate.TargetTransactionPriceMidpoint,
			CandidateTransactionPriceMidpoint: candidate.CandidateTransactionPriceMidpoint,
			PriceDifference:                   candidate.PriceDifference,
			PriceDifferencePct:                candidate.PriceDifferencePct,
			TargetSignal:                      candidate.TargetSignal,
			CandidateSignal:                   candidate.CandidateSignal,
			SignalRankDifference:              candidate.SignalRankDifference,
			TargetLayoutSupply:                candidate.TargetLayoutSupply,
			CandidateTargetLayoutSupply:       candidate.CandidateTargetLayoutSupply,
			SupplyDifference:                  candidate.SupplyDifference,
			SupplyDifferencePct:               candidate.SupplyDifferencePct,
			Metric:                            metric,
		})
	}
	return alternativeComparisonResponse{
		Status:               comparison.Status,
		RuleVersion:          comparison.RuleVersion,
		ReferenceCollectedAt: formatDecisionTime(comparison.ReferenceCollectedAt),
		SafeTotalPrice:       comparison.SafeTotalPrice,
		Candidates:           candidates,
	}
}

func formatDecisionTime(value time.Time) string { return value.UTC().Format(time.RFC3339) }
