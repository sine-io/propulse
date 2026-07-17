package gormrepo

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	appcommunitymarket "github.com/sine-io/propulse/internal/application/communitymarket"
	domaincommunitymarket "github.com/sine-io/propulse/internal/domain/communitymarket"
)

func TestCommunityMarketRepositoryPersistsProfilesReplaysAndPrefersLaterCompleteImport(t *testing.T) {
	ctx, db, collectionRepo := openCollectionRepositoryTest(t)
	source, neighborhood := createCollectionRepositoryFixtures(t, ctx, collectionRepo, db)
	repo := NewCommunityMarketRepository(db)
	collectedAt := time.Date(2026, 7, 16, 12, 58, 57, 0, time.UTC)

	v1 := communityMarketRepositorySnapshot(source.ID, neighborhood.ID, collectedAt)
	v1.ID = uuid.NewString()
	v1.SourceRef = "fangjian-profile-test-v1-" + uuid.NewString()
	v1.ContentChecksum = strings.Repeat("a", 64)
	v1.CreatedAt = collectedAt.Add(time.Minute)
	v1.RawPayload = []byte("v1")
	first, err := repo.SaveSnapshot(ctx, v1)
	if err != nil || !first.Created {
		t.Fatalf("SaveSnapshot(v1) = %#v, %v", first, err)
	}

	full := communityMarketRepositorySnapshot(source.ID, neighborhood.ID, collectedAt)
	full.ID = uuid.NewString()
	full.SourceRef = "fangjian-profile-test-v2-" + uuid.NewString()
	full.ContentChecksum = strings.Repeat("b", 64)
	full.CreatedAt = collectedAt.Add(2 * time.Minute)
	full.RawPayload = []byte("v2")
	buildingCount, buildingYear, households, parkingSpaces := 11, 2012, 1089, 1550
	plotRatio, greenArea, greeningRate := 1.8, 14271.0, 40.0
	full.Data.ProvinceCode = "120000"
	full.Data.ProvinceName = "天津市"
	full.Data.PropertyType = "普通住宅"
	full.Data.PropertyTags = []string{"商品房", "私产"}
	full.Data.BuildingCount = &buildingCount
	full.Data.BuildingType = "板楼"
	full.Data.BuildingYear = &buildingYear
	full.Data.Developer = "天津耀华投资发展有限公司"
	full.Data.HouseholdCount = &households
	full.Data.ClosedManagement = "是"
	full.Data.PlotRatio = &plotRatio
	full.Data.GreenAreaSQM = &greenArea
	full.Data.GreeningRatePercent = &greeningRate
	full.Data.PropertyManagementCompany = "天津碧桂园物业有限公司"
	full.Data.PropertyFee = "2.3-2.9"
	full.Data.FixedParkingSpaces = &parkingSpaces
	full.Data.ParkingRatio = "1:0.7"
	full.Data.ParkingFee = "500"
	full.Data.HeatingType = "集中供暖"
	full.Data.WaterType = "民水"
	full.Data.ElectricityType = "民电"
	full.Data.GasCost = "2.5-2.61"
	full.Data.ManCarSeparation = "否"
	second, err := repo.SaveSnapshot(ctx, full)
	if err != nil || !second.Created {
		t.Fatalf("SaveSnapshot(v2) = %#v, %v", second, err)
	}

	replay := full
	replay.ID = uuid.NewString()
	replayed, err := repo.SaveSnapshot(ctx, replay)
	if err != nil || replayed.Created || replayed.Snapshot.ID != full.ID {
		t.Fatalf("SaveSnapshot(replay) = %#v, %v", replayed, err)
	}

	latest, err := repo.LatestSnapshot(ctx, neighborhood.ID)
	if err != nil {
		t.Fatalf("LatestSnapshot() error = %v", err)
	}
	if latest.ID != full.ID || latest.Data.BuildingCount == nil || *latest.Data.BuildingCount != 11 {
		t.Fatalf("latest snapshot = %#v, want complete later import %s", latest, full.ID)
	}
	if latest.Data.PropertyFee != "2.3-2.9" || len(latest.Data.PropertyTags) != 2 || latest.Data.PropertyTags[1] != "私产" {
		t.Fatalf("latest profile = %#v", latest.Data)
	}

	var legacy CommunityMarketSnapshotModel
	if err := db.WithContext(ctx).First(&legacy, "id = ?", v1.ID).Error; err != nil {
		t.Fatalf("Find(v1) error = %v", err)
	}
	if legacy.ProvinceName != nil || legacy.PropertyTags != nil || legacy.BuildingCount != nil {
		t.Fatalf("v1 optional profile columns = %#v, want NULL", legacy)
	}
}

func TestCommunityMarketRepositoryLatestReturnsNotFound(t *testing.T) {
	ctx, db, _ := openCollectionRepositoryTest(t)
	_, err := NewCommunityMarketRepository(db).LatestSnapshot(ctx, uuid.NewString())
	if !errors.Is(err, appcommunitymarket.ErrSnapshotNotFound) {
		t.Fatalf("LatestSnapshot() error = %v, want %v", err, appcommunitymarket.ErrSnapshotNotFound)
	}
}

func TestCommunityMarketRepositoryAtomicallyPersistsFangjianBundleAndReplays(t *testing.T) {
	ctx, db, collectionRepo := openCollectionRepositoryTest(t)
	source, neighborhood := createCollectionRepositoryFixtures(t, ctx, collectionRepo, db)
	repo := NewCommunityMarketRepository(db)
	collectedAt := time.Date(2026, 7, 17, 1, 0, 0, 0, time.UTC)
	runID := uuid.NewString()
	snapshot := communityMarketRepositorySnapshot(source.ID, neighborhood.ID, collectedAt)
	snapshot.ID = uuid.NewString()
	snapshot.SourceRef = "fangjian-bundle-" + uuid.NewString()
	snapshot.ContentChecksum = strings.Repeat("c", 64)
	snapshot.RawPayload = []byte(`{"schemaVersion":"fangjian.bundle/v1"}`)
	snapshot.RawContentType = "application/json"
	snapshot.CollectionRunID = &runID
	snapshot.QualityStatus = "complete"
	snapshot.CreatedAt = collectedAt.Add(time.Minute)
	snapshot.Data.Analysis = []byte(`{"tradeTrends":{"tradeTrends":[]}}`)
	snapshot.Data.Surroundings = []byte(`{"poi":[]}`)
	snapshot.Data.CityContext = []byte(`{"summary":{}}`)
	batch := appcommunitymarket.FangjianImportBatch{
		Snapshot: snapshot, CollectionRunID: runID,
		Listings:     []appcommunitymarket.MarketListing{{RoomID: "listing-1", Layout: "二室", AreaSQM: 70, ListingTotalPriceWan: 60, ListingUnitPrice: 8571, ListedAt: collectedAt.Add(-10 * 24 * time.Hour), DaysOnMarket: 10, FloorBand: "高楼层", FloorDescription: "高楼层(共18层)", Orientation: "南", AdjustmentCount: 1}},
		Transactions: []appcommunitymarket.MarketTransaction{{RoomID: "trade-1", Layout: "三室", AreaSQM: 100, ListingTotalPriceWan: 100, TradeTotalPriceWan: 90, TradeUnitPrice: 9000, TradeDate: collectedAt.Add(-24 * time.Hour), NegotiationWan: 10, NegotiationPercent: 10, FloorBand: "中楼层", Orientation: "南 北"}},
		Adjustments:  []appcommunitymarket.ListingAdjustment{{ID: uuid.NewString(), RoomID: "listing-1", AdjustedAt: collectedAt.Add(-24 * time.Hour), PriceBeforeWan: 65, PriceAfterWan: 60, AmountWan: -5}},
	}
	created, err := repo.SaveFangjian(ctx, batch)
	if err != nil || !created.Created {
		t.Fatalf("SaveFangjian() = %#v, %v", created, err)
	}
	listings, err := repo.LatestListings(ctx, neighborhood.ID)
	if err != nil || len(listings) != 1 || listings[0].ListingUnitPrice != 8571 || listings[0].AdjustmentCount != 1 {
		t.Fatalf("LatestListings() = %#v, %v", listings, err)
	}
	transactions, err := repo.LatestTransactions(ctx, neighborhood.ID)
	if err != nil || len(transactions) != 1 || transactions[0].ListingTotalPriceWan != 100 || transactions[0].Orientation != "南 北" {
		t.Fatalf("LatestTransactions() = %#v, %v", transactions, err)
	}
	adjustments, err := repo.LatestAdjustments(ctx, neighborhood.ID, "listing-1")
	if err != nil || len(adjustments) != 1 || adjustments[0].AmountWan != -5 {
		t.Fatalf("LatestAdjustments() = %#v, %v", adjustments, err)
	}
	replayBatch := batch
	replayRunID := uuid.NewString()
	replayBatch.CollectionRunID = replayRunID
	replayBatch.Snapshot.ID = uuid.NewString()
	replayBatch.Snapshot.CollectionRunID = &replayRunID
	replayed, err := repo.SaveFangjian(ctx, replayBatch)
	if err != nil || replayed.Created || replayed.Snapshot.ID != snapshot.ID {
		t.Fatalf("SaveFangjian(replay) = %#v, %v", replayed, err)
	}
}

func TestCommunityMarketRepositoryRollsBackWholeFangjianBundle(t *testing.T) {
	ctx, db, collectionRepo := openCollectionRepositoryTest(t)
	source, neighborhood := createCollectionRepositoryFixtures(t, ctx, collectionRepo, db)
	repo := NewCommunityMarketRepository(db)
	runID := uuid.NewString()
	snapshot := communityMarketRepositorySnapshot(source.ID, neighborhood.ID, time.Now().UTC())
	snapshot.ID, snapshot.SourceRef, snapshot.ContentChecksum = uuid.NewString(), "rollback-"+uuid.NewString(), strings.Repeat("d", 64)
	snapshot.RawPayload, snapshot.RawContentType, snapshot.CollectionRunID = []byte("bundle"), "application/json", &runID
	snapshot.QualityStatus, snapshot.CreatedAt = "complete", time.Now().UTC()
	_, err := repo.SaveFangjian(ctx, appcommunitymarket.FangjianImportBatch{
		Snapshot: snapshot, CollectionRunID: runID,
		Listings:    []appcommunitymarket.MarketListing{{RoomID: "listing", Layout: "二室", AreaSQM: 70, ListingTotalPriceWan: 60, ListedAt: time.Now().UTC()}},
		Adjustments: []appcommunitymarket.ListingAdjustment{{ID: uuid.NewString(), RoomID: "listing", AdjustedAt: time.Now().UTC(), PriceBeforeWan: 0, PriceAfterWan: 50, AmountWan: 50}},
	})
	if err == nil {
		t.Fatal("SaveFangjian() error = nil, want constraint failure")
	}
	var count int64
	if err := db.Model(&CollectionRunModel{}).Where("id = ?", runID).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("rolled back collection run count/error = %d/%v", count, err)
	}
}

func communityMarketRepositorySnapshot(dataSourceID, neighborhoodID string, collectedAt time.Time) appcommunitymarket.Snapshot {
	return appcommunitymarket.Snapshot{
		DataSourceID:   dataSourceID,
		NeighborhoodID: neighborhoodID,
		CollectedAt:    collectedAt,
		RawContentType: "text/csv",
		Data: domaincommunitymarket.SnapshotData{
			SourceCommunityID: "a2d56505411446cfe70fd3960beb19c7",
			CommunityName:     "富力津门湖鸣泉花园",
			CityCode:          "120100",
			CityName:          "天津市",
			DistrictCode:      "120111",
			DistrictName:      "西青区",
			BlockCode:         "BK2022112435579",
			BlockName:         "梅江南",
			Latitude:          39.057089,
			Longitude:         117.203624,
			OnSaleRoomTypes:   []string{},
		},
	}
}
