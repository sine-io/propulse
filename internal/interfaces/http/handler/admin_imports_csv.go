package handler

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	appcollection "github.com/sine-io/propulse/internal/application/collection"
	domainneighborhood "github.com/sine-io/propulse/internal/domain/neighborhood"
)

const maxMultipartImportBytes = maxImportBytes + 64*1024

const csvImportTemplate = "recordType,sourceRecordId,layout,areaSqm,listingPrice,transactionPrice,transactionDate,daysOnMarket,status,originalListingRef,attributes\r\n"

var csvImportHeaders = []string{
	"recordType",
	"sourceRecordId",
	"layout",
	"areaSqm",
	"listingPrice",
	"transactionPrice",
	"transactionDate",
	"daysOnMarket",
	"status",
	"originalListingRef",
	"attributes",
}

func (h AdminImports) CreateCSV(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxMultipartImportBytes)
	if err := c.Request.ParseMultipartForm(64 << 10); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			writeError(c, http.StatusRequestEntityTooLarge, "payload_too_large", "multipart request exceeds 2 MiB plus metadata allowance")
			return
		}
		writeError(c, http.StatusBadRequest, "invalid_request", "multipart request is invalid")
		return
	}
	defer func() {
		if err := c.Request.MultipartForm.RemoveAll(); err != nil {
			c.Set("multipart_cleanup_error", err)
		}
	}()

	files := c.Request.MultipartForm.File["file"]
	if len(files) != 1 {
		writeError(c, http.StatusBadRequest, "invalid_request", "exactly one CSV file is required")
		return
	}
	if files[0].Size > maxImportBytes {
		writeError(c, http.StatusRequestEntityTooLarge, "payload_too_large", "CSV file exceeds 2 MiB")
		return
	}
	file, err := files[0].Open()
	if err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", "CSV file cannot be read")
		return
	}
	defer func() {
		if err := file.Close(); err != nil {
			c.Set("import_file_close_error", err)
		}
	}()
	raw, err := io.ReadAll(io.LimitReader(file, maxImportBytes+1))
	if err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", "CSV file cannot be read")
		return
	}
	if len(raw) > maxImportBytes {
		writeError(c, http.StatusRequestEntityTooLarge, "payload_too_large", "CSV file exceeds 2 MiB")
		return
	}

	records, issues, recordCount := parseCSVObservations(raw)
	collectedAt, parseErr := time.Parse(time.RFC3339, c.PostForm("collectedAt"))
	if parseErr != nil {
		issues = append(issues, appcollection.ValidationIssue{
			Field: "collectedAt", Code: "invalid_datetime", Message: "collectedAt must use RFC3339",
		})
	}
	if len(issues) > 0 {
		writeImportValidationError(c, issues, recordCount)
		return
	}

	result, err := h.app.ImportCollectionRun(c.Request.Context(), appcollection.ImportCollectionRunCommand{
		DataSourceID:   c.PostForm("dataSourceId"),
		NeighborhoodID: c.PostForm("neighborhoodId"),
		SourceRef:      c.PostForm("sourceRef"),
		CollectedAt:    collectedAt,
		Coverage:       domainneighborhood.Coverage(c.PostForm("coverage")),
		Format:         appcollection.ImportFormatCSV,
		RawPayload:     append([]byte(nil), raw...),
		RawContentType: "text/csv",
		Records:        records,
	})
	if err != nil {
		writeCollectionError(c, err, len(records))
		return
	}
	status := http.StatusCreated
	if result.IdempotentReplay {
		status = http.StatusOK
	}
	c.JSON(status, newImportCollectionRunResponse(result))
}

func (h AdminImports) GetCSVTemplate(c *gin.Context) {
	c.Header("Content-Disposition", `attachment; filename="propulse-import-template.csv"`)
	c.Data(http.StatusOK, "text/csv; charset=utf-8", []byte(csvImportTemplate))
}

func parseCSVObservations(raw []byte) ([]appcollection.ObservationInput, []appcollection.ValidationIssue, int) {
	if !utf8.Valid(raw) {
		return nil, []appcollection.ValidationIssue{{
			Field: "file", Code: "invalid_encoding", Message: "CSV file must be valid UTF-8",
		}}, 0
	}
	payload := bytes.TrimPrefix(raw, []byte{0xef, 0xbb, 0xbf})
	reader := csv.NewReader(bytes.NewReader(payload))
	reader.FieldsPerRecord = -1

	header, err := reader.Read()
	if errors.Is(err, io.EOF) {
		return nil, []appcollection.ValidationIssue{{
			Field: "header", Code: "required", Message: "CSV header is required",
		}}, 0
	}
	if err != nil {
		return nil, []appcollection.ValidationIssue{csvParseIssue(err, 1)}, 0
	}
	headerLine, _ := reader.FieldPos(0)
	indexes, issues := validateCSVHeader(header, headerLine)
	if len(issues) > 0 {
		return nil, issues, 0
	}

	reader.FieldsPerRecord = len(header)
	records := make([]appcollection.ObservationInput, 0)
	recordCount := 0
	for {
		row, readErr := reader.Read()
		if errors.Is(readErr, io.EOF) {
			break
		}
		recordCount++
		if readErr != nil {
			issues = append(issues, csvParseIssue(readErr, headerLine+recordCount))
			continue
		}
		line, _ := reader.FieldPos(0)
		input, rowIssues := csvObservationInput(row, indexes, line)
		issues = append(issues, rowIssues...)
		records = append(records, input)
	}
	if len(issues) > 0 {
		return nil, issues, recordCount
	}
	return records, nil, recordCount
}

func validateCSVHeader(header []string, line int) (map[string]int, []appcollection.ValidationIssue) {
	row := line
	indexes := make(map[string]int, len(header))
	allowed := make(map[string]struct{}, len(csvImportHeaders))
	for _, name := range csvImportHeaders {
		allowed[name] = struct{}{}
	}
	issues := make([]appcollection.ValidationIssue, 0)
	for index, name := range header {
		if _, ok := allowed[name]; !ok {
			issues = append(issues, appcollection.ValidationIssue{
				Row: &row, Field: "header", Code: "unknown_header", Message: fmt.Sprintf("unknown CSV header %q", name),
			})
			continue
		}
		if _, duplicate := indexes[name]; duplicate {
			issues = append(issues, appcollection.ValidationIssue{
				Row: &row, Field: "header", Code: "duplicate_header", Message: fmt.Sprintf("duplicate CSV header %q", name),
			})
			continue
		}
		indexes[name] = index
	}
	for _, name := range csvImportHeaders {
		if _, ok := indexes[name]; !ok {
			issues = append(issues, appcollection.ValidationIssue{
				Row: &row, Field: "header", Code: "missing_header", Message: fmt.Sprintf("missing CSV header %q", name),
			})
		}
	}
	return indexes, issues
}

func csvObservationInput(row []string, indexes map[string]int, line int) (appcollection.ObservationInput, []appcollection.ValidationIssue) {
	input := appcollection.ObservationInput{
		Row:            line,
		RecordType:     appcollection.RecordType(row[indexes["recordType"]]),
		SourceRecordID: row[indexes["sourceRecordId"]],
		Layout:         row[indexes["layout"]],
	}
	issues := make([]appcollection.ValidationIssue, 0)
	rowNumber := line

	if value := row[indexes["areaSqm"]]; value != "" {
		parsed, err := strconv.ParseFloat(value, 64)
		if err != nil {
			issues = append(issues, csvValueIssue(&rowNumber, "areaSqm", "invalid_number", "areaSqm must be a number"))
		} else {
			input.AreaSQM = parsed
		}
	}
	input.ListingPrice, issues = parseOptionalCSVFloat(row[indexes["listingPrice"]], &rowNumber, "listingPrice", issues)
	input.TransactionPrice, issues = parseOptionalCSVFloat(row[indexes["transactionPrice"]], &rowNumber, "transactionPrice", issues)
	input.DaysOnMarket, issues = parseOptionalCSVInt(row[indexes["daysOnMarket"]], &rowNumber, "daysOnMarket", issues)

	if value := row[indexes["transactionDate"]]; value != "" {
		parsed, err := time.Parse(time.DateOnly, value)
		if err != nil {
			issues = append(issues, csvValueIssue(&rowNumber, "transactionDate", "invalid_date", "transactionDate must use YYYY-MM-DD"))
		} else {
			input.TransactionDate = &parsed
		}
	}
	if value := row[indexes["status"]]; value != "" {
		status := appcollection.ListingStatus(value)
		input.Status = &status
	}
	if value := row[indexes["originalListingRef"]]; value != "" {
		input.OriginalListingRef = &value
	}

	attributes := row[indexes["attributes"]]
	input.Attributes = map[string]string{}
	if attributes != "" {
		decoder := json.NewDecoder(strings.NewReader(attributes))
		if err := decoder.Decode(&input.Attributes); err != nil || input.Attributes == nil {
			issues = append(issues, csvValueIssue(&rowNumber, "attributes", "invalid_json_object", "attributes must be a JSON object with string values"))
		} else if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
			issues = append(issues, csvValueIssue(&rowNumber, "attributes", "invalid_json_object", "attributes must contain one JSON object"))
		}
	}
	return input, issues
}

func parseOptionalCSVFloat(value string, row *int, field string, issues []appcollection.ValidationIssue) (*float64, []appcollection.ValidationIssue) {
	if value == "" {
		return nil, issues
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return nil, append(issues, csvValueIssue(row, field, "invalid_number", field+" must be a number"))
	}
	return &parsed, issues
}

func parseOptionalCSVInt(value string, row *int, field string, issues []appcollection.ValidationIssue) (*int, []appcollection.ValidationIssue) {
	if value == "" {
		return nil, issues
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return nil, append(issues, csvValueIssue(row, field, "invalid_integer", field+" must be an integer"))
	}
	return &parsed, issues
}

func csvParseIssue(err error, fallbackLine int) appcollection.ValidationIssue {
	line := fallbackLine
	var parseErr *csv.ParseError
	if errors.As(err, &parseErr) && parseErr.Line > 0 {
		line = parseErr.Line
	}
	return appcollection.ValidationIssue{
		Row: &line, Field: "record", Code: "invalid_csv", Message: "CSV record is malformed",
	}
}

func csvValueIssue(row *int, field, code, message string) appcollection.ValidationIssue {
	return appcollection.ValidationIssue{Row: row, Field: field, Code: code, Message: message}
}
