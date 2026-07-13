package handler

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	appcollection "github.com/sine-io/propulse/internal/application/collection"
	domainneighborhood "github.com/sine-io/propulse/internal/domain/neighborhood"
)

const maxImportBytes = 2 << 20

type CollectionApplication interface {
	ImportCollectionRun(context.Context, appcollection.ImportCollectionRunCommand) (appcollection.ImportCollectionRunResult, error)
	GetCollectionRun(context.Context, appcollection.GetCollectionRunQuery) (appcollection.CollectionRunDetail, error)
}

type AdminImports struct{ app CollectionApplication }

func NewAdminImports(app CollectionApplication) AdminImports { return AdminImports{app: app} }

type jsonImportRequest struct {
	DataSourceID   string             `json:"dataSourceId"`
	NeighborhoodID string             `json:"neighborhoodId"`
	SourceRef      string             `json:"sourceRef"`
	CollectedAt    string             `json:"collectedAt"`
	Coverage       string             `json:"coverage"`
	Records        []jsonImportRecord `json:"records"`
}

type jsonImportRecord struct {
	RecordType         string            `json:"recordType"`
	SourceRecordID     string            `json:"sourceRecordId"`
	Layout             string            `json:"layout"`
	AreaSQM            *float64          `json:"areaSqm"`
	ListingPrice       *float64          `json:"listingPrice"`
	TransactionPrice   *float64          `json:"transactionPrice"`
	TransactionDate    *string           `json:"transactionDate"`
	DaysOnMarket       *int              `json:"daysOnMarket"`
	Status             *string           `json:"status"`
	OriginalListingRef *string           `json:"originalListingRef"`
	Attributes         map[string]string `json:"attributes"`
}

type importCollectionRunResponse struct {
	CollectionRun               collectionRunResponse `json:"collectionRun"`
	ListingObservationCount     int                   `json:"listingObservationCount"`
	TransactionObservationCount int                   `json:"transactionObservationCount"`
	IdempotentReplay            bool                  `json:"idempotentReplay"`
	MetricRefreshStatus         string                `json:"metricRefreshStatus"`
}

type collectionRunResponse struct {
	ID                string                            `json:"id"`
	DataSourceID      string                            `json:"dataSourceId"`
	NeighborhoodID    string                            `json:"neighborhoodId"`
	SourceRef         string                            `json:"sourceRef"`
	CollectedAt       string                            `json:"collectedAt"`
	Coverage          domainneighborhood.Coverage       `json:"coverage"`
	Format            appcollection.ImportFormat        `json:"format"`
	ContentChecksum   string                            `json:"contentChecksum"`
	RawContentType    string                            `json:"rawContentType"`
	ValidationSummary validationSummaryResponse         `json:"validationSummary"`
	Status            appcollection.CollectionRunStatus `json:"status"`
	MetricStatus      appcollection.MetricStatus        `json:"metricStatus"`
	CreatedAt         string                            `json:"createdAt"`
	UpdatedAt         string                            `json:"updatedAt"`
}

type validationSummaryResponse struct {
	RecordCount      int                             `json:"recordCount"`
	ListingCount     int                             `json:"listingCount"`
	TransactionCount int                             `json:"transactionCount"`
	Issues           []appcollection.ValidationIssue `json:"issues"`
}

type collectionRunDetailResponse struct {
	CollectionRun    collectionRunResponse            `json:"collectionRun"`
	Source           dataSourceResponse               `json:"source"`
	Listings         []listingObservationResponse     `json:"listings"`
	Transactions     []transactionObservationResponse `json:"transactions"`
	RawPayloadBase64 string                           `json:"rawPayloadBase64"`
}

type listingObservationResponse struct {
	ID              string                      `json:"id"`
	CollectionRunID string                      `json:"collectionRunId"`
	NeighborhoodID  string                      `json:"neighborhoodId"`
	SourceListingID string                      `json:"sourceListingId"`
	SourceRow       int                         `json:"sourceRow"`
	Layout          string                      `json:"layout"`
	AreaSQM         float64                     `json:"areaSqm"`
	ListingPrice    float64                     `json:"listingPrice"`
	DaysOnMarket    int                         `json:"daysOnMarket"`
	Status          appcollection.ListingStatus `json:"status"`
	CapturedAt      string                      `json:"capturedAt"`
	Attributes      map[string]string           `json:"attributes"`
}

type transactionObservationResponse struct {
	ID                 string  `json:"id"`
	CollectionRunID    string  `json:"collectionRunId"`
	NeighborhoodID     string  `json:"neighborhoodId"`
	SourceRecordID     string  `json:"sourceRecordId"`
	SourceRow          int     `json:"sourceRow"`
	Layout             string  `json:"layout"`
	AreaSQM            float64 `json:"areaSqm"`
	TransactionPrice   float64 `json:"transactionPrice"`
	TransactionDate    string  `json:"transactionDate"`
	OriginalListingRef *string `json:"originalListingRef,omitempty"`
	CapturedAt         string  `json:"capturedAt"`
}

func (h AdminImports) CreateJSON(c *gin.Context) {
	raw, err := io.ReadAll(io.LimitReader(c.Request.Body, maxImportBytes+1))
	if err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", "request body is invalid")
		return
	}
	if len(raw) > maxImportBytes {
		writeError(c, http.StatusRequestEntityTooLarge, "payload_too_large", "request body exceeds 2 MiB")
		return
	}

	var request jsonImportRequest
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", "request body is invalid")
		return
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		writeError(c, http.StatusBadRequest, "invalid_request", "request body must contain one JSON value")
		return
	}

	collectedAt, err := time.Parse(time.RFC3339, request.CollectedAt)
	if err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", "collectedAt must use RFC3339")
		return
	}
	records, issues := jsonObservationInputs(request.Records)
	if len(issues) > 0 {
		writeValidationError(c, issues)
		return
	}

	result, err := h.app.ImportCollectionRun(c.Request.Context(), appcollection.ImportCollectionRunCommand{
		DataSourceID: request.DataSourceID, NeighborhoodID: request.NeighborhoodID, SourceRef: request.SourceRef,
		CollectedAt: collectedAt, Coverage: domainneighborhood.Coverage(request.Coverage), Format: appcollection.ImportFormatJSON,
		RawPayload: append([]byte(nil), raw...), RawContentType: "application/json", Records: records,
	})
	if err != nil {
		writeCollectionError(c, err)
		return
	}
	status := http.StatusCreated
	if result.IdempotentReplay {
		status = http.StatusOK
	}
	c.JSON(status, newImportCollectionRunResponse(result))
}

func (h AdminImports) GetDetail(c *gin.Context) {
	detail, err := h.app.GetCollectionRun(c.Request.Context(), appcollection.GetCollectionRunQuery{ID: c.Param("id")})
	if err != nil {
		switch {
		case errors.Is(err, appcollection.ErrInvalidRequest):
			writeError(c, http.StatusBadRequest, "invalid_request", "import id must be a UUID")
		case errors.Is(err, appcollection.ErrCollectionRunNotFound):
			writeError(c, http.StatusNotFound, "import_not_found", "import not found")
		default:
			writeError(c, http.StatusInternalServerError, "import_failed", "import lookup failed")
		}
		return
	}
	c.JSON(http.StatusOK, newCollectionRunDetailResponse(detail))
}

func jsonObservationInputs(records []jsonImportRecord) ([]appcollection.ObservationInput, []appcollection.ValidationIssue) {
	inputs := make([]appcollection.ObservationInput, 0, len(records))
	issues := make([]appcollection.ValidationIssue, 0)
	for index, record := range records {
		row := index + 1
		input := appcollection.ObservationInput{
			Row: row, RecordType: appcollection.RecordType(record.RecordType), SourceRecordID: record.SourceRecordID,
			Layout: record.Layout, ListingPrice: record.ListingPrice, TransactionPrice: record.TransactionPrice,
			DaysOnMarket: record.DaysOnMarket, OriginalListingRef: record.OriginalListingRef, Attributes: record.Attributes,
		}
		if record.AreaSQM != nil {
			input.AreaSQM = *record.AreaSQM
		}
		if record.Status != nil {
			status := appcollection.ListingStatus(*record.Status)
			input.Status = &status
		}
		if record.TransactionDate != nil {
			parsed, err := time.Parse(time.DateOnly, *record.TransactionDate)
			if err != nil {
				rowCopy := row
				issues = append(issues, appcollection.ValidationIssue{Row: &rowCopy, Field: "transactionDate", Code: "invalid_date", Message: "transactionDate must use YYYY-MM-DD"})
			} else {
				input.TransactionDate = &parsed
			}
		}
		inputs = append(inputs, input)
	}
	return inputs, issues
}

func writeCollectionError(c *gin.Context, err error) {
	var validationErr *appcollection.ValidationError
	switch {
	case errors.As(err, &validationErr):
		writeValidationError(c, validationErr.Issues)
	case errors.Is(err, appcollection.ErrDataSourceNotFound):
		writeError(c, http.StatusNotFound, "data_source_not_found", "data source not found")
	case errors.Is(err, appcollection.ErrNeighborhoodNotFound):
		writeError(c, http.StatusNotFound, "neighborhood_not_found", "neighborhood not found")
	default:
		writeError(c, http.StatusInternalServerError, "import_failed", "import failed")
	}
}

func newImportCollectionRunResponse(result appcollection.ImportCollectionRunResult) importCollectionRunResponse {
	return importCollectionRunResponse{
		CollectionRun: newCollectionRunResponse(result.Run), ListingObservationCount: result.ListingCount,
		TransactionObservationCount: result.TransactionCount, IdempotentReplay: result.IdempotentReplay,
		MetricRefreshStatus: string(result.MetricRefreshStatus),
	}
}

func newCollectionRunResponse(run appcollection.CollectionRun) collectionRunResponse {
	issues := run.ValidationSummary.Issues
	if issues == nil {
		issues = []appcollection.ValidationIssue{}
	}
	return collectionRunResponse{
		ID: run.ID, DataSourceID: run.DataSourceID, NeighborhoodID: run.NeighborhoodID, SourceRef: run.SourceRef,
		CollectedAt: run.CollectedAt.UTC().Format(time.RFC3339), Coverage: run.Coverage, Format: run.Format,
		ContentChecksum: run.ContentChecksum, RawContentType: run.RawContentType,
		ValidationSummary: validationSummaryResponse{RecordCount: run.ValidationSummary.RecordCount, ListingCount: run.ValidationSummary.ListingCount, TransactionCount: run.ValidationSummary.TransactionCount, Issues: issues},
		Status:            run.Status, MetricStatus: run.MetricStatus, CreatedAt: run.CreatedAt.UTC().Format(time.RFC3339), UpdatedAt: run.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func newCollectionRunDetailResponse(detail appcollection.CollectionRunDetail) collectionRunDetailResponse {
	listings := make([]listingObservationResponse, 0, len(detail.Listings))
	for _, item := range detail.Listings {
		listings = append(listings, listingObservationResponse{
			ID: item.ID, CollectionRunID: item.CollectionRunID, NeighborhoodID: item.NeighborhoodID,
			SourceListingID: item.SourceListingID, SourceRow: item.SourceRow, Layout: item.Layout, AreaSQM: item.AreaSQM,
			ListingPrice: item.ListingPrice, DaysOnMarket: item.DaysOnMarket, Status: item.Status,
			CapturedAt: item.CapturedAt.UTC().Format(time.RFC3339), Attributes: item.Attributes,
		})
	}
	transactions := make([]transactionObservationResponse, 0, len(detail.Transactions))
	for _, item := range detail.Transactions {
		transactions = append(transactions, transactionObservationResponse{
			ID: item.ID, CollectionRunID: item.CollectionRunID, NeighborhoodID: item.NeighborhoodID,
			SourceRecordID: item.SourceRecordID, SourceRow: item.SourceRow, Layout: item.Layout, AreaSQM: item.AreaSQM,
			TransactionPrice: item.TransactionPrice, TransactionDate: item.TransactionDate.UTC().Format(time.DateOnly),
			OriginalListingRef: item.OriginalListingRef, CapturedAt: item.CapturedAt.UTC().Format(time.RFC3339),
		})
	}
	return collectionRunDetailResponse{
		CollectionRun: newCollectionRunResponse(detail.Run), Source: newDataSourceResponse(detail.Source),
		Listings: listings, Transactions: transactions, RawPayloadBase64: base64.StdEncoding.EncodeToString(detail.Run.RawPayload),
	}
}
