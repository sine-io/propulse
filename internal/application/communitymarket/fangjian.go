package communitymarket

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	domainneighborhood "github.com/sine-io/propulse/internal/domain/neighborhood"
)

const (
	MaxFangjianBundleBytes = 16 * 1024 * 1024
	defaultMarketPageSize  = 20
	maxMarketPageSize      = 100
)

func (s *Service) ImportFangjian(ctx context.Context, command ImportFangjianCommand) (ImportFangjianResult, error) {
	now := s.now().UTC()
	command.DataSourceID = strings.TrimSpace(command.DataSourceID)
	command.NeighborhoodID = strings.TrimSpace(command.NeighborhoodID)
	command.SourceRef = strings.TrimSpace(command.SourceRef)
	command.Bundle.Community = command.Bundle.Community.Normalize()
	command.Bundle.Listings = normalizeListings(command.Bundle.Listings, command.Bundle.CollectedAt)
	command.Bundle.Transactions = normalizeTransactions(command.Bundle.Transactions)
	command.Bundle.Adjustments = normalizeAdjustments(command.Bundle.Adjustments)

	issues := validateFangjianCommand(command, now)
	if len(issues) > 0 {
		return ImportFangjianResult{}, &ValidationError{Issues: issues}
	}
	exists, err := s.repo.DataSourceExists(ctx, command.DataSourceID)
	if err != nil {
		return ImportFangjianResult{}, fmt.Errorf("%w: %w", ErrImportFailed, err)
	}
	if !exists {
		return ImportFangjianResult{}, ErrDataSourceNotFound
	}
	exists, err = s.repo.NeighborhoodExists(ctx, command.NeighborhoodID)
	if err != nil {
		return ImportFangjianResult{}, fmt.Errorf("%w: %w", ErrImportFailed, err)
	}
	if !exists {
		return ImportFangjianResult{}, ErrNeighborhoodNotFound
	}

	runID := s.newID()
	checksum := checksumFangjian(command)
	snapshot := Snapshot{
		ID:              s.newID(),
		DataSourceID:    command.DataSourceID,
		NeighborhoodID:  command.NeighborhoodID,
		SourceRef:       command.SourceRef,
		CollectedAt:     command.Bundle.CollectedAt.UTC(),
		ContentChecksum: checksum,
		RawPayload:      append([]byte(nil), command.RawPayload...),
		RawContentType:  "application/json",
		CollectionRunID: &runID,
		QualityStatus:   "complete",
		Data:            command.Bundle.Community,
		CreatedAt:       now,
	}
	for index := range command.Bundle.Adjustments {
		command.Bundle.Adjustments[index].ID = s.newID()
	}
	saved, err := s.repo.SaveFangjian(ctx, FangjianImportBatch{
		Snapshot: snapshot, CollectionRunID: runID,
		Listings: command.Bundle.Listings, Transactions: command.Bundle.Transactions,
		Adjustments: command.Bundle.Adjustments,
	})
	if err != nil {
		return ImportFangjianResult{}, fmt.Errorf("%w: %w", ErrImportFailed, err)
	}
	resultRunID := runID
	if saved.Snapshot.CollectionRunID != nil {
		resultRunID = *saved.Snapshot.CollectionRunID
	}
	return ImportFangjianResult{
		Snapshot: saved.Snapshot, CollectionRunID: resultRunID,
		ListingCount: len(command.Bundle.Listings), TransactionCount: len(command.Bundle.Transactions),
		AdjustmentCount: len(command.Bundle.Adjustments), IdempotentReplay: !saved.Created,
	}, nil
}

func (s *Service) ListListings(ctx context.Context, query MarketListQuery) (Page[MarketListing], error) {
	query, err := normalizeMarketQuery(query)
	if err != nil {
		return Page[MarketListing]{}, err
	}
	items, err := s.repo.LatestListings(ctx, query.NeighborhoodID)
	if err != nil {
		return Page[MarketListing]{}, err
	}
	filtered := make([]MarketListing, 0, len(items))
	for _, item := range items {
		if matchesMarketFilter(item.Layout, item.FloorBand, item.ListingTotalPriceWan, query) {
			filtered = append(filtered, item)
		}
	}
	sortListings(filtered, query.SortBy, query.SortOrder)
	return paginate(filtered, query.Page, query.PageSize), nil
}

func (s *Service) GetListing(ctx context.Context, query GetListingQuery) (MarketListingDetail, error) {
	query.NeighborhoodID = strings.TrimSpace(query.NeighborhoodID)
	query.RoomID = strings.TrimSpace(query.RoomID)
	if _, err := uuid.Parse(query.NeighborhoodID); err != nil || query.RoomID == "" || utf8.RuneCountInString(query.RoomID) > 128 {
		return MarketListingDetail{}, ErrInvalidQuery
	}
	detail, err := s.repo.LatestListing(ctx, query.NeighborhoodID, query.RoomID)
	if err != nil {
		return MarketListingDetail{}, err
	}
	detail.Freshness = listingFreshness(s.now(), detail.CollectedAt)
	return detail, nil
}

func listingFreshness(now, collectedAt time.Time) domainneighborhood.Freshness {
	if collectedAt.IsZero() {
		return domainneighborhood.FreshnessUnknown
	}
	age := now.Sub(collectedAt)
	if age <= 7*24*time.Hour {
		return domainneighborhood.FreshnessCurrent
	}
	if age <= 30*24*time.Hour {
		return domainneighborhood.FreshnessStale
	}
	return domainneighborhood.FreshnessExpired
}

func (s *Service) ListTransactions(ctx context.Context, query MarketListQuery) (Page[MarketTransaction], error) {
	query, err := normalizeMarketQuery(query)
	if err != nil {
		return Page[MarketTransaction]{}, err
	}
	items, err := s.repo.LatestTransactions(ctx, query.NeighborhoodID)
	if err != nil {
		return Page[MarketTransaction]{}, err
	}
	filtered := make([]MarketTransaction, 0, len(items))
	for _, item := range items {
		if matchesMarketFilter(item.Layout, item.FloorBand, item.TradeTotalPriceWan, query) {
			filtered = append(filtered, item)
		}
	}
	sortTransactions(filtered, query.SortBy, query.SortOrder)
	return paginate(filtered, query.Page, query.PageSize), nil
}

func (s *Service) ListingAdjustments(ctx context.Context, query ListingAdjustmentsQuery) ([]ListingAdjustment, error) {
	query.NeighborhoodID = strings.TrimSpace(query.NeighborhoodID)
	query.RoomID = strings.TrimSpace(query.RoomID)
	if _, err := uuid.Parse(query.NeighborhoodID); err != nil || query.RoomID == "" || utf8.RuneCountInString(query.RoomID) > 128 {
		return nil, ErrInvalidQuery
	}
	return s.repo.LatestAdjustments(ctx, query.NeighborhoodID, query.RoomID)
}

func (s *Service) Compare(ctx context.Context, query ComparisonQuery) (Comparison, error) {
	primaryID := strings.TrimSpace(query.NeighborhoodID)
	peerID := strings.TrimSpace(query.PeerNeighborhoodID)
	if _, err := uuid.Parse(primaryID); err != nil {
		return Comparison{}, ErrInvalidQuery
	}
	if _, err := uuid.Parse(peerID); err != nil || primaryID == peerID {
		return Comparison{}, ErrInvalidQuery
	}
	primary, err := s.repo.LatestSnapshot(ctx, primaryID)
	if err != nil {
		return Comparison{}, err
	}
	peer, err := s.repo.LatestSnapshot(ctx, peerID)
	if err != nil {
		return Comparison{}, err
	}
	return Comparison{
		Primary: primary, Peer: peer,
		ListingUnitPrice:  comparisonMetric(primary.Data.ListingAvgUnitPrice, peer.Data.ListingAvgUnitPrice),
		Supply:            comparisonMetric(intFloat(primary.Data.ListingCount), intFloat(peer.Data.ListingCount)),
		RecentTrades:      comparisonMetric(intFloat(primary.Data.TradeCount3Months), intFloat(peer.Data.TradeCount3Months)),
		ListingTradeGap:   comparisonMetric(listingTradeGap(primary), listingTradeGap(peer)),
		AverageTradeCycle: comparisonMetric(averageTradeCycle(primary.Data.Analysis), averageTradeCycle(peer.Data.Analysis)),
	}, nil
}

func validateFangjianCommand(command ImportFangjianCommand, now time.Time) []ValidationIssue {
	issues := make([]ValidationIssue, 0)
	if _, err := uuid.Parse(command.DataSourceID); err != nil {
		issues = append(issues, ValidationIssue{Field: "dataSourceId", Code: "invalid_uuid", Message: "dataSourceId must be a UUID"})
	}
	if _, err := uuid.Parse(command.NeighborhoodID); err != nil {
		issues = append(issues, ValidationIssue{Field: "neighborhoodId", Code: "invalid_uuid", Message: "neighborhoodId must be a UUID"})
	}
	if command.SourceRef == "" || utf8.RuneCountInString(command.SourceRef) > 256 {
		issues = append(issues, ValidationIssue{Field: "sourceRef", Code: "invalid", Message: "sourceRef must contain 1 to 256 characters"})
	}
	if len(command.RawPayload) == 0 || len(command.RawPayload) > MaxFangjianBundleBytes {
		issues = append(issues, ValidationIssue{Field: "bundle", Code: "invalid_size", Message: "bundle must contain at most 16 MiB"})
	}
	if command.Bundle.SchemaVersion != FangjianBundleSchemaVersion {
		issues = append(issues, ValidationIssue{Field: "bundle.schemaVersion", Code: "unsupported", Message: "unsupported Fangjian bundle schema version"})
	}
	if command.Bundle.Quality.Status != "complete" || len(command.Bundle.Quality.Warnings) > 0 {
		issues = append(issues, ValidationIssue{Field: "bundle.quality", Code: "incomplete", Message: "incomplete Fangjian bundles cannot be imported"})
	}
	if command.Bundle.CollectedAt.IsZero() || command.Bundle.CollectedAt.After(now.Add(5*time.Minute)) {
		issues = append(issues, ValidationIssue{Field: "bundle.collectedAt", Code: "invalid", Message: "collectedAt is required and cannot be in the future"})
	}
	for _, violation := range command.Bundle.Community.Validate(command.Bundle.CollectedAt) {
		issues = append(issues, ValidationIssue{Field: "bundle.community." + violation.Field, Code: violation.Code, Message: violation.Message})
	}
	for _, raw := range []struct {
		name  string
		value json.RawMessage
	}{{"analysis", command.Bundle.Community.Analysis}, {"surroundings", command.Bundle.Community.Surroundings}, {"cityContext", command.Bundle.Community.CityContext}} {
		if !validJSONObject(raw.value) {
			issues = append(issues, ValidationIssue{Field: "bundle.community." + raw.name, Code: "invalid_json", Message: raw.name + " must be a JSON object"})
		}
	}
	for _, required := range []struct {
		name string
		raw  json.RawMessage
		keys []string
	}{
		{"analysis", command.Bundle.Community.Analysis, []string{"listingPrice", "tradeTrends", "priceDiff", "roomType", "tradeCycle", "supplyTrend", "tradeSummary", "zf", "adjustCondition", "adjustConditionSummary", "adjustDetailSummary", "hotIndex", "hotIndexCompare", "confidenceIndex"}},
		{"surroundings", command.Bundle.Community.Surroundings, []string{"competitiveSummary", "competitiveProducts", "poi"}},
		{"cityContext", command.Bundle.Community.CityContext, []string{"summary", "map"}},
	} {
		if missing := missingJSONObjectKeys(required.raw, required.keys); len(missing) > 0 {
			issues = append(issues, ValidationIssue{Field: "bundle.community." + required.name, Code: "incomplete", Message: required.name + " is missing required collections: " + strings.Join(missing, ", ")})
		}
	}
	seenListings := map[string]struct{}{}
	for index, item := range command.Bundle.Listings {
		row := index + 1
		if _, duplicate := seenListings[item.RoomID]; item.RoomID == "" || utf8.RuneCountInString(item.RoomID) > 128 || duplicate {
			issues = append(issues, ValidationIssue{Row: &row, Field: "bundle.listings.roomId", Code: "duplicate_or_missing", Message: "listing roomId must be present and unique"})
		}
		seenListings[item.RoomID] = struct{}{}
		if item.Layout == "" || item.AreaSQM <= 0 || item.AreaSQM > 10000 || item.ListingTotalPriceWan <= 0 || item.ListingUnitPrice <= 0 || item.ListedAt.IsZero() || item.ListedAt.After(command.Bundle.CollectedAt) || item.DaysOnMarket < 0 || item.DaysOnMarket > 36500 || item.AdjustmentCount < 0 {
			issues = append(issues, ValidationIssue{Row: &row, Field: "bundle.listings", Code: "invalid", Message: "listing contains invalid identity, date, area, or price"})
		}
	}
	seenTransactions := map[string]struct{}{}
	for index, item := range command.Bundle.Transactions {
		row := index + 1
		if _, duplicate := seenTransactions[item.RoomID]; item.RoomID == "" || utf8.RuneCountInString(item.RoomID) > 128 || duplicate {
			issues = append(issues, ValidationIssue{Row: &row, Field: "bundle.transactions.roomId", Code: "duplicate_or_missing", Message: "transaction roomId must be present and unique"})
		}
		seenTransactions[item.RoomID] = struct{}{}
		if item.Layout == "" || item.AreaSQM <= 0 || item.AreaSQM > 10000 || item.ListingTotalPriceWan <= 0 || item.TradeTotalPriceWan <= 0 || item.TradeUnitPrice <= 0 || item.TradeDate.IsZero() || item.TradeDate.After(command.Bundle.CollectedAt) || item.AdjustmentCount < 0 {
			issues = append(issues, ValidationIssue{Row: &row, Field: "bundle.transactions", Code: "invalid", Message: "transaction contains invalid identity, date, area, or price"})
		}
	}
	seenAdjustments := map[string]struct{}{}
	for index, item := range command.Bundle.Adjustments {
		row := index + 1
		key := fmt.Sprintf("%s|%s|%.4f|%.4f", item.RoomID, item.AdjustedAt.Format("2006-01-02"), item.PriceBeforeWan, item.PriceAfterWan)
		_, duplicate := seenAdjustments[key]
		if item.RoomID == "" || utf8.RuneCountInString(item.RoomID) > 128 || duplicate || item.AdjustedAt.IsZero() || item.AdjustedAt.After(command.Bundle.CollectedAt) || item.PriceBeforeWan <= 0 || item.PriceAfterWan <= 0 || math.Abs((item.PriceAfterWan-item.PriceBeforeWan)-item.AmountWan) > 0.011 {
			issues = append(issues, ValidationIssue{Row: &row, Field: "bundle.adjustments", Code: "invalid", Message: "adjustment contains invalid or duplicate values"})
		}
		seenAdjustments[key] = struct{}{}
	}
	return issues
}

func normalizeListings(items []MarketListing, collectedAt time.Time) []MarketListing {
	result := append([]MarketListing(nil), items...)
	for index := range result {
		result[index].RoomID = strings.TrimSpace(result[index].RoomID)
		result[index].Layout = normalizeLayout(result[index].Layout)
		result[index].FloorBand = strings.TrimSpace(result[index].FloorBand)
		result[index].FloorDescription = strings.TrimSpace(result[index].FloorDescription)
		result[index].Orientation = strings.Join(strings.Fields(result[index].Orientation), " ")
		result[index].ListedAt = result[index].ListedAt.UTC()
		days := int(collectedAt.UTC().Sub(result[index].ListedAt).Hours() / 24)
		if days < 0 {
			days = 0
		}
		result[index].DaysOnMarket = days
	}
	return result
}

func normalizeTransactions(items []MarketTransaction) []MarketTransaction {
	result := append([]MarketTransaction(nil), items...)
	for index := range result {
		result[index].RoomID = strings.TrimSpace(result[index].RoomID)
		result[index].Layout = normalizeLayout(result[index].Layout)
		result[index].FloorBand = strings.TrimSpace(result[index].FloorBand)
		result[index].FloorDescription = strings.TrimSpace(result[index].FloorDescription)
		result[index].Orientation = strings.Join(strings.Fields(result[index].Orientation), " ")
		result[index].TradeDate = result[index].TradeDate.UTC()
	}
	return result
}

func normalizeAdjustments(items []ListingAdjustment) []ListingAdjustment {
	result := append([]ListingAdjustment(nil), items...)
	for index := range result {
		result[index].RoomID = strings.TrimSpace(result[index].RoomID)
		result[index].AdjustedAt = result[index].AdjustedAt.UTC()
	}
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].RoomID != result[j].RoomID {
			return result[i].RoomID < result[j].RoomID
		}
		return result[i].AdjustedAt.Before(result[j].AdjustedAt)
	})
	return result
}

func normalizeLayout(value string) string {
	value = strings.TrimSpace(value)
	rooms := ""
	for _, r := range value {
		if r >= '0' && r <= '9' {
			rooms += string(r)
			continue
		}
		break
	}
	roomCount, err := strconv.Atoi(rooms)
	if err != nil || roomCount < 1 {
		return value
	}
	names := []string{"", "一室", "二室", "三室", "四室", "五室", "六室", "七室", "八室", "九室"}
	if roomCount < len(names) {
		return names[roomCount]
	}
	return fmt.Sprintf("%d室", roomCount)
}

func checksumFangjian(command ImportFangjianCommand) string {
	hash := sha256.New()
	for _, value := range []string{command.DataSourceID, command.NeighborhoodID, command.SourceRef} {
		_, _ = fmt.Fprintf(hash, "%d:%s\n", len(value), value)
	}
	_, _ = hash.Write(command.RawPayload)
	return hex.EncodeToString(hash.Sum(nil))
}

func validJSONObject(raw json.RawMessage) bool {
	if !json.Valid(raw) {
		return false
	}
	trimmed := strings.TrimSpace(string(raw))
	return strings.HasPrefix(trimmed, "{")
}

func missingJSONObjectKeys(raw json.RawMessage, required []string) []string {
	var object map[string]json.RawMessage
	if json.Unmarshal(raw, &object) != nil {
		return append([]string(nil), required...)
	}
	missing := make([]string, 0)
	for _, key := range required {
		value, exists := object[key]
		if !exists || len(value) == 0 || string(value) == "null" {
			missing = append(missing, key)
		}
	}
	return missing
}

func normalizeMarketQuery(query MarketListQuery) (MarketListQuery, error) {
	query.NeighborhoodID = strings.TrimSpace(query.NeighborhoodID)
	query.Layout = normalizeLayout(query.Layout)
	query.Floor = strings.TrimSpace(query.Floor)
	query.SortBy = strings.TrimSpace(query.SortBy)
	query.SortOrder = strings.ToLower(strings.TrimSpace(query.SortOrder))
	if query.Page == 0 {
		query.Page = 1
	}
	if query.PageSize == 0 {
		query.PageSize = defaultMarketPageSize
	}
	if query.SortBy == "" {
		query.SortBy = "date"
	}
	if query.SortOrder == "" {
		query.SortOrder = "desc"
	}
	_, idErr := uuid.Parse(query.NeighborhoodID)
	validSort := map[string]bool{"date": true, "price": true, "unitPrice": true, "area": true, "adjustments": true}
	if idErr != nil || query.Page < 1 || query.PageSize < 1 || query.PageSize > maxMarketPageSize || !validSort[query.SortBy] || (query.SortOrder != "asc" && query.SortOrder != "desc") || (query.MinPriceWan != nil && *query.MinPriceWan < 0) || (query.MaxPriceWan != nil && *query.MaxPriceWan < 0) || (query.MinPriceWan != nil && query.MaxPriceWan != nil && *query.MinPriceWan > *query.MaxPriceWan) {
		return MarketListQuery{}, ErrInvalidQuery
	}
	return query, nil
}

func matchesMarketFilter(layout, floor string, price float64, query MarketListQuery) bool {
	return (query.Layout == "" || layout == query.Layout) &&
		(query.Floor == "" || floor == query.Floor) &&
		(query.MinPriceWan == nil || price >= *query.MinPriceWan) &&
		(query.MaxPriceWan == nil || price <= *query.MaxPriceWan)
}

func sortListings(items []MarketListing, field, order string) {
	sort.SliceStable(items, func(i, j int) bool {
		left, right := listingSortValue(items[i], field), listingSortValue(items[j], field)
		if left == right {
			return items[i].RoomID < items[j].RoomID
		}
		if order == "asc" {
			return left < right
		}
		return left > right
	})
}

func listingSortValue(item MarketListing, field string) float64 {
	switch field {
	case "price":
		return item.ListingTotalPriceWan
	case "unitPrice":
		return item.ListingUnitPrice
	case "area":
		return item.AreaSQM
	case "adjustments":
		return float64(item.AdjustmentCount)
	default:
		return float64(item.ListedAt.Unix())
	}
}

func sortTransactions(items []MarketTransaction, field, order string) {
	sort.SliceStable(items, func(i, j int) bool {
		left, right := transactionSortValue(items[i], field), transactionSortValue(items[j], field)
		if left == right {
			return items[i].RoomID < items[j].RoomID
		}
		if order == "asc" {
			return left < right
		}
		return left > right
	})
}

func transactionSortValue(item MarketTransaction, field string) float64 {
	switch field {
	case "price":
		return item.TradeTotalPriceWan
	case "unitPrice":
		return item.TradeUnitPrice
	case "area":
		return item.AreaSQM
	case "adjustments":
		return float64(item.AdjustmentCount)
	default:
		return float64(item.TradeDate.Unix())
	}
}

func paginate[T any](items []T, page, pageSize int) Page[T] {
	total := len(items)
	start := (page - 1) * pageSize
	if start >= total {
		return Page[T]{Items: []T{}, Total: total, Page: page, PageSize: pageSize}
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	return Page[T]{Items: append([]T(nil), items[start:end]...), Total: total, Page: page, PageSize: pageSize}
}

func comparisonMetric(primary, peer *float64) ComparisonMetric {
	metric := ComparisonMetric{Primary: primary, Peer: peer}
	if primary != nil && peer != nil {
		delta := roundMarketNumber(*primary - *peer)
		metric.Delta = &delta
	}
	return metric
}

func intFloat(value *int) *float64 {
	if value == nil {
		return nil
	}
	converted := float64(*value)
	return &converted
}

func listingTradeGap(snapshot Snapshot) *float64 {
	listing := snapshot.Data.ListingAvgUnitPrice
	trade := snapshot.Data.TradeUnitPrice3Months
	if trade == nil {
		trade = snapshot.Data.LatestTradeAvgUnitPrice
	}
	if listing == nil || trade == nil {
		return nil
	}
	gap := roundMarketNumber(*listing - *trade)
	return &gap
}

func roundMarketNumber(value float64) float64 {
	return math.Round(value*100) / 100
}

func averageTradeCycle(raw json.RawMessage) *float64 {
	var analysis map[string]json.RawMessage
	if json.Unmarshal(raw, &analysis) == nil {
		var section map[string]json.RawMessage
		if json.Unmarshal(analysis["tradeCycle"], &section) == nil {
			var rows []map[string]any
			if json.Unmarshal(section["tradeCycle6"], &rows) == nil {
				for index := len(rows) - 1; index >= 0; index-- {
					if value, ok := rows[index]["avgDealCycle"].(float64); ok {
						return &value
					}
				}
			}
		}
	}
	var value any
	if json.Unmarshal(raw, &value) != nil {
		return nil
	}
	values := make([]float64, 0)
	collectNumericKey(value, "tradeCycle", &values)
	collectNumericKey(value, "avgTradeCycle", &values)
	collectNumericKey(value, "avgDealCycle", &values)
	if len(values) == 0 {
		return nil
	}
	total := 0.0
	for _, item := range values {
		total += item
	}
	average := total / float64(len(values))
	return &average
}

func collectNumericKey(value any, key string, values *[]float64) {
	switch typed := value.(type) {
	case map[string]any:
		for name, child := range typed {
			if name == key {
				if number, ok := child.(float64); ok {
					*values = append(*values, number)
				}
			}
			collectNumericKey(child, key, values)
		}
	case []any:
		for _, child := range typed {
			collectNumericKey(child, key, values)
		}
	}
}
