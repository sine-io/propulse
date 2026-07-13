package collection

import (
	"errors"
	"time"

	domainneighborhood "github.com/sine-io/propulse/internal/domain/neighborhood"
)

type ImportFormat string

const (
	ImportFormatJSON ImportFormat = "json"
	ImportFormatCSV  ImportFormat = "csv"
)

type RecordType string

const (
	RecordTypeListing     RecordType = "listing"
	RecordTypeTransaction RecordType = "transaction"
)

type ListingStatus string

const (
	ListingStatusActive    ListingStatus = "active"
	ListingStatusPending   ListingStatus = "pending"
	ListingStatusWithdrawn ListingStatus = "withdrawn"
	ListingStatusSold      ListingStatus = "sold"
)

type ObservationInput struct {
	Row                int
	RecordType         RecordType
	SourceRecordID     string
	Layout             string
	AreaSQM            float64
	ListingPrice       *float64
	TransactionPrice   *float64
	TransactionDate    *time.Time
	DaysOnMarket       *int
	Status             *ListingStatus
	OriginalListingRef *string
	Attributes         map[string]string
}

type ImportCollectionRunCommand struct {
	DataSourceID   string
	NeighborhoodID string
	SourceRef      string
	CollectedAt    time.Time
	Coverage       domainneighborhood.Coverage
	Format         ImportFormat
	RawPayload     []byte
	RawContentType string
	Records        []ObservationInput
}

type ValidationIssue struct {
	Row     *int   `json:"row,omitempty"`
	Field   string `json:"field"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

type ValidationError struct {
	Issues []ValidationIssue
}

func (e *ValidationError) Error() string {
	return "one or more import fields are invalid"
}

var ErrDataSourceNotFound = errors.New("data_source_not_found")
var ErrCollectionRunNotFound = errors.New("collection_run_not_found")

type MetricStatus string

const (
	MetricStatusPending   MetricStatus = "pending"
	MetricStatusCompleted MetricStatus = "completed"
	MetricStatusFailed    MetricStatus = "failed"
)

type CollectionRunStatus string

const (
	CollectionRunStatusCompleted CollectionRunStatus = "completed"
)

type DataSource struct {
	ID         string
	Name       string
	SourceType string
	City       string
	Notes      string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type CreateDataSourceCommand struct {
	Name       string
	SourceType string
	City       string
	Notes      string
}

type CollectionRun struct {
	ID                string
	DataSourceID      string
	NeighborhoodID    string
	SourceRef         string
	CollectedAt       time.Time
	Coverage          domainneighborhood.Coverage
	Format            ImportFormat
	ContentChecksum   string
	RawPayload        []byte
	RawContentType    string
	ValidationSummary ValidationSummary
	Status            CollectionRunStatus
	MetricStatus      MetricStatus
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type ListingObservation struct {
	ID              string
	CollectionRunID string
	NeighborhoodID  string
	SourceListingID string
	SourceRow       int
	Layout          string
	AreaSQM         float64
	ListingPrice    float64
	DaysOnMarket    int
	Status          ListingStatus
	CapturedAt      time.Time
	Attributes      map[string]string
}

type TransactionObservation struct {
	ID                 string
	CollectionRunID    string
	NeighborhoodID     string
	SourceRecordID     string
	SourceRow          int
	Layout             string
	AreaSQM            float64
	TransactionPrice   float64
	TransactionDate    time.Time
	OriginalListingRef *string
	CapturedAt         time.Time
}

type ValidationSummary struct {
	RecordCount      int               `json:"recordCount"`
	ListingCount     int               `json:"listingCount"`
	TransactionCount int               `json:"transactionCount"`
	Issues           []ValidationIssue `json:"issues"`
}

type NormalizedImport struct {
	DataSourceID      string
	NeighborhoodID    string
	SourceRef         string
	CollectedAt       time.Time
	Coverage          domainneighborhood.Coverage
	Format            ImportFormat
	RawPayload        []byte
	RawContentType    string
	Listings          []ListingObservation
	Transactions      []TransactionObservation
	ValidationSummary ValidationSummary
}

type ImportBatch struct {
	Run          CollectionRun
	Listings     []ListingObservation
	Transactions []TransactionObservation
}

type SaveImportResult struct {
	Run     CollectionRun
	Created bool
}

type CollectionRunDetail struct {
	Run          CollectionRun
	Source       DataSource
	Listings     []ListingObservation
	Transactions []TransactionObservation
}

type GetCollectionRunQuery struct {
	ID string
}

type ImportCollectionRunResult struct {
	Run                 CollectionRun
	ListingCount        int
	TransactionCount    int
	IdempotentReplay    bool
	MetricRefreshStatus MetricStatus
}

func (normalized NormalizedImport) NewBatch(runID string, newID func() string) ImportBatch {
	if runID == "" {
		runID = newID()
	}
	rawPayload := append([]byte(nil), normalized.RawPayload...)
	run := CollectionRun{
		ID:                runID,
		DataSourceID:      normalized.DataSourceID,
		NeighborhoodID:    normalized.NeighborhoodID,
		SourceRef:         normalized.SourceRef,
		CollectedAt:       normalized.CollectedAt,
		Coverage:          normalized.Coverage,
		Format:            normalized.Format,
		ContentChecksum:   contentChecksum(normalized.command()),
		RawPayload:        rawPayload,
		RawContentType:    normalized.RawContentType,
		ValidationSummary: normalized.ValidationSummary,
		Status:            CollectionRunStatusCompleted,
		MetricStatus:      MetricStatusPending,
		CreatedAt:         normalized.CollectedAt,
		UpdatedAt:         normalized.CollectedAt,
	}
	listings := append([]ListingObservation(nil), normalized.Listings...)
	for i := range listings {
		listings[i].ID = newID()
		listings[i].CollectionRunID = runID
		listings[i].NeighborhoodID = normalized.NeighborhoodID
		listings[i].CapturedAt = normalized.CollectedAt
		listings[i].Attributes = copyAttributes(listings[i].Attributes)
	}
	transactions := append([]TransactionObservation(nil), normalized.Transactions...)
	for i := range transactions {
		transactions[i].ID = newID()
		transactions[i].CollectionRunID = runID
		transactions[i].NeighborhoodID = normalized.NeighborhoodID
		transactions[i].CapturedAt = normalized.CollectedAt
	}
	return ImportBatch{Run: run, Listings: listings, Transactions: transactions}
}

func (normalized NormalizedImport) command() ImportCollectionRunCommand {
	return ImportCollectionRunCommand{
		DataSourceID:   normalized.DataSourceID,
		NeighborhoodID: normalized.NeighborhoodID,
		SourceRef:      normalized.SourceRef,
		CollectedAt:    normalized.CollectedAt,
		Coverage:       normalized.Coverage,
		Format:         normalized.Format,
		RawPayload:     normalized.RawPayload,
		RawContentType: normalized.RawContentType,
	}
}
