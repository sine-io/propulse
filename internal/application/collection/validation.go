package collection

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	domainneighborhood "github.com/sine-io/propulse/internal/domain/neighborhood"
)

const maxRawPayloadBytes = 2 * 1024 * 1024

func validateAndNormalize(command ImportCollectionRunCommand, now time.Time) (NormalizedImport, []ValidationIssue) {
	command.DataSourceID = strings.TrimSpace(command.DataSourceID)
	command.NeighborhoodID = strings.TrimSpace(command.NeighborhoodID)
	command.SourceRef = strings.TrimSpace(command.SourceRef)
	command.RawContentType = strings.TrimSpace(command.RawContentType)
	collectedAt := command.CollectedAt.UTC()
	now = now.UTC()

	var issues []ValidationIssue
	if _, err := uuid.Parse(command.DataSourceID); err != nil {
		issues = appendIssue(issues, nil, "dataSourceId", "invalid_uuid", "dataSourceId must be a UUID")
	}
	if _, err := uuid.Parse(command.NeighborhoodID); err != nil {
		issues = appendIssue(issues, nil, "neighborhoodId", "invalid_uuid", "neighborhoodId must be a UUID")
	}
	if command.SourceRef == "" {
		issues = appendIssue(issues, nil, "sourceRef", "required", "sourceRef is required")
	} else if utf8.RuneCountInString(command.SourceRef) > 256 {
		issues = appendIssue(issues, nil, "sourceRef", "too_long", "sourceRef must be at most 256 characters")
	}
	if command.CollectedAt.IsZero() {
		issues = appendIssue(issues, nil, "collectedAt", "required", "collectedAt is required")
	} else if collectedAt.After(now.Add(5 * time.Minute)) {
		issues = appendIssue(issues, nil, "collectedAt", "future", "collectedAt must not be more than five minutes in the future")
	}
	if command.Coverage != domainneighborhood.CoverageFull && command.Coverage != domainneighborhood.CoveragePartial {
		issues = appendIssue(issues, nil, "coverage", "invalid", "coverage must be full or partial")
	}
	if command.Format != ImportFormatJSON && command.Format != ImportFormatCSV {
		issues = appendIssue(issues, nil, "format", "invalid", "format must be json or csv")
	}
	if len(command.RawPayload) == 0 {
		issues = appendIssue(issues, nil, "rawPayload", "required", "rawPayload is required")
	} else if len(command.RawPayload) > maxRawPayloadBytes {
		issues = appendIssue(issues, nil, "rawPayload", "too_large", "rawPayload must be at most 2 MiB")
	}
	if command.RawContentType == "" {
		issues = appendIssue(issues, nil, "rawContentType", "required", "rawContentType is required")
	} else if len(command.RawContentType) > 255 {
		issues = appendIssue(issues, nil, "rawContentType", "too_long", "rawContentType must be at most 255 bytes")
	}
	if len(command.Records) == 0 {
		issues = appendIssue(issues, nil, "records", "required", "at least one record is required")
	} else if len(command.Records) > 500 {
		issues = appendIssue(issues, nil, "records", "too_many", "at most 500 records are allowed")
	}
	if len(command.Records) > 500 {
		return NormalizedImport{}, issues
	}

	listings := make([]ListingObservation, 0, len(command.Records))
	transactions := make([]TransactionObservation, 0, len(command.Records))
	seen := make(map[string]struct{}, len(command.Records))
	for index, input := range command.Records {
		rowNumber := input.Row
		if rowNumber == 0 {
			rowNumber = index + 1
		}
		row := rowNumber
		rowPtr := &row
		input.SourceRecordID = strings.TrimSpace(input.SourceRecordID)
		input.Layout = strings.TrimSpace(input.Layout)
		if input.Status != nil {
			status := ListingStatus(strings.TrimSpace(string(*input.Status)))
			input.Status = &status
		}
		if input.OriginalListingRef != nil {
			ref := strings.TrimSpace(*input.OriginalListingRef)
			input.OriginalListingRef = &ref
		}

		rowIssuesStart := len(issues)
		if input.RecordType != RecordTypeListing && input.RecordType != RecordTypeTransaction {
			issues = appendIssue(issues, rowPtr, "recordType", "invalid", "recordType must be listing or transaction")
		}
		if input.SourceRecordID == "" {
			issues = appendIssue(issues, rowPtr, "sourceRecordId", "required", "sourceRecordId is required")
		} else if utf8.RuneCountInString(input.SourceRecordID) > 128 {
			issues = appendIssue(issues, rowPtr, "sourceRecordId", "too_long", "sourceRecordId must be at most 128 characters")
		} else {
			key := string(input.RecordType) + "\x00" + input.SourceRecordID
			if _, ok := seen[key]; ok {
				issues = appendIssue(issues, rowPtr, "sourceRecordId", "duplicate", "sourceRecordId must be unique for the record type")
			} else if input.RecordType == RecordTypeListing || input.RecordType == RecordTypeTransaction {
				seen[key] = struct{}{}
			}
		}
		if input.Layout == "" {
			issues = appendIssue(issues, rowPtr, "layout", "required", "layout is required")
		} else if utf8.RuneCountInString(input.Layout) > 64 {
			issues = appendIssue(issues, rowPtr, "layout", "too_long", "layout must be at most 64 characters")
		}
		if !isFinite(input.AreaSQM) || input.AreaSQM <= 0 || input.AreaSQM > 10000 {
			issues = appendIssue(issues, rowPtr, "areaSqm", "out_of_range", "areaSqm must be in (0, 10000]")
		}
		if input.OriginalListingRef != nil && utf8.RuneCountInString(*input.OriginalListingRef) > 128 {
			issues = appendIssue(issues, rowPtr, "originalListingRef", "too_long", "originalListingRef must be at most 128 characters")
		}

		switch input.RecordType {
		case RecordTypeListing:
			issues = validateListingInput(issues, rowPtr, input)
		case RecordTypeTransaction:
			issues = validateTransactionInput(issues, rowPtr, input, collectedAt)
		}
		issues = validateAttributes(issues, rowPtr, input.Attributes)

		if len(issues) != rowIssuesStart {
			continue
		}
		switch input.RecordType {
		case RecordTypeListing:
			listings = append(listings, ListingObservation{
				SourceListingID: input.SourceRecordID,
				SourceRow:       rowNumber,
				Layout:          input.Layout,
				AreaSQM:         input.AreaSQM,
				ListingPrice:    *input.ListingPrice,
				DaysOnMarket:    *input.DaysOnMarket,
				Status:          *input.Status,
				Attributes:      copyAttributes(input.Attributes),
			})
		case RecordTypeTransaction:
			var originalRef *string
			if input.OriginalListingRef != nil && *input.OriginalListingRef != "" {
				ref := *input.OriginalListingRef
				originalRef = &ref
			}
			transactions = append(transactions, TransactionObservation{
				SourceRecordID:     input.SourceRecordID,
				SourceRow:          rowNumber,
				Layout:             input.Layout,
				AreaSQM:            input.AreaSQM,
				TransactionPrice:   *input.TransactionPrice,
				TransactionDate:    input.TransactionDate.UTC(),
				OriginalListingRef: originalRef,
			})
		}
	}

	if len(issues) > 0 {
		return NormalizedImport{}, issues
	}
	normalized := NormalizedImport{
		DataSourceID:   command.DataSourceID,
		NeighborhoodID: command.NeighborhoodID,
		SourceRef:      command.SourceRef,
		CollectedAt:    collectedAt,
		Coverage:       command.Coverage,
		Format:         command.Format,
		RawPayload:     append([]byte(nil), command.RawPayload...),
		RawContentType: command.RawContentType,
		Listings:       listings,
		Transactions:   transactions,
	}
	normalized.ValidationSummary = ValidationSummary{
		RecordCount:      len(command.Records),
		ListingCount:     len(listings),
		TransactionCount: len(transactions),
		Issues:           nil,
	}
	return normalized, nil
}

func validateListingInput(issues []ValidationIssue, row *int, input ObservationInput) []ValidationIssue {
	if input.ListingPrice == nil {
		issues = appendIssue(issues, row, "listingPrice", "required", "listingPrice is required for listing rows")
	} else {
		issues = validatePrice(issues, row, "listingPrice", *input.ListingPrice)
	}
	if input.TransactionPrice != nil {
		issues = appendIssue(issues, row, "transactionPrice", "forbidden", "transactionPrice is not allowed for listing rows")
	}
	if input.TransactionDate != nil {
		issues = appendIssue(issues, row, "transactionDate", "forbidden", "transactionDate is not allowed for listing rows")
	}
	if input.DaysOnMarket == nil {
		issues = appendIssue(issues, row, "daysOnMarket", "required", "daysOnMarket is required for listing rows")
	} else if *input.DaysOnMarket < 0 || *input.DaysOnMarket > 36500 {
		issues = appendIssue(issues, row, "daysOnMarket", "out_of_range", "daysOnMarket must be between 0 and 36500")
	}
	if input.Status == nil {
		issues = appendIssue(issues, row, "status", "required", "status is required for listing rows")
	} else if !allowedListingStatus(*input.Status) {
		issues = appendIssue(issues, row, "status", "invalid", "status must be active, pending, withdrawn, or sold")
	}
	if input.OriginalListingRef != nil && *input.OriginalListingRef != "" {
		issues = appendIssue(issues, row, "originalListingRef", "forbidden", "originalListingRef is not allowed for listing rows")
	}
	return issues
}

func validateTransactionInput(issues []ValidationIssue, row *int, input ObservationInput, collectedAt time.Time) []ValidationIssue {
	if input.ListingPrice != nil {
		issues = appendIssue(issues, row, "listingPrice", "forbidden", "listingPrice is not allowed for transaction rows")
	}
	if input.DaysOnMarket != nil {
		issues = appendIssue(issues, row, "daysOnMarket", "forbidden", "daysOnMarket is not allowed for transaction rows")
	}
	if input.Status != nil {
		issues = appendIssue(issues, row, "status", "forbidden", "status is not allowed for transaction rows")
	}
	if input.TransactionPrice == nil {
		issues = appendIssue(issues, row, "transactionPrice", "required", "transactionPrice is required for transaction rows")
	} else {
		issues = validatePrice(issues, row, "transactionPrice", *input.TransactionPrice)
	}
	if input.TransactionDate == nil {
		issues = appendIssue(issues, row, "transactionDate", "required", "transactionDate is required for transaction rows")
	} else if input.TransactionDate.UTC().After(endOfUTCDate(collectedAt)) {
		issues = appendIssue(issues, row, "transactionDate", "future", "transactionDate must be no later than the collection date")
	}
	return issues
}

func validateAttributes(issues []ValidationIssue, row *int, attributes map[string]string) []ValidationIssue {
	if len(attributes) > 20 {
		issues = appendIssue(issues, row, "attributes", "too_many", "attributes must contain at most 20 pairs")
	}
	keys := make([]string, 0, len(attributes))
	for key := range attributes {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		trimmedKey := strings.TrimSpace(key)
		value := attributes[key]
		if trimmedKey == "" {
			issues = appendIssue(issues, row, "attributes.key", "required", "attribute keys are required")
		} else if utf8.RuneCountInString(trimmedKey) > 64 {
			issues = appendIssue(issues, row, "attributes.key", "too_long", "attribute keys must be at most 64 characters")
		}
		if utf8.RuneCountInString(value) > 512 {
			issues = appendIssue(issues, row, "attributes.value", "too_long", "attribute values must be at most 512 characters")
		}
	}
	return issues
}

func validatePrice(issues []ValidationIssue, row *int, field string, value float64) []ValidationIssue {
	if !isFinite(value) || value <= 0 {
		return appendIssue(issues, row, field, "out_of_range", fmt.Sprintf("%s must be positive", field))
	}
	if !hasAtMostTwoDecimals(value) {
		return appendIssue(issues, row, field, "too_many_decimals", fmt.Sprintf("%s must have at most two decimal places", field))
	}
	return issues
}

func appendIssue(issues []ValidationIssue, row *int, field, code, message string) []ValidationIssue {
	var rowCopy *int
	if row != nil {
		value := *row
		rowCopy = &value
	}
	return append(issues, ValidationIssue{Row: rowCopy, Field: field, Code: code, Message: message})
}

func allowedListingStatus(status ListingStatus) bool {
	switch status {
	case ListingStatusActive, ListingStatusPending, ListingStatusWithdrawn, ListingStatusSold:
		return true
	default:
		return false
	}
}

func isFinite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

func hasAtMostTwoDecimals(value float64) bool {
	scaled := value * 100
	return math.Abs(scaled-math.Round(scaled)) < 1e-9
}

func endOfUTCDate(value time.Time) time.Time {
	utc := value.UTC()
	return time.Date(utc.Year(), utc.Month(), utc.Day(), 23, 59, 59, int(time.Second-time.Nanosecond), time.UTC)
}

func copyAttributes(attributes map[string]string) map[string]string {
	if len(attributes) == 0 {
		return nil
	}
	copied := make(map[string]string, len(attributes))
	for key, value := range attributes {
		copied[strings.TrimSpace(key)] = value
	}
	return copied
}
