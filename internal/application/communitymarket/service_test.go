package communitymarket

import (
	"context"
	"errors"
	"testing"
	"time"

	domaincommunitymarket "github.com/sine-io/propulse/internal/domain/communitymarket"
	domainneighborhood "github.com/sine-io/propulse/internal/domain/neighborhood"
)

func TestImportSnapshotValidatesAndPersistsSourceNativeAggregate(t *testing.T) {
	now := time.Date(2026, 7, 16, 13, 0, 0, 0, time.UTC)
	repo := &memoryRepository{dataSourceExists: true, neighborhoodExists: true}
	service := NewService(repo, func() time.Time { return now }, func() string { return "33333333-3333-4333-8333-333333333333" })
	listingCount := 46
	listingPrice := 22741.0
	tradeCount := 5
	tradePrice := 16780.0
	collectedAt := now.Add(-time.Hour)

	result, err := service.ImportSnapshot(context.Background(), ImportSnapshotCommand{
		DataSourceID:   "11111111-1111-4111-8111-111111111111",
		NeighborhoodID: "22222222-2222-4222-8222-222222222222",
		SourceRef:      " fangjian-2026-07-16 ",
		CollectedAt:    collectedAt,
		RawPayload:     []byte("aggregate,csv\n"),
		RawContentType: " text/csv ",
		Data: domaincommunitymarket.SnapshotData{
			SourceCommunityID:     " a2d56505411446cfe70fd3960beb19c7 ",
			CommunityName:         " 富力津门湖鸣泉花园 ",
			CityCode:              "120100",
			CityName:              "天津市",
			DistrictCode:          "120111",
			DistrictName:          "西青区",
			BlockCode:             "BK2022112435579",
			BlockName:             "梅江南",
			Latitude:              39.057089,
			Longitude:             117.203624,
			ListingCount:          &listingCount,
			ListingAvgUnitPrice:   &listingPrice,
			TradeCount3Months:     &tradeCount,
			TradeUnitPrice3Months: &tradePrice,
			OnSaleRoomTypes:       []string{"三室", " 三室 ", "二室"},
		},
	})
	if err != nil {
		t.Fatalf("ImportSnapshot() error = %v", err)
	}
	if result.IdempotentReplay || repo.saveCalls != 1 {
		t.Fatalf("result/saveCalls = %#v/%d", result, repo.saveCalls)
	}
	got := result.Snapshot
	if got.SourceRef != "fangjian-2026-07-16" || got.RawContentType != "text/csv" {
		t.Fatalf("normalized metadata = %#v", got)
	}
	if got.Data.SourceCommunityID != "a2d56505411446cfe70fd3960beb19c7" || got.Data.ListingCount == nil || *got.Data.ListingCount != 46 {
		t.Fatalf("snapshot data = %#v", got.Data)
	}
	if len(got.Data.OnSaleRoomTypes) != 2 || got.ContentChecksum == "" {
		t.Fatalf("room types/checksum = %#v/%q", got.Data.OnSaleRoomTypes, got.ContentChecksum)
	}
}

func TestImportSnapshotRejectsInvalidAggregateWithoutWriting(t *testing.T) {
	now := time.Date(2026, 7, 16, 13, 0, 0, 0, time.UTC)
	repo := &memoryRepository{dataSourceExists: true, neighborhoodExists: true}
	service := NewService(repo, func() time.Time { return now }, nil)
	negative := -1

	_, err := service.ImportSnapshot(context.Background(), ImportSnapshotCommand{
		DataSourceID:   "11111111-1111-4111-8111-111111111111",
		NeighborhoodID: "22222222-2222-4222-8222-222222222222",
		SourceRef:      "snapshot",
		CollectedAt:    now,
		RawPayload:     []byte("csv"),
		RawContentType: "text/csv",
		Data: domaincommunitymarket.SnapshotData{
			SourceCommunityID: "source-community", CommunityName: "鸣泉花园",
			CityCode: "120100", CityName: "天津市", DistrictCode: "120111", DistrictName: "西青区",
			BlockCode: "block", BlockName: "梅江南", Latitude: 39, Longitude: 117, ListingCount: &negative,
		},
	})
	var validationErr *ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("ImportSnapshot() error = %v, want ValidationError", err)
	}
	if repo.saveCalls != 0 {
		t.Fatalf("SaveSnapshot calls = %d, want 0", repo.saveCalls)
	}
}

func TestImportSnapshotRejectsMissingBindingsBeforeSaving(t *testing.T) {
	now := time.Date(2026, 7, 16, 13, 0, 0, 0, time.UTC)
	tests := []struct {
		name string
		repo *memoryRepository
		want error
	}{
		{name: "data source", repo: &memoryRepository{neighborhoodExists: true}, want: ErrDataSourceNotFound},
		{name: "neighborhood", repo: &memoryRepository{dataSourceExists: true}, want: ErrNeighborhoodNotFound},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewService(test.repo, func() time.Time { return now }, nil).ImportSnapshot(context.Background(), validImportCommand(now))
			if !errors.Is(err, test.want) {
				t.Fatalf("ImportSnapshot() error = %v, want %v", err, test.want)
			}
			if test.repo.saveCalls != 0 {
				t.Fatalf("SaveSnapshot calls = %d, want 0", test.repo.saveCalls)
			}
		})
	}
}

func TestImportSnapshotWrapsRepositoryFailures(t *testing.T) {
	now := time.Date(2026, 7, 16, 13, 0, 0, 0, time.UTC)
	backendErr := errors.New("database unavailable")
	tests := []struct {
		name string
		repo *memoryRepository
	}{
		{name: "data source lookup", repo: &memoryRepository{dataSourceErr: backendErr}},
		{name: "neighborhood lookup", repo: &memoryRepository{dataSourceExists: true, neighborhoodErr: backendErr}},
		{name: "save", repo: &memoryRepository{dataSourceExists: true, neighborhoodExists: true, saveErr: backendErr}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewService(test.repo, func() time.Time { return now }, nil).ImportSnapshot(context.Background(), validImportCommand(now))
			if !errors.Is(err, ErrImportFailed) || !errors.Is(err, backendErr) {
				t.Fatalf("ImportSnapshot() error = %v, want wrapped repository failure", err)
			}
		})
	}
}

func TestImportSnapshotReportsIdempotentReplayFromRepository(t *testing.T) {
	now := time.Date(2026, 7, 16, 13, 0, 0, 0, time.UTC)
	existing := Snapshot{ID: "44444444-4444-4444-8444-444444444444"}
	repo := &memoryRepository{
		dataSourceExists:   true,
		neighborhoodExists: true,
		saveResult:         &SaveSnapshotResult{Snapshot: existing, Created: false},
	}
	result, err := NewService(repo, func() time.Time { return now }, nil).ImportSnapshot(context.Background(), validImportCommand(now))
	if err != nil {
		t.Fatalf("ImportSnapshot() error = %v", err)
	}
	if !result.IdempotentReplay || result.Snapshot.ID != existing.ID {
		t.Fatalf("ImportSnapshot() result = %#v", result)
	}
}

func validImportCommand(collectedAt time.Time) ImportSnapshotCommand {
	return ImportSnapshotCommand{
		DataSourceID:   "11111111-1111-4111-8111-111111111111",
		NeighborhoodID: "22222222-2222-4222-8222-222222222222",
		SourceRef:      "snapshot",
		CollectedAt:    collectedAt,
		RawPayload:     []byte("csv"),
		RawContentType: "text/csv",
		Data: domaincommunitymarket.SnapshotData{
			SourceCommunityID: "source-community",
			CommunityName:     "鸣泉花园",
			CityCode:          "120100",
			CityName:          "天津市",
			DistrictCode:      "120111",
			DistrictName:      "西青区",
			BlockCode:         "block",
			BlockName:         "梅江南",
			Latitude:          39,
			Longitude:         117,
		},
	}
}

type memoryRepository struct {
	dataSourceExists   bool
	neighborhoodExists bool
	dataSourceErr      error
	neighborhoodErr    error
	saveErr            error
	saveResult         *SaveSnapshotResult
	saveCalls          int
	snapshot           Snapshot
	fangjianSaveCalls  int
	fangjianSaveResult *SaveFangjianResult
	fangjianBatch      FangjianImportBatch
	listings           []MarketListing
	listingDetail      *MarketListingDetail
	transactions       []MarketTransaction
	adjustments        []ListingAdjustment
}

func (r *memoryRepository) DataSourceExists(context.Context, string) (bool, error) {
	return r.dataSourceExists, r.dataSourceErr
}

func (r *memoryRepository) NeighborhoodExists(context.Context, string) (bool, error) {
	return r.neighborhoodExists, r.neighborhoodErr
}

func (r *memoryRepository) SaveSnapshot(_ context.Context, snapshot Snapshot) (SaveSnapshotResult, error) {
	r.saveCalls++
	r.snapshot = snapshot
	if r.saveErr != nil {
		return SaveSnapshotResult{}, r.saveErr
	}
	if r.saveResult != nil {
		return *r.saveResult, nil
	}
	return SaveSnapshotResult{Snapshot: snapshot, Created: true}, nil
}

func (r *memoryRepository) LatestSnapshot(context.Context, string) (Snapshot, error) {
	if r.snapshot.ID == "" {
		return Snapshot{}, ErrSnapshotNotFound
	}
	return r.snapshot, nil
}

func (r *memoryRepository) SaveFangjian(_ context.Context, batch FangjianImportBatch) (SaveFangjianResult, error) {
	r.fangjianSaveCalls++
	r.fangjianBatch = batch
	r.snapshot = batch.Snapshot
	if r.fangjianSaveResult != nil {
		return *r.fangjianSaveResult, nil
	}
	return SaveFangjianResult{Snapshot: batch.Snapshot, Created: true}, nil
}

func (r *memoryRepository) LatestListings(context.Context, string) ([]MarketListing, error) {
	return append([]MarketListing(nil), r.listings...), nil
}

func (r *memoryRepository) LatestListing(_ context.Context, neighborhoodID, roomID string) (MarketListingDetail, error) {
	if r.listingDetail != nil {
		return *r.listingDetail, nil
	}
	for _, listing := range r.listings {
		if listing.RoomID == roomID {
			return MarketListingDetail{MarketListing: listing, NeighborhoodID: neighborhoodID, CollectedAt: time.Now()}, nil
		}
	}
	return MarketListingDetail{}, ErrListingNotFound
}

func TestGetListingValidatesIdentifiersAndAddsFreshness(t *testing.T) {
	now := time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC)
	detail := MarketListingDetail{
		MarketListing:  MarketListing{RoomID: "room-1", Layout: "3室2厅", AreaSQM: 118, ListingTotalPriceWan: 500},
		NeighborhoodID: "22222222-2222-4222-8222-222222222222", CollectedAt: now.Add(-14 * 24 * time.Hour),
	}
	service := NewService(&memoryRepository{listingDetail: &detail}, func() time.Time { return now }, nil)
	got, err := service.GetListing(context.Background(), GetListingQuery{NeighborhoodID: detail.NeighborhoodID, RoomID: " room-1 "})
	if err != nil {
		t.Fatal(err)
	}
	if got.Freshness != domainneighborhood.FreshnessStale || got.RoomID != "room-1" {
		t.Fatalf("GetListing() = %#v", got)
	}
	if _, err := service.GetListing(context.Background(), GetListingQuery{NeighborhoodID: "bad", RoomID: "room-1"}); !errors.Is(err, ErrInvalidQuery) {
		t.Fatalf("invalid GetListing() error = %v", err)
	}
}

func (r *memoryRepository) LatestTransactions(context.Context, string) ([]MarketTransaction, error) {
	return append([]MarketTransaction(nil), r.transactions...), nil
}

func (r *memoryRepository) LatestAdjustments(context.Context, string, string) ([]ListingAdjustment, error) {
	return append([]ListingAdjustment(nil), r.adjustments...), nil
}
