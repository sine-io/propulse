package collection

import (
	"reflect"
	"testing"
	"time"

	domainneighborhood "github.com/sine-io/propulse/internal/domain/neighborhood"
)

func TestValidateAndNormalizeSplitsListingAndTransaction(t *testing.T) {
	now := time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC)
	transactionDate := now.AddDate(0, 0, -1)
	daysOnMarket := 14
	active := ListingStatusActive
	originalRef := " listing-001 "
	listingPrice := 520000.25
	transactionPrice := 498000.75

	normalized, issues := validateAndNormalize(ImportCollectionRunCommand{
		DataSourceID:   "11111111-1111-1111-1111-111111111111",
		NeighborhoodID: "22222222-2222-2222-2222-222222222222",
		SourceRef:      "weekly-2026-07-09",
		CollectedAt:    now,
		Coverage:       domainneighborhood.CoverageFull,
		Format:         ImportFormatJSON,
		RawPayload:     []byte(`{"records":2}`),
		RawContentType: "application/json",
		Records: []ObservationInput{
			{
				Row:            1,
				RecordType:     RecordTypeListing,
				SourceRecordID: "listing-001",
				Layout:         "三房",
				AreaSQM:        88.5,
				ListingPrice:   &listingPrice,
				DaysOnMarket:   &daysOnMarket,
				Status:         &active,
				Attributes:     map[string]string{"floor": "high"},
			},
			{
				Row:                2,
				RecordType:         RecordTypeTransaction,
				SourceRecordID:     "txn-001",
				Layout:             "两房",
				AreaSQM:            66,
				TransactionPrice:   &transactionPrice,
				TransactionDate:    &transactionDate,
				OriginalListingRef: &originalRef,
			},
		},
	}, now)
	if len(issues) != 0 {
		t.Fatalf("issues = %#v, want none", issues)
	}
	if len(normalized.Listings) != 1 {
		t.Fatalf("listings = %d, want 1", len(normalized.Listings))
	}
	if len(normalized.Transactions) != 1 {
		t.Fatalf("transactions = %d, want 1", len(normalized.Transactions))
	}
	if _, ok := reflect.TypeOf(normalized.Listings[0]).FieldByName("PriceCut"); ok {
		t.Fatal("ListingObservation contains caller-supplied PriceCut field")
	}
	if _, ok := reflect.TypeOf(normalized.Transactions[0]).FieldByName("PriceCut"); ok {
		t.Fatal("TransactionObservation contains caller-supplied PriceCut field")
	}
	if normalized.Listings[0].SourceListingID != "listing-001" || normalized.Listings[0].ListingPrice != listingPrice || normalized.Listings[0].DaysOnMarket != daysOnMarket || normalized.Listings[0].Status != ListingStatusActive {
		t.Fatalf("listing = %#v", normalized.Listings[0])
	}
	if normalized.Transactions[0].SourceRecordID != "txn-001" || normalized.Transactions[0].TransactionPrice != transactionPrice || !normalized.Transactions[0].TransactionDate.Equal(transactionDate) {
		t.Fatalf("transaction = %#v", normalized.Transactions[0])
	}
	if normalized.Transactions[0].OriginalListingRef == nil || *normalized.Transactions[0].OriginalListingRef != "listing-001" {
		t.Fatalf("OriginalListingRef = %#v, want trimmed listing-001", normalized.Transactions[0].OriginalListingRef)
	}
	if normalized.ValidationSummary.RecordCount != 2 || normalized.ValidationSummary.ListingCount != 1 || normalized.ValidationSummary.TransactionCount != 1 {
		t.Fatalf("summary = %#v", normalized.ValidationSummary)
	}

	ids := []string{"run-1", "listing-id", "transaction-id"}
	importedAt := now.Add(time.Hour)
	batch := normalized.NewBatch("", importedAt, func() string {
		id := ids[0]
		ids = ids[1:]
		return id
	})
	if batch.Run.ID != "run-1" || batch.Run.Status != CollectionRunStatusCompleted || batch.Run.MetricStatus != MetricStatusPending {
		t.Fatalf("run = %#v", batch.Run)
	}
	if !batch.Run.CreatedAt.Equal(importedAt) || !batch.Run.UpdatedAt.Equal(importedAt) {
		t.Fatalf("run persistence timestamps = %v/%v, want %v", batch.Run.CreatedAt, batch.Run.UpdatedAt, importedAt)
	}
	if batch.Run.ContentChecksum == "" {
		t.Fatal("ContentChecksum is empty")
	}
	if len(batch.Run.RawPayload) == 0 || &batch.Run.RawPayload[0] == &normalized.RawPayload[0] {
		t.Fatal("run raw payload was not copied")
	}
	if batch.Listings[0].ID != "listing-id" || batch.Listings[0].CollectionRunID != "run-1" || batch.Listings[0].NeighborhoodID != normalized.NeighborhoodID || !batch.Listings[0].CapturedAt.Equal(now) {
		t.Fatalf("batch listing = %#v", batch.Listings[0])
	}
	if batch.Transactions[0].ID != "transaction-id" || batch.Transactions[0].CollectionRunID != "run-1" || batch.Transactions[0].NeighborhoodID != normalized.NeighborhoodID || !batch.Transactions[0].CapturedAt.Equal(now) {
		t.Fatalf("batch transaction = %#v", batch.Transactions[0])
	}
}

func TestValidateAndNormalizeRejectsInvalidRecordsWithAllIssues(t *testing.T) {
	now := time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC)
	collectedAt := now.Add(6 * time.Minute)
	badListingPrice := 10.123
	badTransactionPrice := -1.0
	badTransactionDate := now.Add(24 * time.Hour)
	badDaysOnMarket := -1
	badStatus := ListingStatus("leased")

	_, issues := validateAndNormalize(ImportCollectionRunCommand{
		DataSourceID:   "not-a-uuid",
		NeighborhoodID: "also-not-a-uuid",
		SourceRef:      " ",
		CollectedAt:    collectedAt,
		Coverage:       domainneighborhood.CoverageUnknown,
		Format:         ImportFormat("xml"),
		RawPayload:     nil,
		RawContentType: " ",
		Records: []ObservationInput{
			{
				Row:              1,
				RecordType:       RecordTypeListing,
				SourceRecordID:   " ",
				Layout:           " ",
				AreaSQM:          -10,
				ListingPrice:     &badListingPrice,
				TransactionPrice: &badTransactionPrice,
				DaysOnMarket:     &badDaysOnMarket,
				Status:           &badStatus,
				Attributes:       map[string]string{"": "value"},
			},
			{
				Row:             2,
				RecordType:      RecordTypeTransaction,
				SourceRecordID:  "txn-001",
				Layout:          "两房",
				AreaSQM:         80,
				ListingPrice:    &badListingPrice,
				DaysOnMarket:    &badDaysOnMarket,
				Status:          &badStatus,
				TransactionDate: &badTransactionDate,
			},
		},
	}, now)

	want := []issueKey{
		{field: "dataSourceId", code: "invalid_uuid"},
		{field: "neighborhoodId", code: "invalid_uuid"},
		{field: "sourceRef", code: "required"},
		{field: "collectedAt", code: "future"},
		{field: "coverage", code: "invalid"},
		{field: "format", code: "invalid"},
		{field: "rawPayload", code: "required"},
		{field: "rawContentType", code: "required"},
		{row: intPtr(1), field: "sourceRecordId", code: "required"},
		{row: intPtr(1), field: "layout", code: "required"},
		{row: intPtr(1), field: "areaSqm", code: "out_of_range"},
		{row: intPtr(1), field: "listingPrice", code: "too_many_decimals"},
		{row: intPtr(1), field: "transactionPrice", code: "forbidden"},
		{row: intPtr(1), field: "daysOnMarket", code: "out_of_range"},
		{row: intPtr(1), field: "status", code: "invalid"},
		{row: intPtr(1), field: "attributes.key", code: "required"},
		{row: intPtr(2), field: "listingPrice", code: "forbidden"},
		{row: intPtr(2), field: "daysOnMarket", code: "forbidden"},
		{row: intPtr(2), field: "status", code: "forbidden"},
		{row: intPtr(2), field: "transactionPrice", code: "required"},
		{row: intPtr(2), field: "transactionDate", code: "future"},
	}
	assertIssues(t, issues, want)
}

func TestValidateAndNormalizeRejectsMoreThanFiveHundredRecords(t *testing.T) {
	command := validImportCollectionRunCommand()
	command.Records = make([]ObservationInput, 501)
	for i := range command.Records {
		command.Records[i] = validListingInput(i + 1)
	}

	_, issues := validateAndNormalize(command, command.CollectedAt)
	assertIssues(t, issues, []issueKey{{field: "records", code: "too_many"}})
}

func TestValidateAndNormalizeRejectsDuplicateSourceRecordIDs(t *testing.T) {
	command := validImportCollectionRunCommand()
	command.Records = []ObservationInput{
		validListingInput(1),
		validListingInput(2),
		validTransactionInput(3),
		validTransactionInput(4),
	}
	command.Records[0].SourceRecordID = " duplicate "
	command.Records[1].SourceRecordID = "duplicate"
	command.Records[2].SourceRecordID = "same-id"
	command.Records[3].SourceRecordID = "same-id"

	_, issues := validateAndNormalize(command, command.CollectedAt)
	assertIssues(t, issues, []issueKey{
		{row: intPtr(2), field: "sourceRecordId", code: "duplicate"},
		{row: intPtr(4), field: "sourceRecordId", code: "duplicate"},
	})
}

func TestValidateAndNormalizeTrimsSourceAndRecordFields(t *testing.T) {
	command := validImportCollectionRunCommand()
	command.DataSourceID = " 11111111-1111-1111-1111-111111111111 "
	command.NeighborhoodID = " 22222222-2222-2222-2222-222222222222 "
	command.SourceRef = " weekly-2026-07-09 "
	command.RawContentType = " application/json "
	command.Records[0].SourceRecordID = " listing-001 "
	command.Records[0].Layout = " 三房 "
	status := ListingStatus(" active ")
	command.Records[0].Status = &status

	normalized, issues := validateAndNormalize(command, command.CollectedAt)
	if len(issues) != 0 {
		t.Fatalf("issues = %#v, want none", issues)
	}
	if normalized.DataSourceID != "11111111-1111-1111-1111-111111111111" || normalized.NeighborhoodID != "22222222-2222-2222-2222-222222222222" || normalized.SourceRef != "weekly-2026-07-09" || normalized.RawContentType != "application/json" {
		t.Fatalf("normalized source fields were not trimmed: %#v", normalized)
	}
	if got := normalized.Listings[0]; got.SourceListingID != "listing-001" || got.Layout != "三房" || got.Status != ListingStatusActive {
		t.Fatalf("listing fields were not trimmed: %#v", got)
	}
}

func validImportCollectionRunCommand() ImportCollectionRunCommand {
	now := time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC)
	return ImportCollectionRunCommand{
		DataSourceID:   "11111111-1111-1111-1111-111111111111",
		NeighborhoodID: "22222222-2222-2222-2222-222222222222",
		SourceRef:      "weekly-2026-07-09",
		CollectedAt:    now,
		Coverage:       domainneighborhood.CoverageFull,
		Format:         ImportFormatJSON,
		RawPayload:     []byte(`{"records":1}`),
		RawContentType: "application/json",
		Records:        []ObservationInput{validListingInput(1)},
	}
}

func validListingInput(row int) ObservationInput {
	price := 520000.25
	daysOnMarket := 14
	status := ListingStatusActive
	return ObservationInput{
		Row:            row,
		RecordType:     RecordTypeListing,
		SourceRecordID: "listing-001",
		Layout:         "三房",
		AreaSQM:        88.5,
		ListingPrice:   &price,
		DaysOnMarket:   &daysOnMarket,
		Status:         &status,
	}
}

func validTransactionInput(row int) ObservationInput {
	price := 498000.75
	transactionDate := time.Date(2026, 7, 8, 12, 0, 0, 0, time.UTC)
	return ObservationInput{
		Row:              row,
		RecordType:       RecordTypeTransaction,
		SourceRecordID:   "txn-001",
		Layout:           "两房",
		AreaSQM:          66,
		TransactionPrice: &price,
		TransactionDate:  &transactionDate,
	}
}

type issueKey struct {
	row   *int
	field string
	code  string
}

func assertIssues(t *testing.T, got []ValidationIssue, want []issueKey) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("issues = %#v, want %d issues", got, len(want))
	}
	for i := range want {
		if !sameRow(got[i].Row, want[i].row) || got[i].Field != want[i].field || got[i].Code != want[i].code {
			t.Fatalf("issue[%d] = %#v, want row=%v field=%q code=%q", i, got[i], want[i].row, want[i].field, want[i].code)
		}
		if got[i].Message == "" {
			t.Fatalf("issue[%d] message is empty", i)
		}
	}
}

func sameRow(got, want *int) bool {
	if got == nil || want == nil {
		return got == nil && want == nil
	}
	return *got == *want
}

func intPtr(value int) *int {
	return &value
}
