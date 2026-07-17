package communitymarket

import (
	"context"
	"errors"
	"testing"
	"time"

	domaincommunitymarket "github.com/sine-io/propulse/internal/domain/communitymarket"
)

func TestImportFangjianNormalizesAndBuildsOneAtomicBatch(t *testing.T) {
	now := time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC)
	repo := &memoryRepository{dataSourceExists: true, neighborhoodExists: true}
	ids := 0
	service := NewService(repo, func() time.Time { return now }, func() string {
		ids++
		return []string{
			"11111111-1111-4111-8111-111111111119",
			"11111111-1111-4111-8111-111111111118",
			"11111111-1111-4111-8111-111111111117",
		}[ids-1]
	})
	command := validFangjianCommand(now)

	result, err := service.ImportFangjian(context.Background(), command)
	if err != nil {
		t.Fatalf("ImportFangjian() error = %v", err)
	}
	if repo.fangjianSaveCalls != 1 || !result.Snapshot.CollectionRunIDIsPresentForTest() {
		t.Fatalf("result/save calls = %#v/%d", result, repo.fangjianSaveCalls)
	}
	listing := repo.fangjianBatch.Listings[0]
	if listing.Layout != "二室" || listing.DaysOnMarket != 10 {
		t.Fatalf("normalized listing = %#v", listing)
	}
	if repo.fangjianBatch.Transactions[0].Layout != "三室" {
		t.Fatalf("normalized transaction = %#v", repo.fangjianBatch.Transactions[0])
	}
	if result.ListingCount != 1 || result.TransactionCount != 1 || result.AdjustmentCount != 1 {
		t.Fatalf("result counts = %#v", result)
	}
}

func TestImportFangjianRejectsIncompleteOrDuplicateBundleWithoutWriting(t *testing.T) {
	now := time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name   string
		mutate func(*ImportFangjianCommand)
	}{
		{name: "incomplete", mutate: func(command *ImportFangjianCommand) { command.Bundle.Quality.Status = "incomplete" }},
		{name: "warnings", mutate: func(command *ImportFangjianCommand) { command.Bundle.Quality.Warnings = []string{"leaf_limit"} }},
		{name: "duplicate listing", mutate: func(command *ImportFangjianCommand) {
			command.Bundle.Listings = append(command.Bundle.Listings, command.Bundle.Listings[0])
		}},
		{name: "invalid adjustment arithmetic", mutate: func(command *ImportFangjianCommand) { command.Bundle.Adjustments[0].AmountWan = 99 }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repo := &memoryRepository{dataSourceExists: true, neighborhoodExists: true}
			command := validFangjianCommand(now)
			test.mutate(&command)
			_, err := NewService(repo, func() time.Time { return now }, nil).ImportFangjian(context.Background(), command)
			var validationErr *ValidationError
			if !errors.As(err, &validationErr) || repo.fangjianSaveCalls != 0 {
				t.Fatalf("error/save calls = %v/%d", err, repo.fangjianSaveCalls)
			}
		})
	}
}

func TestMarketQueriesFilterSortAndPaginateLatestBatch(t *testing.T) {
	repo := &memoryRepository{listings: []MarketListing{
		{RoomID: "b", Layout: "二室", FloorBand: "高楼层", ListingTotalPriceWan: 80, ListingUnitPrice: 9000, AreaSQM: 89, ListedAt: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)},
		{RoomID: "a", Layout: "二室", FloorBand: "高楼层", ListingTotalPriceWan: 60, ListingUnitPrice: 8000, AreaSQM: 75, ListedAt: time.Date(2026, 7, 2, 0, 0, 0, 0, time.UTC)},
		{RoomID: "c", Layout: "三室", FloorBand: "低楼层", ListingTotalPriceWan: 120, ListingUnitPrice: 10000, AreaSQM: 120, ListedAt: time.Date(2026, 7, 3, 0, 0, 0, 0, time.UTC)},
	}}
	result, err := NewService(repo, nil, nil).ListListings(context.Background(), MarketListQuery{
		NeighborhoodID: "22222222-2222-4222-8222-222222222222", Layout: "2室1厅", Floor: "高楼层",
		MinPriceWan: floatPointerForTest(50), MaxPriceWan: floatPointerForTest(100), SortBy: "price", SortOrder: "asc", Page: 1, PageSize: 1,
	})
	if err != nil {
		t.Fatalf("ListListings() error = %v", err)
	}
	if result.Total != 2 || len(result.Items) != 1 || result.Items[0].RoomID != "a" {
		t.Fatalf("ListListings() = %#v", result)
	}
}

func validFangjianCommand(now time.Time) ImportFangjianCommand {
	collectedAt := now.Add(-time.Hour)
	return ImportFangjianCommand{
		DataSourceID: "11111111-1111-4111-8111-111111111111", NeighborhoodID: "22222222-2222-4222-8222-222222222222",
		SourceRef: "fangjian-test", RawPayload: []byte(`{"schemaVersion":"fangjian.bundle/v1"}`),
		Bundle: FangjianBundle{
			SchemaVersion: FangjianBundleSchemaVersion, CollectedAt: collectedAt,
			Community: domaincommunitymarket.SnapshotData{
				SourceCommunityID: "source-community", CommunityName: "测试花园", CityCode: "120100", CityName: "天津市",
				DistrictCode: "120111", DistrictName: "西青区", BlockCode: "block", BlockName: "大寺", Latitude: 39, Longitude: 117,
				Analysis:     []byte(`{"listingPrice":{},"tradeTrends":{},"priceDiff":{},"roomType":{},"tradeCycle":{},"supplyTrend":{},"tradeSummary":{},"zf":{},"adjustCondition":{},"adjustConditionSummary":{},"adjustDetailSummary":{},"hotIndex":{},"hotIndexCompare":{},"confidenceIndex":{}}`),
				Surroundings: []byte(`{"competitiveSummary":{},"competitiveProducts":{},"poi":[]}`), CityContext: []byte(`{"summary":{},"map":{}}`),
			},
			Listings:     []MarketListing{{RoomID: "listing-1", Layout: "2室1厅", AreaSQM: 70, ListingTotalPriceWan: 60, ListingUnitPrice: 8500, ListedAt: collectedAt.Add(-10 * 24 * time.Hour), AdjustmentCount: 1}},
			Transactions: []MarketTransaction{{RoomID: "trade-1", Layout: "3室2厅", AreaSQM: 100, ListingTotalPriceWan: 100, TradeTotalPriceWan: 90, TradeUnitPrice: 9000, TradeDate: collectedAt.Add(-24 * time.Hour), NegotiationWan: -10, NegotiationPercent: 10}},
			Adjustments:  []ListingAdjustment{{RoomID: "listing-1", AdjustedAt: collectedAt.Add(-24 * time.Hour), PriceBeforeWan: 65, PriceAfterWan: 60, AmountWan: -5}},
			Quality:      BundleQuality{Status: "complete", Warnings: []string{}},
		},
	}
}

func floatPointerForTest(value float64) *float64 { return &value }

func (snapshot Snapshot) CollectionRunIDIsPresentForTest() bool {
	return snapshot.CollectionRunID != nil && *snapshot.CollectionRunID != ""
}
