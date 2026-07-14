package handler

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	appcollection "github.com/sine-io/propulse/internal/application/collection"
)

func TestCSVAndJSONProduceEquivalentObservationInputs(t *testing.T) {
	listingPrice := 520.25
	days := 12
	status := "active"
	transactionPrice := 505.5
	transactionDate := "2026-07-01"
	originalRef := "listing-1"
	jsonRecords, issues := jsonObservationInputs([]jsonImportRecord{
		{
			RecordType: "listing", SourceRecordID: "listing-1", Layout: "three-bedroom", AreaSQM: floatPtr(89.5),
			ListingPrice: &listingPrice, DaysOnMarket: &days, Status: &status, Attributes: map[string]string{"floor": "8"},
		},
		{
			RecordType: "transaction", SourceRecordID: "tx-1", Layout: "three-bedroom", AreaSQM: floatPtr(89.5),
			TransactionPrice: &transactionPrice, TransactionDate: &transactionDate, OriginalListingRef: &originalRef,
			Attributes: map[string]string{},
		},
	})
	if len(issues) != 0 {
		t.Fatalf("JSON issues = %#v", issues)
	}
	csvBody := csvImportTemplate +
		`listing,listing-1,three-bedroom,89.5,520.25,,,12,active,,"{""floor"":""8""}"` + "\r\n" +
		`transaction,tx-1,three-bedroom,89.5,,505.5,2026-07-01,,,listing-1,{}` + "\r\n"
	csvRecords, issues, count := parseCSVObservations([]byte(csvBody))
	if len(issues) != 0 || count != 2 {
		t.Fatalf("CSV count/issues = %d/%#v", count, issues)
	}
	for index := range jsonRecords {
		jsonRecords[index].Row = 0
		csvRecords[index].Row = 0
	}
	if !reflect.DeepEqual(csvRecords, jsonRecords) {
		t.Fatalf("CSV records = %#v, want JSON records %#v", csvRecords, jsonRecords)
	}
}

func TestParseCSVAcceptsUTF8BOMAndUsesPhysicalLineNumbers(t *testing.T) {
	body := append([]byte{0xef, 0xbb, 0xbf}, []byte(csvImportTemplate+"\r\nlisting,listing-1,three-bedroom,not-a-number,520,,,12,active,,{}\r\n")...)
	_, issues, count := parseCSVObservations(body)
	if count != 1 || len(issues) != 1 {
		t.Fatalf("count/issues = %d/%#v", count, issues)
	}
	if issues[0].Row == nil || *issues[0].Row != 3 || issues[0].Field != "areaSqm" {
		t.Fatalf("issue = %#v, want physical line 3 areaSqm", issues[0])
	}
}

func TestParseCSVRejectsInvalidUTF8(t *testing.T) {
	_, issues, count := parseCSVObservations([]byte{0xff, 0xfe})
	if count != 0 || len(issues) != 1 || issues[0].Code != "invalid_encoding" {
		t.Fatalf("count/issues = %d/%#v", count, issues)
	}
}

func TestParseCSVRejectsInvalidHeaders(t *testing.T) {
	tests := []struct {
		name string
		body string
		code string
	}{
		{name: "unknown", body: strings.Replace(csvImportTemplate, "attributes", "extra", 1), code: "unknown_header"},
		{name: "missing", body: strings.Replace(csvImportTemplate, ",attributes", "", 1), code: "missing_header"},
		{name: "duplicate", body: strings.Replace(csvImportTemplate, "attributes", "status", 1), code: "duplicate_header"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, issues, count := parseCSVObservations([]byte(test.body))
			if count != 0 || !hasValidationCode(issues, test.code) {
				t.Fatalf("count/issues = %d/%#v, want %s", count, issues, test.code)
			}
		})
	}
}

func TestParseCSVRejectsMalformedRecordAndAttributes(t *testing.T) {
	t.Run("field count", func(t *testing.T) {
		_, issues, count := parseCSVObservations([]byte(csvImportTemplate + "listing,only-two\r\n"))
		if count != 1 || len(issues) != 1 || issues[0].Code != "invalid_csv" {
			t.Fatalf("count/issues = %d/%#v", count, issues)
		}
	})
	t.Run("attributes", func(t *testing.T) {
		body := csvImportTemplate + `listing,listing-1,three-bedroom,89,520,,,12,active,,[]` + "\r\n"
		_, issues, count := parseCSVObservations([]byte(body))
		if count != 1 || len(issues) != 1 || issues[0].Field != "attributes" {
			t.Fatalf("count/issues = %d/%#v", count, issues)
		}
	})
}

func TestAdminImportsCSVPreservesRawPayloadAndCallsSharedCommand(t *testing.T) {
	app := &stubTrustedCollectionApplication{result: trustedImportResult(false)}
	engine := gin.New()
	engine.POST("/imports/csv", NewAdminImports(app).CreateCSV)
	raw := []byte(csvImportTemplate + `listing,listing-1,three-bedroom,89.5,520,,,12,active,,{}` + "\r\n")
	request := newCSVImportRequest(t, "/imports/csv", raw)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d; body=%s", recorder.Code, recorder.Body.String())
	}
	if app.importCalls != 1 || app.command.Format != appcollection.ImportFormatCSV || !bytes.Equal(app.command.RawPayload, raw) {
		t.Fatalf("calls/command = %d/%#v", app.importCalls, app.command)
	}
	if len(app.command.Records) != 1 || app.command.Records[0].Row != 2 {
		t.Fatalf("records = %#v", app.command.Records)
	}
}

func TestAdminImportsCSVValidationIsAtomicAndCounted(t *testing.T) {
	app := &stubTrustedCollectionApplication{}
	engine := gin.New()
	engine.POST("/imports/csv", NewAdminImports(app).CreateCSV)
	raw := []byte(csvImportTemplate +
		`listing,listing-1,three-bedroom,89.5,520,,,12,active,,{}` + "\r\n" +
		`listing,listing-2,three-bedroom,nope,510,,,4,active,,{}` + "\r\n")
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, newCSVImportRequest(t, "/imports/csv", raw))

	if recorder.Code != http.StatusUnprocessableEntity || app.importCalls != 0 {
		t.Fatalf("status/calls = %d/%d; body=%s", recorder.Code, app.importCalls, recorder.Body.String())
	}
	var response importValidationErrorResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.AcceptedRecordCount != 0 || response.RejectedRecordCount != 2 {
		t.Fatalf("validation counts = %#v", response)
	}
}

func TestAdminImportsCSVTemplate(t *testing.T) {
	engine := gin.New()
	engine.GET("/imports/csv/template", NewAdminImports(&stubTrustedCollectionApplication{}).GetCSVTemplate)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/imports/csv/template", nil))
	if recorder.Code != http.StatusOK || recorder.Body.String() != csvImportTemplate {
		t.Fatalf("status/body = %d/%q", recorder.Code, recorder.Body.String())
	}
	if got := recorder.Header().Get("Content-Type"); got != "text/csv; charset=utf-8" {
		t.Fatalf("Content-Type = %q", got)
	}
	if got := recorder.Header().Get("Content-Disposition"); got != `attachment; filename="propulse-import-template.csv"` {
		t.Fatalf("Content-Disposition = %q", got)
	}
}

func newCSVImportRequest(t *testing.T, target string, raw []byte) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	fields := map[string]string{
		"dataSourceId":   "11111111-1111-1111-1111-111111111111",
		"neighborhoodId": "22222222-2222-2222-2222-222222222222",
		"sourceRef":      "weekly-1",
		"collectedAt":    "2026-07-13T10:00:00Z",
		"coverage":       "full",
	}
	for name, value := range fields {
		if err := writer.WriteField(name, value); err != nil {
			t.Fatal(err)
		}
	}
	part, err := writer.CreateFormFile("file", "import.csv")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(raw); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, target, &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	return request
}

func hasValidationCode(issues []appcollection.ValidationIssue, code string) bool {
	for _, issue := range issues {
		if issue.Code == code {
			return true
		}
	}
	return false
}

func floatPtr(value float64) *float64 { return &value }
