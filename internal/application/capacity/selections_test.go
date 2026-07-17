package capacity

import (
	"context"
	"errors"
	"testing"
	"time"

	appasset "github.com/sine-io/propulse/internal/application/asset"
	appcommunitymarket "github.com/sine-io/propulse/internal/application/communitymarket"
	domainasset "github.com/sine-io/propulse/internal/domain/asset"
	domainneighborhood "github.com/sine-io/propulse/internal/domain/neighborhood"
)

func TestCreateCalculationFreezesAssetAndLatestTargetListingSnapshots(t *testing.T) {
	now := time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC)
	repo := &memoryCalculationRepository{nextID: "calc-selection", createdAt: now}
	policyRepo := &memoryPolicyRepository{effective: applicationTestPolicy()}
	assetReader := selectionAssetReader{asset: capacityTestAsset(now)}
	listingReader := selectionListingReader{detail: capacityTestListing(now)}
	service := NewServiceWithPoliciesAndSelections(repo, policyRepo, assetReader, listingReader, testAssumptions(), repo.now, repo.newID)
	input := applicationPolicyInput()
	input.OldHomeValue = 320
	input.OldLoanBalance = 60
	input.TargetTotalPrice = 480

	record, err := service.CreateCalculation(context.Background(), CreateCalculationCommand{
		UserID: "user-a", Input: input,
		OldHomeSelection: &OldHomeSelectionInput{
			Mode: OldHomeAsset, AssetID: assetReader.asset.ID, ExpectedSalePriceWan: capacityFloat(320), PriceConfirmed: true,
		},
		TargetHomeSelection: &TargetHomeSelectionInput{
			NeighborhoodID: listingReader.detail.NeighborhoodID, RoomID: listingReader.detail.RoomID,
			ExpectedPurchasePriceWan: capacityFloat(480), PriceConfirmed: true,
		},
	})
	if err != nil {
		t.Fatalf("CreateCalculation() error = %v", err)
	}
	if record.SelectionContext == nil || record.SelectionContext.OldHome == nil || record.SelectionContext.TargetHome == nil {
		t.Fatalf("SelectionContext = %#v", record.SelectionContext)
	}
	if record.Input.TransactionScenario.OldHomeHoldingYears != 5 || record.Input.TransactionScenario.OldHomeOriginalPrice != 180 ||
		record.Input.TransactionScenario.TargetHomeAreaSQM != 118 {
		t.Fatalf("resolved scenario = %#v", record.Input.TransactionScenario)
	}
	if record.SelectionContext.OldHome.PriceDifferenceWan == nil || *record.SelectionContext.OldHome.PriceDifferenceWan != 0 ||
		record.SelectionContext.TargetHome.PriceDifferenceWan != -20 ||
		record.SelectionContext.TargetHome.MarketReference.CollectionRunID != listingReader.detail.CollectionRunID {
		t.Fatalf("frozen selection context = %#v", record.SelectionContext)
	}
	if saved := repo.records[record.ID]; saved.SelectionContext.TargetHome.Property.NeighborhoodName != "海河花园" {
		t.Fatalf("saved selection context = %#v", saved.SelectionContext)
	}
}

func TestCreateCalculationRejectsDeletedAssetBeforeCalculationPersistence(t *testing.T) {
	repo := &memoryCalculationRepository{nextID: "unused", createdAt: time.Now()}
	service := NewServiceWithPoliciesAndSelections(
		repo, &memoryPolicyRepository{effective: applicationTestPolicy()},
		selectionAssetReader{err: appasset.ErrAssetNotFound}, selectionListingReader{}, testAssumptions(), repo.now, repo.newID,
	)
	input := applicationPolicyInput()
	_, err := service.CreateCalculation(context.Background(), CreateCalculationCommand{
		UserID: "user-a", Input: input,
		OldHomeSelection: &OldHomeSelectionInput{Mode: OldHomeAsset, AssetID: "11111111-1111-4111-8111-111111111111", ExpectedSalePriceWan: capacityFloat(input.OldHomeValue), PriceConfirmed: true},
	})
	if !errors.Is(err, ErrSelectedAssetNotFound) {
		t.Fatalf("CreateCalculation() error = %v, want ErrSelectedAssetNotFound", err)
	}
	if len(repo.records) != 0 {
		t.Fatalf("saved records = %d, want 0", len(repo.records))
	}
}

func TestCreateCalculationNoOldHomeSelectionForcesZeroOldHomeInputs(t *testing.T) {
	now := time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC)
	repo := &memoryCalculationRepository{nextID: "calc-none", createdAt: now}
	service := NewServiceWithPoliciesAndSelections(repo, &memoryPolicyRepository{effective: applicationTestPolicy()}, nil, nil, testAssumptions(), repo.now, repo.newID)
	input := applicationPolicyInput()
	record, err := service.CreateCalculation(context.Background(), CreateCalculationCommand{
		UserID: "user-a", Input: input, OldHomeSelection: &OldHomeSelectionInput{Mode: OldHomeNone},
	})
	if err != nil {
		t.Fatal(err)
	}
	if record.Input.OldHomeValue != 0 || record.Input.OldLoanBalance != 0 || record.Input.TransactionScenario.OldHomeOriginalPrice != 0 ||
		record.SelectionContext == nil || record.SelectionContext.OldHome.Mode != OldHomeNone {
		t.Fatalf("no-old-home record = %#v", record)
	}
}

type selectionAssetReader struct {
	asset domainasset.Asset
	err   error
}

func (reader selectionAssetReader) GetAsset(context.Context, appasset.GetAssetQuery) (domainasset.Asset, error) {
	return reader.asset, reader.err
}

type selectionListingReader struct {
	detail appcommunitymarket.MarketListingDetail
	err    error
}

func (reader selectionListingReader) GetListing(context.Context, appcommunitymarket.GetListingQuery) (appcommunitymarket.MarketListingDetail, error) {
	return reader.detail, reader.err
}

func capacityTestAsset(now time.Time) domainasset.Asset {
	price := 320.0
	return domainasset.Asset{
		ID: "11111111-1111-4111-8111-111111111111", UserID: "user-a", Name: "现住房",
		Property: domainasset.PropertySnapshot{
			NeighborhoodID: "22222222-2222-4222-8222-222222222222", NeighborhoodName: "梅江家园",
			City: "天津", District: "西青区", Layout: "2室1厅", AreaSQM: 82,
			FloorDescription: "中楼层/18层", Orientation: "南北", CurrentListingPriceWan: &price,
		},
		OriginalPurchasePriceWan: 180, PurchasedOn: time.Date(2020, 8, 20, 0, 0, 0, 0, time.UTC),
		CurrentLoanBalanceWan: 60, SourceKind: domainasset.SourceManual,
		CreatedAt: now.AddDate(0, -1, 0), UpdatedAt: now.Add(-24 * time.Hour),
	}
}

func capacityTestListing(now time.Time) appcommunitymarket.MarketListingDetail {
	return appcommunitymarket.MarketListingDetail{
		MarketListing: appcommunitymarket.MarketListing{
			RoomID: "room-1", Layout: "3室2厅", AreaSQM: 118, ListingTotalPriceWan: 500,
			ListingUnitPrice: 42373, ListedAt: now.Add(-20 * 24 * time.Hour), FloorBand: "中楼层",
			FloorDescription: "中楼层/20层", Orientation: "南北",
		},
		NeighborhoodID: "33333333-3333-4333-8333-333333333333", NeighborhoodName: "海河花园",
		City: "天津", District: "河西区", Status: "active",
		SnapshotID: "44444444-4444-4444-8444-444444444444", CollectionRunID: "55555555-5555-4555-8555-555555555555",
		CollectedAt: now.Add(-24 * time.Hour),
		Source: appcommunitymarket.MarketSource{
			DataSourceID: "66666666-6666-4666-8666-666666666666", DataSourceName: "房鉴",
			DataSourceType: "fangjian", SourceRef: "batch-1",
		},
		QualityStatus: "complete", Freshness: domainneighborhood.FreshnessCurrent,
	}
}

func capacityFloat(value float64) *float64 { return &value }
