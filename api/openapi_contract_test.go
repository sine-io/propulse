package api

import (
	"fmt"
	"os"
	"reflect"
	"sort"
	"testing"

	"github.com/goccy/go-yaml"
)

type operationContract struct {
	path      string
	method    string
	protected bool
}

func TestAccessProtectionContract(t *testing.T) {
	spec := loadOpenAPI(t)

	if _, ok := spec["security"]; ok {
		t.Fatal("document-level security must remain absent")
	}

	components := requiredMap(t, spec, "components")
	securitySchemes := requiredMap(t, components, "securitySchemes")
	accessBearerAuth := requiredMap(t, securitySchemes, "AccessBearerAuth")
	if got := requiredString(t, accessBearerAuth, "type"); got != "http" {
		t.Fatalf("AccessBearerAuth type = %q, want http", got)
	}
	if got := requiredString(t, accessBearerAuth, "scheme"); got != "bearer" {
		t.Fatalf("AccessBearerAuth scheme = %q, want bearer", got)
	}
	if _, ok := securitySchemes["AdminBearerAuth"]; ok {
		t.Fatal("legacy AdminBearerAuth security scheme must be absent")
	}

	contracts := []operationContract{
		{path: "/api/v1/access", method: "get", protected: true},
		{path: "/api/v1/capacity/assumptions", method: "get"},
		{path: "/api/v1/capacity/calculations", method: "post", protected: true},
		{path: "/api/v1/capacity/calculations/{id}", method: "get", protected: true},
		{path: "/api/v1/assets", method: "post", protected: true},
		{path: "/api/v1/assets", method: "get", protected: true},
		{path: "/api/v1/assets/{id}", method: "get", protected: true},
		{path: "/api/v1/assets/{id}", method: "patch", protected: true},
		{path: "/api/v1/assets/{id}", method: "delete", protected: true},
		{path: "/api/v1/neighborhoods", method: "get"},
		{path: "/api/v1/neighborhoods", method: "post", protected: true},
		{path: "/api/v1/neighborhoods/{id}", method: "get"},
		{path: "/api/v1/neighborhoods/{id}/metrics", method: "get"},
		{path: "/api/v1/neighborhoods/{id}/metrics/history", method: "get"},
		{path: "/api/v1/neighborhoods/{id}/community-market", method: "get"},
		{path: "/api/v1/neighborhoods/{id}/community-market/latest", method: "get"},
		{path: "/api/v1/neighborhoods/{id}/market-listings", method: "get"},
		{path: "/api/v1/neighborhoods/{id}/market-listings/{roomId}", method: "get"},
		{path: "/api/v1/neighborhoods/{id}/market-transactions", method: "get"},
		{path: "/api/v1/neighborhoods/{id}/market-listings/{roomId}/adjustments", method: "get"},
		{path: "/api/v1/community-market/comparison", method: "get"},
		{path: "/api/v1/watchlist/items", method: "post", protected: true},
		{path: "/api/v1/watchlist", method: "get", protected: true},
		{path: "/api/v1/review-notes", method: "post", protected: true},
		{path: "/api/v1/review-notes", method: "get", protected: true},
		{path: "/api/v1/review-notes/{id}", method: "get", protected: true},
		{path: "/api/v1/review-notes/{id}", method: "patch", protected: true},
		{path: "/api/v1/decision/action-window", method: "get", protected: true},
		{path: "/admin/api/data-sources", method: "post", protected: true},
		{path: "/admin/api/data-sources", method: "get", protected: true},
		{path: "/admin/api/imports", method: "get", protected: true},
		{path: "/admin/api/imports/json", method: "post", protected: true},
		{path: "/admin/api/imports/csv", method: "post", protected: true},
		{path: "/admin/api/imports/csv/template", method: "get", protected: true},
		{path: "/admin/api/imports/{id}", method: "get", protected: true},
		{path: "/admin/api/capacity/policies", method: "get", protected: true},
		{path: "/admin/api/capacity/policies", method: "post", protected: true},
		{path: "/admin/api/community-market/imports/csv", method: "post", protected: true},
		{path: "/admin/api/community-market/imports/fangjian", method: "post", protected: true},
	}

	paths := requiredMap(t, spec, "paths")
	assertExactOperationTopology(t, paths, contracts)
	responses := requiredMap(t, components, "responses")
	accessRequired := requiredMap(t, responses, "AccessRequired")
	assertAccessRequiredResponse(t, accessRequired)

	for _, contract := range contracts {
		t.Run(fmt.Sprintf("%s %s", contract.method, contract.path), func(t *testing.T) {
			pathItem := requiredMap(t, paths, contract.path)
			operation := requiredMap(t, pathItem, contract.method)
			operationResponses := requiredMap(t, operation, "responses")

			if contract.protected {
				assertSingleAccessSecurity(t, operation)
				unauthorized := requiredMap(t, operationResponses, "401")
				if got := requiredString(t, unauthorized, "$ref"); got != "#/components/responses/AccessRequired" {
					t.Fatalf("401 $ref = %q, want shared AccessRequired response", got)
				}
				if len(unauthorized) != 1 {
					t.Fatalf("401 response must only reference AccessRequired, got keys %v", mapKeys(unauthorized))
				}
				return
			}

			if _, ok := operation["security"]; ok {
				t.Fatal("public operation must not declare security")
			}
			if _, ok := operationResponses["401"]; ok {
				t.Fatal("public operation must not declare a 401 response")
			}
		})
	}

	importOperation := requiredMap(t, requiredMap(t, paths, "/admin/api/imports/json"), "post")
	if importResponses := requiredMap(t, importOperation, "responses"); hasKey(importResponses, "403") {
		t.Fatal("admin import operation must not retain the obsolete 403 response")
	}
}

func TestFangjianCommunityMarketContract(t *testing.T) {
	spec := loadOpenAPI(t)
	paths := requiredMap(t, spec, "paths")
	schemas := requiredMap(t, requiredMap(t, spec, "components"), "schemas")

	for path, responseSchema := range map[string]string{
		"/api/v1/neighborhoods/{id}/community-market/latest":              "#/components/schemas/CommunityMarketSnapshot",
		"/api/v1/neighborhoods/{id}/market-listings":                      "#/components/schemas/MarketListingsPage",
		"/api/v1/neighborhoods/{id}/market-listings/{roomId}":             "#/components/schemas/MarketListingDetail",
		"/api/v1/neighborhoods/{id}/market-transactions":                  "#/components/schemas/MarketTransactionsPage",
		"/api/v1/neighborhoods/{id}/market-listings/{roomId}/adjustments": "#/components/schemas/ListingAdjustmentsResponse",
		"/api/v1/community-market/comparison":                             "#/components/schemas/CommunityMarketComparison",
	} {
		operation := requiredMap(t, requiredMap(t, paths, path), "get")
		if got := requiredString(t, responseJSONSchema(t, operation, "200"), "$ref"); got != responseSchema {
			t.Fatalf("%s response schema = %q, want %q", path, got, responseSchema)
		}
	}

	importOperation := requiredMap(t, requiredMap(t, paths, "/admin/api/community-market/imports/fangjian"), "post")
	if got := requiredString(t, responseJSONSchema(t, importOperation, "201"), "$ref"); got != "#/components/schemas/ImportFangjianResponse" {
		t.Fatalf("Fangjian import response schema = %q", got)
	}
	for _, status := range []string{"200", "201", "400", "401", "404", "413", "422", "500"} {
		if _, ok := requiredMap(t, importOperation, "responses")[status]; !ok {
			t.Fatalf("Fangjian import is missing %s response", status)
		}
	}
	assertRequiredFields(t, requiredMap(t, schemas, "FangjianBundle"), []string{"schemaVersion", "collectedAt", "community", "listings", "transactions", "adjustments", "quality"})
	assertRequiredFields(t, requiredMap(t, schemas, "CommunityMarketSnapshot"), []string{"collectionRunId", "qualityStatus", "analysis", "surroundings", "cityContext"})
	assertRequiredFields(t, requiredMap(t, schemas, "MarketListing"), []string{"roomId", "listedAt", "daysOnMarket", "adjustmentCount"})
	assertRequiredFields(t, requiredMap(t, schemas, "MarketTransaction"), []string{"listingTotalPriceWan", "tradeTotalPriceWan", "negotiationWan", "orientation"})
}

func TestPropertyAssetAndCalculationSelectionContracts(t *testing.T) {
	spec := loadOpenAPI(t)
	components := requiredMap(t, spec, "components")
	schemas := requiredMap(t, components, "schemas")
	paths := requiredMap(t, spec, "paths")

	createAsset := requiredMap(t, requiredMap(t, paths, "/api/v1/assets"), "post")
	requestBody := requiredMap(t, createAsset, "requestBody")
	content := requiredMap(t, requestBody, "content")
	mediaType := requiredMap(t, content, "application/json")
	if got := requiredString(t, requiredMap(t, mediaType, "schema"), "$ref"); got != "#/components/schemas/CreateAssetRequest" {
		t.Fatalf("asset create request schema = %q", got)
	}
	if got := requiredString(t, responseJSONSchema(t, createAsset, "201"), "$ref"); got != "#/components/schemas/AssetResponse" {
		t.Fatalf("asset create response schema = %q", got)
	}
	request := requiredMap(t, schemas, "CreateAssetRequest")
	assertRequiredFields(t, request, []string{"neighborhoodId", "propertySelection", "originalPurchasePriceWan", "purchasedOn", "currentLoanBalanceWan"})
	if additional, ok := request["additionalProperties"].(bool); !ok || additional {
		t.Fatalf("CreateAssetRequest additionalProperties = %#v, want false", request["additionalProperties"])
	}
	if _, exists := requiredMap(t, request, "properties")["userId"]; exists {
		t.Fatal("CreateAssetRequest must not expose userId")
	}
	asset := requiredMap(t, schemas, "AssetResponse")
	assertRequiredFields(t, asset, []string{"id", "property", "sourceKind", "listingSource", "createdAt", "updatedAt"})
	if _, exists := requiredMap(t, asset, "properties")["userId"]; exists {
		t.Fatal("AssetResponse must not expose userId")
	}

	capacityInput := requiredMap(t, schemas, "HousingCapacityInput")
	inputProperties := requiredMap(t, capacityInput, "properties")
	for _, field := range []string{"oldHomeSelection", "targetHomeSelection"} {
		if inputProperties[field] == nil {
			t.Fatalf("HousingCapacityInput is missing %s", field)
		}
	}
	calculation := requiredMap(t, schemas, "CalculationResponse")
	if requiredMap(t, requiredMap(t, calculation, "properties"), "selectionContext")["$ref"] != "#/components/schemas/PropertySelectionContext" {
		t.Fatal("CalculationResponse selectionContext must reference the frozen context schema")
	}
}

func TestReviewNotesContract(t *testing.T) {
	spec := loadOpenAPI(t)
	paths := requiredMap(t, spec, "paths")
	schemas := requiredMap(t, requiredMap(t, spec, "components"), "schemas")

	collectionPath := requiredMap(t, paths, "/api/v1/review-notes")
	create := requiredMap(t, collectionPath, "post")
	list := requiredMap(t, collectionPath, "get")
	notePath := requiredMap(t, paths, "/api/v1/review-notes/{id}")
	get := requiredMap(t, notePath, "get")
	update := requiredMap(t, notePath, "patch")

	for _, operation := range []map[string]interface{}{create, list, get, update} {
		responses := requiredMap(t, operation, "responses")
		for _, status := range []string{"400", "401", "500"} {
			if _, ok := responses[status]; !ok {
				t.Fatalf("review operation responses = %v, missing %s", mapKeys(responses), status)
			}
		}
	}
	for _, operation := range []map[string]interface{}{create, get, update} {
		if _, ok := requiredMap(t, operation, "responses")["404"]; !ok {
			t.Fatal("review create/get/update operation is missing 404 response")
		}
	}
	if got := requiredString(t, responseJSONSchema(t, create, "201"), "$ref"); got != "#/components/schemas/ReviewNoteResponse" {
		t.Fatalf("create response schema = %q", got)
	}
	if got := requiredString(t, responseJSONSchema(t, list, "200"), "$ref"); got != "#/components/schemas/ReviewNotesPageResponse" {
		t.Fatalf("list response schema = %q", got)
	}
	for _, operation := range []map[string]interface{}{get, update} {
		if got := requiredString(t, responseJSONSchema(t, operation, "200"), "$ref"); got != "#/components/schemas/ReviewNoteResponse" {
			t.Fatalf("single-note response schema = %q", got)
		}
		id := operationParameter(t, operation, "id", "path")
		if id == nil || requiredString(t, requiredMap(t, id, "schema"), "format") != "uuid" {
			t.Fatalf("review id parameter = %#v, want UUID", id)
		}
	}

	page := operationParameter(t, list, "page", "query")
	pageSize := operationParameter(t, list, "pageSize", "query")
	if page == nil || fmt.Sprint(requiredMap(t, page, "schema")["default"]) != "1" {
		t.Fatalf("page parameter = %#v, want default 1", page)
	}
	if pageSize == nil {
		t.Fatal("pageSize parameter is missing")
	}
	pageSizeSchema := requiredMap(t, pageSize, "schema")
	if fmt.Sprint(pageSizeSchema["default"]) != "20" || fmt.Sprint(pageSizeSchema["maximum"]) != "100" {
		t.Fatalf("pageSize schema = %#v, want default 20 maximum 100", pageSizeSchema)
	}

	createRequest := requiredMap(t, schemas, "CreateReviewNoteRequest")
	assertRequiredFields(t, createRequest, []string{"kind", "content"})
	if additional, ok := createRequest["additionalProperties"].(bool); !ok || additional {
		t.Fatalf("CreateReviewNoteRequest additionalProperties = %#v, want false", createRequest["additionalProperties"])
	}
	createProperties := requiredMap(t, createRequest, "properties")
	if _, exists := createProperties["userId"]; exists {
		t.Fatal("CreateReviewNoteRequest must not expose userId")
	}
	if got := requiredString(t, requiredMap(t, createProperties, "weekStartDate"), "format"); got != "date" {
		t.Fatalf("weekStartDate format = %q, want date", got)
	}
	content := requiredMap(t, createProperties, "content")
	if fmt.Sprint(content["minLength"]) != "1" || fmt.Sprint(content["maxLength"]) != "8000" {
		t.Fatalf("review content limits = %#v", content)
	}

	updateRequest := requiredMap(t, schemas, "UpdateReviewNoteRequest")
	assertRequiredFields(t, updateRequest, []string{"content"})
	updateProperties := requiredMap(t, updateRequest, "properties")
	if len(updateProperties) != 1 || updateProperties["content"] == nil {
		t.Fatalf("UpdateReviewNoteRequest properties = %v, want only content", mapKeys(updateProperties))
	}
	if additional, ok := updateRequest["additionalProperties"].(bool); !ok || additional {
		t.Fatalf("UpdateReviewNoteRequest additionalProperties = %#v, want false", updateRequest["additionalProperties"])
	}

	note := requiredMap(t, schemas, "ReviewNoteResponse")
	assertRequiredFields(t, note, []string{"id", "kind", "neighborhoodId", "weekStartDate", "content", "createdAt", "updatedAt"})
	noteProperties := requiredMap(t, note, "properties")
	if _, exists := noteProperties["userId"]; exists {
		t.Fatal("ReviewNoteResponse must not expose userId")
	}
	for _, field := range []string{"neighborhoodId", "weekStartDate"} {
		property := requiredMap(t, noteProperties, field)
		if nullable, ok := property["nullable"].(bool); !ok || !nullable {
			t.Fatalf("ReviewNoteResponse.%s nullable = %#v, want true", field, property["nullable"])
		}
	}
	assertStringEnumContains(t, requiredMap(t, schemas, "ReviewNoteKind"), "review")
	assertStringEnumContains(t, requiredMap(t, schemas, "ReviewNoteKind"), "viewing_note")
	assertRequiredFields(t, requiredMap(t, schemas, "ReviewNotesPageResponse"), []string{"items", "total", "page", "pageSize"})
}

func TestMetricHistoryAndQualityContracts(t *testing.T) {
	spec := loadOpenAPI(t)
	components := requiredMap(t, spec, "components")
	schemas := requiredMap(t, components, "schemas")
	paths := requiredMap(t, spec, "paths")

	latest := requiredMap(t, schemas, "NeighborhoodMetricResponse")
	assertRequiredFields(t, latest, []string{"collectedAt", "algorithmVersion", "transactionEvidence", "calculatedAt"})
	assertRequiredFields(t, requiredMap(t, schemas, "TransactionMomentumEvidence"), []string{
		"windowStart", "windowEnd", "sampleCount", "recent30DayTransactionCount",
		"preceding60DayTransactionCount", "recent30DayMonthlyFrequency", "preceding60DayMonthlyFrequency",
	})

	historyOperation := requiredMap(t, requiredMap(t, paths, "/api/v1/neighborhoods/{id}/metrics/history"), "get")
	if got := requiredString(t, responseJSONSchema(t, historyOperation, "200"), "$ref"); got != "#/components/schemas/MetricHistoryResponse" {
		t.Fatalf("history response schema = %q", got)
	}
	assertRequiredFields(t, requiredMap(t, schemas, "MetricHistoryResponse"), []string{"status", "neighborhoodId", "algorithmVersion", "window", "items"})
	assertRequiredFields(t, requiredMap(t, schemas, "MetricHistoryPoint"), []string{"batch", "collectedAt", "calculatedAt", "weeklyComparison", "monthlyComparison"})
	assertRequiredFields(t, requiredMap(t, schemas, "MetricComparison"), []string{"status", "currentBatch"})
	assertRequiredFields(t, requiredMap(t, schemas, "MetricChangeValue"), []string{"absoluteChange", "percentageChange", "percentageStatus"})

	watchlist := requiredMap(t, schemas, "WatchlistItem")
	assertRequiredFields(t, watchlist, []string{"hasMetric", "collectedAt", "transactionSampleCount", "coverage", "freshness", "qualityState", "qualityWarnings", "sourceIds", "weeklyComparison"})
	watchlistProperties := requiredMap(t, watchlist, "properties")
	assertStringEnumContains(t, requiredMap(t, watchlistProperties, "transactionMomentum"), "unknown")
	assertStringEnumContains(t, requiredMap(t, watchlistProperties, "status"), "数据不足")
	for _, field := range []string{"listedHomes", "priceCutHomes", "transactionMomentum", "transactionSampleCount", "weeklyComparison"} {
		property := requiredMap(t, watchlistProperties, field)
		if nullable, ok := property["nullable"].(bool); !ok || !nullable {
			t.Fatalf("WatchlistItem.%s nullable = %#v, want true", field, property["nullable"])
		}
	}

	actionResponses := requiredMap(t, requiredMap(t, requiredMap(t, paths, "/api/v1/decision/action-window"), "get"), "responses")
	if _, ok := actionResponses["409"]; !ok {
		t.Fatal("action-window contract is missing metric stale/insufficient response")
	}
}

func assertStringEnumContains(t *testing.T, schema map[string]interface{}, want string) {
	t.Helper()
	values, ok := schema["enum"].([]interface{})
	if !ok {
		t.Fatalf("enum = %#v, want list", schema["enum"])
	}
	for _, value := range values {
		if value == want {
			return
		}
	}
	t.Fatalf("enum = %#v, missing %q", values, want)
}

func TestActionWindowEvidenceContract(t *testing.T) {
	spec := loadOpenAPI(t)
	components := requiredMap(t, spec, "components")
	schemas := requiredMap(t, components, "schemas")
	paths := requiredMap(t, spec, "paths")

	operation := requiredMap(t, requiredMap(t, paths, "/api/v1/decision/action-window"), "get")
	if got := requiredString(t, responseJSONSchema(t, operation, "200"), "$ref"); got != "#/components/schemas/ActionWindowResponse" {
		t.Fatalf("action-window response schema = %q", got)
	}
	response := requiredMap(t, schemas, "ActionWindowResponse")
	assertRequiredFields(t, response, []string{
		"action", "confidence", "confidenceReasons", "summary", "target",
		"capacityCalculation", "metric", "alternativeComparison", "factors", "checklist", "risks",
	})
	assertRequiredFields(t, requiredMap(t, schemas, "ActionWindowTarget"), []string{"neighborhoodId", "name", "area", "targetLayout"})
	assertRequiredFields(t, requiredMap(t, schemas, "CapacityCalculationReference"), []string{"id", "createdAt", "ruleVersion", "traceabilityStatus"})
	assertRequiredFields(t, requiredMap(t, schemas, "DecisionMetricReference"), []string{
		"id", "collectionRunId", "algorithmVersion", "collectedAt", "calculatedAt", "sourceIds",
		"listingSampleCount", "transactionSampleCount", "coverage", "freshness", "qualityState", "qualityWarnings",
	})

	factor := requiredMap(t, schemas, "DecisionFactor")
	assertRequiredFields(t, factor, []string{"key", "status", "summary", "source", "evidence"})
	factorProperties := requiredMap(t, factor, "properties")
	for _, key := range []string{
		"budget_pressure", "down_payment_gap", "market_signal", "transaction_momentum", "target_layout_supply", "alternatives",
	} {
		assertStringEnumContains(t, requiredMap(t, factorProperties, "key"), key)
	}
	for _, status := range []string{"positive", "neutral", "caution", "negative", "unknown"} {
		assertStringEnumContains(t, requiredMap(t, factorProperties, "status"), status)
	}
	sourceProperty := requiredMap(t, factorProperties, "source")
	if nullable, ok := sourceProperty["nullable"].(bool); !ok || !nullable {
		t.Fatalf("DecisionFactor.source nullable = %#v, want true", sourceProperty["nullable"])
	}
	source := requiredMap(t, schemas, "DecisionFactorSource")
	assertRequiredFields(t, source, []string{"type", "id", "observedAt"})
	for _, sourceType := range []string{"capacity_calculation", "neighborhood_metric", "alternative_comparison"} {
		assertStringEnumContains(t, requiredMap(t, requiredMap(t, source, "properties"), "type"), sourceType)
	}
	evidence := requiredMap(t, schemas, "DecisionFactorEvidence")
	assertRequiredFields(t, evidence, []string{"key", "label", "valueType"})
	for _, valueType := range []string{"text", "number", "boolean"} {
		assertStringEnumContains(t, requiredMap(t, requiredMap(t, evidence, "properties"), "valueType"), valueType)
	}

	alternative := requiredMap(t, schemas, "AlternativeComparison")
	assertRequiredFields(t, alternative, []string{"status", "ruleVersion", "referenceCollectedAt", "safeTotalPrice", "candidates"})
	for _, status := range []string{"better_found", "none", "unknown"} {
		assertStringEnumContains(t, requiredMap(t, requiredMap(t, alternative, "properties"), "status"), status)
	}
	candidate := requiredMap(t, schemas, "AlternativeCandidateComparison")
	assertRequiredFields(t, candidate, []string{
		"neighborhoodId", "name", "area", "targetLayout", "status", "reasons", "improvements", "deteriorations",
		"withinBudget", "targetTransactionPriceMidpoint", "candidateTransactionPriceMidpoint", "priceDifference",
		"priceDifferencePct", "targetSignal", "candidateSignal", "signalRankDifference", "targetLayoutSupply",
		"candidateTargetLayoutSupply", "supplyDifference", "supplyDifferencePct", "metric",
	})
	candidateProperties := requiredMap(t, candidate, "properties")
	for _, status := range []string{"better", "not_better", "unknown"} {
		assertStringEnumContains(t, requiredMap(t, candidateProperties, "status"), status)
	}
	for _, field := range []string{
		"withinBudget", "targetTransactionPriceMidpoint", "candidateTransactionPriceMidpoint", "priceDifference",
		"priceDifferencePct", "targetSignal", "candidateSignal", "signalRankDifference", "candidateTargetLayoutSupply",
		"supplyDifference", "supplyDifferencePct", "metric",
	} {
		property := requiredMap(t, candidateProperties, field)
		if nullable, ok := property["nullable"].(bool); !ok || !nullable {
			t.Fatalf("AlternativeCandidateComparison.%s nullable = %#v, want true", field, property["nullable"])
		}
	}
}

func TestActionWindowRequiresWatchedNeighborhoodSelection(t *testing.T) {
	spec := loadOpenAPI(t)
	paths := requiredMap(t, spec, "paths")
	operation := requiredMap(t, requiredMap(t, paths, "/api/v1/decision/action-window"), "get")

	rawParameters, ok := operation["parameters"].([]interface{})
	if !ok {
		t.Fatalf("action-window parameters = %#v, want list", operation["parameters"])
	}
	var neighborhoodID map[string]interface{}
	for _, rawParameter := range rawParameters {
		parameter, ok := rawParameter.(map[string]interface{})
		if ok && parameter["name"] == "neighborhoodId" && parameter["in"] == "query" {
			neighborhoodID = parameter
			break
		}
	}
	if neighborhoodID == nil {
		t.Fatal("action-window neighborhoodId query parameter is missing")
	}
	if required, ok := neighborhoodID["required"].(bool); !ok || !required {
		t.Fatalf("neighborhoodId required = %#v, want true", neighborhoodID["required"])
	}
	schema := requiredMap(t, neighborhoodID, "schema")
	if got := requiredString(t, schema, "type"); got != "string" {
		t.Fatalf("neighborhoodId type = %q, want string", got)
	}
	if got := requiredString(t, schema, "format"); got != "uuid" {
		t.Fatalf("neighborhoodId format = %q, want uuid", got)
	}

	responses := requiredMap(t, operation, "responses")
	badRequest := requiredMap(t, responses, "400")
	jsonContent := requiredMap(t, requiredMap(t, badRequest, "content"), "application/json")
	examples := requiredMap(t, jsonContent, "examples")
	for _, name := range []string{"capacityRequired", "watchlistRequired", "invalidNeighborhoodID", "neighborhoodNotWatched"} {
		if _, ok := examples[name]; !ok {
			t.Fatalf("action-window 400 examples missing %q", name)
		}
	}
}

func TestNeighborhoodCatalogAndWatchlistTargetContract(t *testing.T) {
	spec := loadOpenAPI(t)
	paths := requiredMap(t, spec, "paths")
	schemas := requiredMap(t, requiredMap(t, spec, "components"), "schemas")

	search := requiredMap(t, requiredMap(t, paths, "/api/v1/neighborhoods"), "get")
	for _, name := range []string{"city", "area", "targetLayout", "q", "page", "pageSize"} {
		if operationParameter(t, search, name, "query") == nil {
			t.Fatalf("neighborhood search parameter %q is missing", name)
		}
	}
	searchResponse := requiredMap(t, schemas, "NeighborhoodSearchResponse")
	assertRequiredFields(t, searchResponse, []string{"items", "total", "page", "pageSize", "filters"})
	assertRequiredFields(t, requiredMap(t, schemas, "NeighborhoodSearchFilters"), []string{"cities", "areas"})
	assertRequiredFields(t, requiredMap(t, schemas, "NeighborhoodAreaFilter"), []string{"city", "area"})

	create := requiredMap(t, schemas, "CreateNeighborhoodRequest")
	assertRequiredFields(t, create, []string{"city", "area", "name", "availableLayouts"})
	neighborhood := requiredMap(t, schemas, "NeighborhoodResponse")
	assertRequiredFields(t, neighborhood, []string{"id", "city", "area", "name", "availableLayouts"})
	if _, ok := requiredMap(t, neighborhood, "properties")["targetLayout"]; ok {
		t.Fatal("NeighborhoodResponse must not retain a default targetLayout")
	}

	latest := requiredMap(t, requiredMap(t, paths, "/api/v1/neighborhoods/{id}/metrics"), "get")
	assertRequiredOperationParameter(t, latest, "targetLayout")
	assertRequiredFields(t, requiredMap(t, schemas, "NeighborhoodMetricResponse"), []string{"targetLayout", "targetLayoutSupply"})
	history := requiredMap(t, requiredMap(t, paths, "/api/v1/neighborhoods/{id}/metrics/history"), "get")
	assertRequiredOperationParameter(t, history, "targetLayout")
	assertRequiredFields(t, requiredMap(t, schemas, "MetricHistoryResponse"), []string{"targetLayout"})
	assertRequiredFields(t, requiredMap(t, schemas, "MetricHistoryPoint"), []string{"targetLayoutSupply"})

	add := requiredMap(t, requiredMap(t, paths, "/api/v1/watchlist/items"), "post")
	addRequest := requiredMap(t, schemas, "AddWatchlistItemRequest")
	assertRequiredFields(t, addRequest, []string{"neighborhoodId", "targetLayout"})
	if got := requiredString(t, requiredMap(t, requiredMap(t, addRequest, "properties"), "neighborhoodId"), "format"); got != "uuid" {
		t.Fatalf("AddWatchlistItemRequest.neighborhoodId format = %q, want uuid", got)
	}
	assertRequiredFields(t, requiredMap(t, schemas, "AddWatchlistItemResponse"), []string{"neighborhoodId", "targetLayout"})
	if _, ok := requiredMap(t, add, "responses")["409"]; !ok {
		t.Fatal("watchlist create contract is missing duplicate conflict response")
	}
	assertRequiredFields(t, requiredMap(t, schemas, "WatchlistItem"), []string{"city", "targetLayout"})
}

func TestCommunityMarketProfileContract(t *testing.T) {
	spec := loadOpenAPI(t)
	paths := requiredMap(t, spec, "paths")
	schemas := requiredMap(t, requiredMap(t, spec, "components"), "schemas")
	profileFields := []string{
		"provinceCode", "provinceName", "propertyType", "propertyTags", "buildingCount", "buildingType",
		"buildingYear", "developer", "householdCount", "closedManagement", "plotRatio", "greenAreaSqm",
		"greeningRatePercent", "propertyManagementCompany", "propertyFee", "fixedParkingSpaces", "parkingRatio",
		"parkingFee", "heatingType", "waterType", "electricityType", "gasCost", "manCarSeparation",
	}
	snapshot := requiredMap(t, schemas, "CommunityMarketSnapshot")
	assertRequiredFields(t, snapshot, profileFields)
	properties := requiredMap(t, snapshot, "properties")
	for _, field := range profileFields {
		property := requiredMap(t, properties, field)
		if nullable, ok := property["nullable"].(bool); !ok || !nullable {
			t.Fatalf("CommunityMarketSnapshot.%s nullable = %#v, want true", field, property["nullable"])
		}
	}

	latest := requiredMap(t, requiredMap(t, paths, "/api/v1/neighborhoods/{id}/community-market"), "get")
	for _, status := range []string{"200", "404", "500"} {
		if _, ok := requiredMap(t, latest, "responses")[status]; !ok {
			t.Fatalf("community market query is missing %s response", status)
		}
	}
	importCSV := requiredMap(t, requiredMap(t, paths, "/admin/api/community-market/imports/csv"), "post")
	for _, status := range []string{"200", "201", "400", "401", "404", "413", "422", "500"} {
		if _, ok := requiredMap(t, importCSV, "responses")[status]; !ok {
			t.Fatalf("community market import is missing %s response", status)
		}
	}
}

func assertRequiredOperationParameter(t *testing.T, operation map[string]interface{}, name string) {
	t.Helper()
	parameter := operationParameter(t, operation, name, "query")
	if parameter == nil {
		t.Fatalf("required query parameter %q is missing", name)
	}
	if required, ok := parameter["required"].(bool); !ok || !required {
		t.Fatalf("query parameter %q required = %#v, want true", name, parameter["required"])
	}
}

func operationParameter(t *testing.T, operation map[string]interface{}, name, location string) map[string]interface{} {
	t.Helper()
	rawParameters, ok := operation["parameters"].([]interface{})
	if !ok {
		t.Fatalf("operation parameters = %#v, want list", operation["parameters"])
	}
	for _, rawParameter := range rawParameters {
		parameter, ok := rawParameter.(map[string]interface{})
		if ok && parameter["name"] == name && parameter["in"] == location {
			return parameter
		}
	}
	return nil
}

func TestCapacityCalculationContract(t *testing.T) {
	spec := loadOpenAPI(t)
	components := requiredMap(t, spec, "components")
	schemas := requiredMap(t, components, "schemas")
	paths := requiredMap(t, spec, "paths")

	input := requiredMap(t, schemas, "HousingCapacityInput")
	assertRequiredFields(t, input, []string{
		"cashOnHand", "oldHomeValue", "oldLoanBalance", "monthlyIncome", "currentMonthlyMortgage",
		"acceptableMonthlyMortgage", "targetTotalPrice", "renovationBudget", "transitionRentCost",
	})
	inputProperties := requiredMap(t, input, "properties")
	for field, ref := range map[string]string{
		"transactionScenario": "#/components/schemas/TransactionScenario",
		"loanPlan":            "#/components/schemas/LoanPlan",
		"manualOverrides":     "#/components/schemas/CalculationOverrides",
	} {
		if got := requiredString(t, requiredMap(t, inputProperties, field), "$ref"); got != ref {
			t.Fatalf("%s $ref = %q, want %q", field, got, ref)
		}
	}
	if got := requiredString(t, requiredMap(t, inputProperties, "cityPolicyOverride"), "$ref"); got != "#/components/schemas/CityPolicyOverride" {
		t.Fatalf("cityPolicyOverride $ref = %q", got)
	}

	policy := requiredMap(t, schemas, "CityPolicyOverride")
	assertRequiredFields(t, policy, []string{"city", "policyName", "downPaymentRate", "effectiveDate", "source"})
	loan := requiredMap(t, schemas, "LoanParams")
	assertRequiredFields(t, loan, []string{"annualInterestRate", "loanTermMonths", "repaymentMethod"})

	result := requiredMap(t, schemas, "HousingCapacityResult")
	assertRequiredFields(t, result, []string{"traceabilityStatus", "appliedAssumptions", "ruleVersion", "effectiveDate"})
	resultProperties := requiredMap(t, result, "properties")
	for _, field := range []string{"loanBreakdown", "taxBreakdown", "policyVersion", "sources", "manualOverrides", "disclaimer"} {
		if _, ok := resultProperties[field]; !ok {
			t.Fatalf("HousingCapacityResult is missing %s", field)
		}
	}
	applied := requiredMap(t, resultProperties, "appliedAssumptions")
	if nullable, ok := applied["nullable"].(bool); !ok || !nullable {
		t.Fatalf("appliedAssumptions nullable = %#v, want true", applied["nullable"])
	}

	createSchema := responseJSONSchema(t, requiredMap(t, requiredMap(t, paths, "/api/v1/capacity/calculations"), "post"), "201")
	if got := requiredString(t, createSchema, "$ref"); got != "#/components/schemas/CreateCalculationResponse" {
		t.Fatalf("POST response schema = %q", got)
	}
	createResponse := requiredMap(t, schemas, "CreateCalculationResponse")
	allOf, ok := createResponse["allOf"].([]interface{})
	if !ok || len(allOf) != 1 {
		t.Fatalf("CreateCalculationResponse allOf = %#v", createResponse["allOf"])
	}
	createRef, ok := allOf[0].(map[string]interface{})
	if !ok || requiredString(t, createRef, "$ref") != "#/components/schemas/CalculationResponse" {
		t.Fatalf("CreateCalculationResponse = %#v, want CalculationResponse alias", createResponse)
	}

	getSchema := responseJSONSchema(t, requiredMap(t, requiredMap(t, paths, "/api/v1/capacity/calculations/{id}"), "get"), "200")
	if got := requiredString(t, getSchema, "$ref"); got != "#/components/schemas/CalculationResponse" {
		t.Fatalf("GET response schema = %q", got)
	}
	assertRequiredFields(t, requiredMap(t, schemas, "CalculationResponse"), []string{"id", "input", "result", "createdAt"})
	assertRequiredFields(t, requiredMap(t, schemas, "AppliedAssumptions"), []string{
		"ruleVersion", "effectiveDate", "ruleSource", "loan", "loanSource", "loanOrigin", "cityPolicy",
		"reserveMonths", "pressureThresholds", "oldHomeShareThreshold",
	})
}

func TestTrustedImportContract(t *testing.T) {
	spec := loadOpenAPI(t)
	components := requiredMap(t, spec, "components")
	schemas := requiredMap(t, components, "schemas")
	paths := requiredMap(t, spec, "paths")

	response := requiredMap(t, schemas, "ImportCollectionRunResponse")
	assertRequiredFields(t, response, []string{
		"collectionRunId", "acceptedRecordCount", "rejectedRecordCount", "collectionRun",
		"listingObservationCount", "transactionObservationCount", "idempotentReplay", "metricRefreshStatus",
	})
	validation := requiredMap(t, schemas, "ImportValidationErrorResponse")
	assertRequiredFields(t, validation, []string{"error", "acceptedRecordCount", "rejectedRecordCount"})

	csvOperation := requiredMap(t, requiredMap(t, paths, "/admin/api/imports/csv"), "post")
	requestBody := requiredMap(t, csvOperation, "requestBody")
	multipartContent := requiredMap(t, requiredMap(t, requestBody, "content"), "multipart/form-data")
	multipartSchema := requiredMap(t, multipartContent, "schema")
	assertRequiredFields(t, multipartSchema, []string{
		"dataSourceId", "neighborhoodId", "sourceRef", "collectedAt", "coverage", "file",
	})
	if got := requiredString(t, requiredMap(t, requiredMap(t, multipartSchema, "properties"), "file"), "format"); got != "binary" {
		t.Fatalf("CSV file format = %q, want binary", got)
	}
	for _, status := range []string{"200", "201"} {
		schema := responseJSONSchema(t, csvOperation, status)
		if got := requiredString(t, schema, "$ref"); got != "#/components/schemas/ImportCollectionRunResponse" {
			t.Fatalf("CSV %s response schema = %q", status, got)
		}
	}
	if got := requiredString(t, responseJSONSchema(t, csvOperation, "422"), "$ref"); got != "#/components/schemas/ImportValidationErrorResponse" {
		t.Fatalf("CSV 422 schema = %q", got)
	}
}

func loadOpenAPI(t *testing.T) map[string]interface{} {
	t.Helper()
	contents, err := os.ReadFile("openapi.yaml")
	if err != nil {
		t.Fatalf("read openapi.yaml: %v", err)
	}

	var spec map[string]interface{}
	if err := yaml.Unmarshal(contents, &spec); err != nil {
		t.Fatalf("parse openapi.yaml: %v", err)
	}
	return spec
}

func assertExactOperationTopology(t *testing.T, paths map[string]interface{}, contracts []operationContract) {
	t.Helper()
	want := make(map[string]struct{}, len(contracts))
	for _, contract := range contracts {
		want[operationKey(contract.path, contract.method)] = struct{}{}
	}

	got := make(map[string]struct{})
	for path, rawPathItem := range paths {
		for method := range requiredHTTPMethods(t, rawPathItem) {
			got[operationKey(path, method)] = struct{}{}
		}
	}
	if !reflect.DeepEqual(sortedKeys(got), sortedKeys(want)) {
		t.Fatalf("operation topology = %v, want %v", sortedKeys(got), sortedKeys(want))
	}
}

func assertSingleAccessSecurity(t *testing.T, operation map[string]interface{}) {
	t.Helper()
	rawSecurity, ok := operation["security"]
	if !ok {
		t.Fatal("protected operation must declare security")
	}
	security, ok := rawSecurity.([]interface{})
	if !ok || len(security) != 1 {
		t.Fatalf("security = %#v, want one requirement", rawSecurity)
	}
	requirement, ok := security[0].(map[string]interface{})
	if !ok || len(requirement) != 1 {
		t.Fatalf("security requirement = %#v, want only AccessBearerAuth", security[0])
	}
	rawSchemes, ok := requirement["AccessBearerAuth"]
	if !ok {
		t.Fatalf("security requirement = %#v, want AccessBearerAuth", requirement)
	}
	schemes, ok := rawSchemes.([]interface{})
	if !ok || len(schemes) != 0 {
		t.Fatalf("AccessBearerAuth requirement = %#v, want empty scopes", rawSchemes)
	}
}

func assertAccessRequiredResponse(t *testing.T, response map[string]interface{}) {
	t.Helper()
	content := requiredMap(t, response, "content")
	jsonContent := requiredMap(t, content, "application/json")
	schema := requiredMap(t, jsonContent, "schema")
	if got := requiredString(t, schema, "$ref"); got != "#/components/schemas/ErrorResponse" {
		t.Fatalf("AccessRequired schema ref = %q, want ErrorResponse", got)
	}
	if len(schema) != 1 {
		t.Fatalf("AccessRequired schema must only reference ErrorResponse, got keys %v", mapKeys(schema))
	}

	example := requiredMap(t, jsonContent, "example")
	wantExample := map[string]interface{}{
		"error": map[string]interface{}{
			"code":    "unauthorized",
			"message": "valid bearer access token is required",
		},
	}
	if !reflect.DeepEqual(example, wantExample) {
		t.Fatalf("AccessRequired example = %#v, want %#v", example, wantExample)
	}

	headers := requiredMap(t, response, "headers")
	challenge := requiredMap(t, headers, "WWW-Authenticate")
	if got := requiredString(t, challenge, "example"); got != "Bearer" {
		t.Fatalf("WWW-Authenticate example = %q, want Bearer", got)
	}
	challengeSchema := requiredMap(t, challenge, "schema")
	if got := requiredString(t, challengeSchema, "type"); got != "string" {
		t.Fatalf("WWW-Authenticate schema type = %q, want string", got)
	}
	if got := requiredString(t, challengeSchema, "example"); got != "Bearer" {
		t.Fatalf("WWW-Authenticate schema example = %q, want Bearer", got)
	}
}

func assertRequiredFields(t *testing.T, schema map[string]interface{}, fields []string) {
	t.Helper()
	raw, ok := schema["required"].([]interface{})
	if !ok {
		t.Fatalf("schema required = %#v, want list", schema["required"])
	}
	required := make(map[string]struct{}, len(raw))
	for _, item := range raw {
		field, ok := item.(string)
		if !ok {
			t.Fatalf("required item = %#v, want string", item)
		}
		required[field] = struct{}{}
	}
	for _, field := range fields {
		if _, ok := required[field]; !ok {
			t.Fatalf("required fields = %v, missing %q", sortedKeys(required), field)
		}
	}
}

func responseJSONSchema(t *testing.T, operation map[string]interface{}, status string) map[string]interface{} {
	t.Helper()
	responses := requiredMap(t, operation, "responses")
	response := requiredMap(t, responses, status)
	content := requiredMap(t, response, "content")
	jsonContent := requiredMap(t, content, "application/json")
	return requiredMap(t, jsonContent, "schema")
}

func requiredHTTPMethods(t *testing.T, rawPathItem interface{}) map[string]interface{} {
	t.Helper()
	pathItem, ok := rawPathItem.(map[string]interface{})
	if !ok {
		t.Fatalf("path item = %#v, want map", rawPathItem)
	}
	methods := make(map[string]interface{})
	for key, value := range pathItem {
		switch key {
		case "get", "put", "post", "delete", "options", "head", "patch", "trace":
			methods[key] = value
		}
	}
	return methods
}

func requiredMap(t *testing.T, parent map[string]interface{}, key string) map[string]interface{} {
	t.Helper()
	value, ok := parent[key]
	if !ok {
		t.Fatalf("missing %q", key)
	}
	result, ok := value.(map[string]interface{})
	if !ok {
		t.Fatalf("%q = %#v, want map", key, value)
	}
	return result
}

func requiredString(t *testing.T, parent map[string]interface{}, key string) string {
	t.Helper()
	value, ok := parent[key]
	if !ok {
		t.Fatalf("missing %q", key)
	}
	result, ok := value.(string)
	if !ok {
		t.Fatalf("%q = %#v, want string", key, value)
	}
	return result
}

func hasKey(parent map[string]interface{}, key string) bool {
	_, ok := parent[key]
	return ok
}

func mapKeys(parent map[string]interface{}) []string {
	keys := make([]string, 0, len(parent))
	for key := range parent {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sortedKeys(values map[string]struct{}) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func operationKey(path, method string) string {
	return method + " " + path
}
