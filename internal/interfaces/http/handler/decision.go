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
	Action              domaindecision.ActionWindow          `json:"action"`
	Confidence          domaindecision.Confidence            `json:"confidence"`
	ConfidenceReasons   []string                             `json:"confidenceReasons"`
	Summary             string                               `json:"summary"`
	Target              actionWindowTargetResponse           `json:"target"`
	CapacityCalculation capacityCalculationReferenceResponse `json:"capacityCalculation"`
	Metric              decisionMetricReferenceResponse      `json:"metric"`
	Factors             []decisionFactorResponse             `json:"factors"`
	Checklist           []string                             `json:"checklist"`
	Risks               []string                             `json:"risks"`
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
		Metric: decisionMetricReferenceResponse{
			ID:                     result.Metric.ID,
			CollectionRunID:        result.Metric.CollectionRunID,
			AlgorithmVersion:       result.Metric.AlgorithmVersion,
			CollectedAt:            formatDecisionTime(result.Metric.CollectedAt),
			CalculatedAt:           formatDecisionTime(result.Metric.CalculatedAt),
			SourceIDs:              append([]string{}, result.Metric.SourceIDs...),
			ListingSampleCount:     result.Metric.ListingSampleCount,
			TransactionSampleCount: result.Metric.TransactionSampleCount,
			Coverage:               result.Metric.Coverage,
			Freshness:              result.Metric.Freshness,
			QualityState:           result.Metric.QualityState,
			QualityWarnings:        append([]domainneighborhood.QualityWarning{}, result.Metric.QualityWarnings...),
		},
		Factors:   factors,
		Checklist: append([]string{}, result.Checklist...),
		Risks:     append([]string{}, result.Risks...),
	}
}

func formatDecisionTime(value time.Time) string { return value.UTC().Format(time.RFC3339) }
