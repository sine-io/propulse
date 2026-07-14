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
		{path: "/api/v1/neighborhoods", method: "get"},
		{path: "/api/v1/neighborhoods", method: "post", protected: true},
		{path: "/api/v1/neighborhoods/{id}", method: "get"},
		{path: "/api/v1/neighborhoods/{id}/metrics", method: "get"},
		{path: "/api/v1/watchlist/items", method: "post", protected: true},
		{path: "/api/v1/watchlist", method: "get", protected: true},
		{path: "/api/v1/decision/action-window", method: "get", protected: true},
		{path: "/admin/api/data-sources", method: "post", protected: true},
		{path: "/admin/api/data-sources", method: "get", protected: true},
		{path: "/admin/api/imports/json", method: "post", protected: true},
		{path: "/admin/api/imports/{id}", method: "get", protected: true},
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

func TestCapacityCalculationContract(t *testing.T) {
	spec := loadOpenAPI(t)
	components := requiredMap(t, spec, "components")
	schemas := requiredMap(t, components, "schemas")
	paths := requiredMap(t, spec, "paths")

	input := requiredMap(t, schemas, "HousingCapacityInput")
	assertRequiredFields(t, input, []string{
		"cashOnHand", "oldHomeValue", "oldLoanBalance", "monthlyIncome", "currentMonthlyMortgage",
		"acceptableMonthlyMortgage", "targetTotalPrice", "renovationBudget", "transactionCosts", "transitionRentCost",
	})
	inputProperties := requiredMap(t, input, "properties")
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
			"code":    "access_required",
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
