package asset

import (
	"context"
	"errors"
	"sort"
	"testing"
	"time"

	appcommunitymarket "github.com/sine-io/propulse/internal/application/communitymarket"
	appneighborhood "github.com/sine-io/propulse/internal/application/neighborhood"
	domainasset "github.com/sine-io/propulse/internal/domain/asset"
	domainneighborhood "github.com/sine-io/propulse/internal/domain/neighborhood"
)

const (
	testAssetID        = "11111111-1111-4111-8111-111111111111"
	testNeighborhoodID = "22222222-2222-4222-8222-222222222222"
)

func TestCreateAssetFreezesAuthoritativeListingAndIgnoresClientPropertyFields(t *testing.T) {
	now := time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC)
	repo := newMemoryAssetRepository()
	service := NewService(repo, fakeNeighborhoodReader{}, fakeListingReader{detail: testListingDetail(now)}, func() time.Time { return now }, func() string { return testAssetID })

	created, err := service.CreateAsset(context.Background(), CreateAssetCommand{
		UserID: "user-a", NeighborhoodID: testNeighborhoodID,
		PropertySelection: PropertySelectionInput{
			Mode: PropertySelectionMarketListing, RoomID: "room-1",
			Layout: "伪造户型", AreaSQM: 999, CurrentListingPriceWan: floatPtr(1),
		},
		OriginalPurchasePriceWan: 180, PurchasedOn: "2020-08-20", CurrentLoanBalanceWan: 60,
	})
	if err != nil {
		t.Fatalf("CreateAsset() error = %v", err)
	}
	if created.Name != "海河花园 3室2厅" || created.Property.Layout != "3室2厅" || created.Property.AreaSQM != 118 ||
		created.Property.CurrentListingPriceWan == nil || *created.Property.CurrentListingPriceWan != 500 {
		t.Fatalf("created authoritative snapshot = %#v", created)
	}
	if created.ListingSource == nil || created.ListingSource.SourceListingID != "room-1" || created.ListingSource.CollectionRunID == "" {
		t.Fatalf("listing source snapshot = %#v", created.ListingSource)
	}
}

func TestAssetCRUDIsUserScopedAndSoftDeleteHidesTheRecord(t *testing.T) {
	now := time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC)
	repo := newMemoryAssetRepository()
	service := NewService(repo, fakeNeighborhoodReader{}, fakeListingReader{}, func() time.Time { return now }, func() string { return testAssetID })
	created, err := service.CreateAsset(context.Background(), CreateAssetCommand{
		UserID: "user-a", NeighborhoodID: testNeighborhoodID,
		PropertySelection:        PropertySelectionInput{Mode: PropertySelectionManual, Layout: "2室1厅", AreaSQM: 82},
		OriginalPurchasePriceWan: 180, PurchasedOn: "2020-08-20", CurrentLoanBalanceWan: 60,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.GetAsset(context.Background(), GetAssetQuery{UserID: "user-b", ID: created.ID}); !errors.Is(err, ErrAssetNotFound) {
		t.Fatalf("cross-user GetAsset() error = %v, want ErrAssetNotFound", err)
	}
	newBalance := 55.0
	newName := "改善前住房"
	updated, err := service.UpdateAsset(context.Background(), UpdateAssetCommand{
		UserID: "user-a", ID: created.ID, Name: &newName, CurrentLoanBalanceWan: &newBalance,
	})
	if err != nil || updated.Name != newName || updated.CurrentLoanBalanceWan != newBalance {
		t.Fatalf("UpdateAsset() = %#v, %v", updated, err)
	}
	if err := service.DeleteAsset(context.Background(), DeleteAssetCommand{UserID: "user-a", ID: created.ID}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.GetAsset(context.Background(), GetAssetQuery{UserID: "user-a", ID: created.ID}); !errors.Is(err, ErrAssetNotFound) {
		t.Fatalf("deleted GetAsset() error = %v, want ErrAssetNotFound", err)
	}
	page, err := service.ListAssets(context.Background(), ListAssetsQuery{UserID: "user-a"})
	if err != nil || page.Total != 0 || len(page.Items) != 0 {
		t.Fatalf("ListAssets() = %#v, %v", page, err)
	}
}

func TestCreateAssetMapsUnavailableListingWithoutWriting(t *testing.T) {
	repo := newMemoryAssetRepository()
	service := NewService(repo, fakeNeighborhoodReader{}, fakeListingReader{err: appcommunitymarket.ErrListingUnavailable}, nil, nil)
	_, err := service.CreateAsset(context.Background(), CreateAssetCommand{
		UserID: "user-a", NeighborhoodID: testNeighborhoodID,
		PropertySelection:        PropertySelectionInput{Mode: PropertySelectionMarketListing, RoomID: "room-old"},
		OriginalPurchasePriceWan: 180, PurchasedOn: "2020-08-20", CurrentLoanBalanceWan: 60,
	})
	if !errors.Is(err, ErrListingUnavailable) {
		t.Fatalf("CreateAsset() error = %v, want ErrListingUnavailable", err)
	}
	if len(repo.items) != 0 {
		t.Fatalf("repository writes = %d, want 0", len(repo.items))
	}
}

type fakeNeighborhoodReader struct{}

func (fakeNeighborhoodReader) GetNeighborhood(context.Context, appneighborhood.GetNeighborhoodQuery) (appneighborhood.Neighborhood, error) {
	city := "天津"
	return appneighborhood.Neighborhood{ID: testNeighborhoodID, Name: "海河花园", City: &city, Area: "河西区", AvailableLayouts: []string{"2室1厅", "3室2厅"}}, nil
}

type fakeListingReader struct {
	detail appcommunitymarket.MarketListingDetail
	err    error
}

func (reader fakeListingReader) GetListing(context.Context, appcommunitymarket.GetListingQuery) (appcommunitymarket.MarketListingDetail, error) {
	return reader.detail, reader.err
}

func testListingDetail(now time.Time) appcommunitymarket.MarketListingDetail {
	return appcommunitymarket.MarketListingDetail{
		MarketListing: appcommunitymarket.MarketListing{
			RoomID: "room-1", Layout: "3室2厅", AreaSQM: 118, ListingTotalPriceWan: 500,
			ListingUnitPrice: 42373, ListedAt: now.Add(-20 * 24 * time.Hour), FloorBand: "中楼层",
			FloorDescription: "中楼层/20层", Orientation: "南北",
		},
		NeighborhoodID: testNeighborhoodID, NeighborhoodName: "海河花园", City: "天津", District: "河西区",
		Status: "active", SnapshotID: "33333333-3333-4333-8333-333333333333",
		CollectionRunID: "44444444-4444-4444-8444-444444444444", CollectedAt: now.Add(-24 * time.Hour),
		Source: appcommunitymarket.MarketSource{
			DataSourceID: "55555555-5555-4555-8555-555555555555", DataSourceName: "房鉴",
			DataSourceType: "fangjian", SourceRef: "batch-1",
		},
		QualityStatus: "complete", Freshness: domainneighborhood.FreshnessCurrent,
	}
}

type memoryAssetRepository struct {
	items map[string]domainasset.Asset
}

func newMemoryAssetRepository() *memoryAssetRepository {
	return &memoryAssetRepository{items: map[string]domainasset.Asset{}}
}

func (repo *memoryAssetRepository) Create(_ context.Context, asset domainasset.Asset) (domainasset.Asset, error) {
	repo.items[asset.ID] = asset
	return asset, nil
}

func (repo *memoryAssetRepository) Update(_ context.Context, asset domainasset.Asset) (domainasset.Asset, error) {
	current, ok := repo.items[asset.ID]
	if !ok || current.UserID != asset.UserID || current.DeletedAt != nil {
		return domainasset.Asset{}, ErrAssetNotFound
	}
	repo.items[asset.ID] = asset
	return asset, nil
}

func (repo *memoryAssetRepository) Find(_ context.Context, userID, id string) (domainasset.Asset, error) {
	asset, ok := repo.items[id]
	if !ok || asset.UserID != userID || asset.DeletedAt != nil {
		return domainasset.Asset{}, ErrAssetNotFound
	}
	return asset, nil
}

func (repo *memoryAssetRepository) List(_ context.Context, userID string, limit, offset int) ([]domainasset.Asset, int, error) {
	items := make([]domainasset.Asset, 0)
	for _, asset := range repo.items {
		if asset.UserID == userID && asset.DeletedAt == nil {
			items = append(items, asset)
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].UpdatedAt.After(items[j].UpdatedAt) })
	total := len(items)
	if offset >= total {
		return []domainasset.Asset{}, total, nil
	}
	end := min(offset+limit, total)
	return items[offset:end], total, nil
}

func (repo *memoryAssetRepository) SoftDelete(_ context.Context, userID, id string) error {
	asset, err := repo.Find(context.Background(), userID, id)
	if err != nil {
		return err
	}
	now := time.Now()
	asset.DeletedAt = &now
	repo.items[id] = asset
	return nil
}

func floatPtr(value float64) *float64 { return &value }
