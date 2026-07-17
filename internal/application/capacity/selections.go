package capacity

import (
	"context"
	"errors"
	"math"
	"strings"
	"time"

	appasset "github.com/sine-io/propulse/internal/application/asset"
	appcommunitymarket "github.com/sine-io/propulse/internal/application/communitymarket"
	domainasset "github.com/sine-io/propulse/internal/domain/asset"
	domaincapacity "github.com/sine-io/propulse/internal/domain/capacity"
)

type OldHomeSelectionMode string

const (
	OldHomeNone  OldHomeSelectionMode = "none"
	OldHomeAsset OldHomeSelectionMode = "asset"
)

type OldHomeSelectionInput struct {
	Mode                 OldHomeSelectionMode
	AssetID              string
	ExpectedSalePriceWan *float64
	PriceConfirmed       bool
}

type TargetHomeSelectionInput struct {
	NeighborhoodID           string
	RoomID                   string
	ExpectedPurchasePriceWan *float64
	PriceConfirmed           bool
}

type SelectionPropertySnapshot struct {
	NeighborhoodID           string   `json:"neighborhoodId"`
	NeighborhoodName         string   `json:"neighborhoodName"`
	City                     string   `json:"city"`
	District                 string   `json:"district"`
	Layout                   string   `json:"layout"`
	AreaSQM                  float64  `json:"areaSqm"`
	FloorBand                string   `json:"floorBand"`
	FloorDescription         string   `json:"floorDescription"`
	Orientation              string   `json:"orientation"`
	ReferenceListingPriceWan *float64 `json:"referenceListingPriceWan"`
}

type MarketReferenceSnapshot struct {
	SourceListingID string    `json:"sourceListingId"`
	DataSourceID    string    `json:"dataSourceId"`
	DataSourceName  string    `json:"dataSourceName"`
	DataSourceType  string    `json:"dataSourceType"`
	SourceRef       string    `json:"sourceRef"`
	CollectionRunID string    `json:"collectionRunId"`
	SnapshotID      string    `json:"snapshotId"`
	CollectedAt     time.Time `json:"collectedAt"`
	ListedAt        time.Time `json:"listedAt"`
	QualityStatus   string    `json:"qualityStatus"`
	Freshness       string    `json:"freshness"`
}

type OldHomeSelectionSnapshot struct {
	Mode                     OldHomeSelectionMode       `json:"mode"`
	AssetID                  *string                    `json:"assetId"`
	AssetName                string                     `json:"assetName"`
	Property                 *SelectionPropertySnapshot `json:"property"`
	OriginalPurchasePriceWan float64                    `json:"originalPurchasePriceWan"`
	PurchasedOn              string                     `json:"purchasedOn"`
	HoldingYears             int                        `json:"holdingYears"`
	ConfirmedSalePriceWan    float64                    `json:"confirmedSalePriceWan"`
	ConfirmedLoanBalanceWan  float64                    `json:"confirmedLoanBalanceWan"`
	PriceDifferenceWan       *float64                   `json:"priceDifferenceWan"`
	AssetUpdatedAt           *time.Time                 `json:"assetUpdatedAt"`
	MarketReference          *MarketReferenceSnapshot   `json:"marketReference"`
	ConfirmedAt              time.Time                  `json:"confirmedAt"`
}

type TargetHomeSelectionSnapshot struct {
	Property                  SelectionPropertySnapshot `json:"property"`
	ConfirmedPurchasePriceWan float64                   `json:"confirmedPurchasePriceWan"`
	PriceDifferenceWan        float64                   `json:"priceDifferenceWan"`
	MarketReference           MarketReferenceSnapshot   `json:"marketReference"`
	ConfirmedAt               time.Time                 `json:"confirmedAt"`
}

type SelectionContext struct {
	OldHome    *OldHomeSelectionSnapshot    `json:"oldHome"`
	TargetHome *TargetHomeSelectionSnapshot `json:"targetHome"`
}

func (s *Service) resolveSelections(
	ctx context.Context,
	userID string,
	input domaincapacity.HousingCapacityInput,
	oldSelection *OldHomeSelectionInput,
	targetSelection *TargetHomeSelectionInput,
	now time.Time,
) (domaincapacity.HousingCapacityInput, *SelectionContext, error) {
	if oldSelection == nil && targetSelection == nil {
		return input, nil, nil
	}
	contextSnapshot := &SelectionContext{}
	if oldSelection != nil {
		resolved, err := s.resolveOldHome(ctx, strings.TrimSpace(userID), &input, *oldSelection, now)
		if err != nil {
			return domaincapacity.HousingCapacityInput{}, nil, err
		}
		contextSnapshot.OldHome = resolved
	}
	if targetSelection != nil {
		resolved, err := s.resolveTargetHome(ctx, &input, *targetSelection, now)
		if err != nil {
			return domaincapacity.HousingCapacityInput{}, nil, err
		}
		contextSnapshot.TargetHome = resolved
	}
	return input, contextSnapshot, nil
}

func (s *Service) resolveOldHome(ctx context.Context, userID string, input *domaincapacity.HousingCapacityInput, selection OldHomeSelectionInput, now time.Time) (*OldHomeSelectionSnapshot, error) {
	switch selection.Mode {
	case OldHomeNone:
		if strings.TrimSpace(selection.AssetID) != "" || selection.ExpectedSalePriceWan != nil {
			return nil, ErrInvalidSelection
		}
		input.OldHomeValue = 0
		input.OldLoanBalance = 0
		if input.TransactionScenario != nil {
			input.TransactionScenario.OldHomeHoldingYears = 0
			input.TransactionScenario.OldHomeOriginalPrice = 0
			input.TransactionScenario.OldHomeOnlyFamilyHome = false
		}
		return &OldHomeSelectionSnapshot{Mode: OldHomeNone, ConfirmedAt: now.UTC()}, nil
	case OldHomeAsset:
		if userID == "" || s.assetReader == nil || strings.TrimSpace(selection.AssetID) == "" ||
			selection.ExpectedSalePriceWan == nil || !selection.PriceConfirmed || !validSelectionPrice(*selection.ExpectedSalePriceWan) ||
			math.Abs(input.OldHomeValue-*selection.ExpectedSalePriceWan) > 0.01 {
			return nil, ErrInvalidSelection
		}
		asset, err := s.assetReader.GetAsset(ctx, appasset.GetAssetQuery{UserID: userID, ID: selection.AssetID})
		if err != nil {
			if errors.Is(err, appasset.ErrAssetNotFound) {
				return nil, ErrSelectedAssetNotFound
			}
			if errors.Is(err, appasset.ErrInvalidCommand) {
				return nil, ErrInvalidSelection
			}
			return nil, err
		}
		if input.TransactionScenario == nil {
			return nil, ErrInvalidSelection
		}
		holdingYears := asset.HoldingYears(now)
		input.TransactionScenario.OldHomeHoldingYears = holdingYears
		input.TransactionScenario.OldHomeOriginalPrice = asset.OriginalPurchasePriceWan
		assetID := asset.ID
		updatedAt := asset.UpdatedAt
		property := capacityPropertySnapshot(asset.Property)
		priceDifference := selectionPriceDifference(*selection.ExpectedSalePriceWan, asset.Property.CurrentListingPriceWan)
		return &OldHomeSelectionSnapshot{
			Mode: OldHomeAsset, AssetID: &assetID, AssetName: asset.Name, Property: &property,
			OriginalPurchasePriceWan: asset.OriginalPurchasePriceWan, PurchasedOn: asset.PurchasedOn.Format(time.DateOnly),
			HoldingYears: holdingYears, ConfirmedSalePriceWan: *selection.ExpectedSalePriceWan,
			ConfirmedLoanBalanceWan: input.OldLoanBalance, PriceDifferenceWan: priceDifference,
			AssetUpdatedAt: &updatedAt, MarketReference: assetMarketReference(asset), ConfirmedAt: now.UTC(),
		}, nil
	default:
		return nil, ErrInvalidSelection
	}
}

func (s *Service) resolveTargetHome(ctx context.Context, input *domaincapacity.HousingCapacityInput, selection TargetHomeSelectionInput, now time.Time) (*TargetHomeSelectionSnapshot, error) {
	if s.targetListingReader == nil || strings.TrimSpace(selection.NeighborhoodID) == "" || strings.TrimSpace(selection.RoomID) == "" ||
		selection.ExpectedPurchasePriceWan == nil || !selection.PriceConfirmed || !validSelectionPrice(*selection.ExpectedPurchasePriceWan) ||
		math.Abs(input.TargetTotalPrice-*selection.ExpectedPurchasePriceWan) > 0.01 || input.TransactionScenario == nil {
		return nil, ErrInvalidSelection
	}
	detail, err := s.targetListingReader.GetListing(ctx, appcommunitymarket.GetListingQuery{
		NeighborhoodID: selection.NeighborhoodID, RoomID: selection.RoomID,
	})
	if err != nil {
		switch {
		case errors.Is(err, appcommunitymarket.ErrListingUnavailable):
			return nil, ErrTargetListingUnavailable
		case errors.Is(err, appcommunitymarket.ErrListingNotFound), errors.Is(err, appcommunitymarket.ErrSnapshotNotFound):
			return nil, ErrTargetListingNotFound
		case errors.Is(err, appcommunitymarket.ErrInvalidQuery):
			return nil, ErrInvalidSelection
		default:
			return nil, err
		}
	}
	if detail.Status != "active" || detail.QualityStatus != "complete" {
		return nil, ErrTargetListingUnavailable
	}
	input.TransactionScenario.City = detail.City
	input.TransactionScenario.TargetHomeType = domaincapacity.TargetHomeResale
	input.TransactionScenario.TargetHomeAreaSQM = detail.AreaSQM
	property := SelectionPropertySnapshot{
		NeighborhoodID: detail.NeighborhoodID, NeighborhoodName: detail.NeighborhoodName, City: detail.City,
		District: detail.District, Layout: detail.Layout, AreaSQM: detail.AreaSQM,
		FloorBand: detail.FloorBand, FloorDescription: detail.FloorDescription, Orientation: detail.Orientation,
		ReferenceListingPriceWan: floatPointer(detail.ListingTotalPriceWan),
	}
	return &TargetHomeSelectionSnapshot{
		Property: property, ConfirmedPurchasePriceWan: *selection.ExpectedPurchasePriceWan,
		PriceDifferenceWan: roundSelection(*selection.ExpectedPurchasePriceWan - detail.ListingTotalPriceWan),
		MarketReference:    listingMarketReference(detail), ConfirmedAt: now.UTC(),
	}, nil
}

func capacityPropertySnapshot(property domainasset.PropertySnapshot) SelectionPropertySnapshot {
	return SelectionPropertySnapshot{
		NeighborhoodID: property.NeighborhoodID, NeighborhoodName: property.NeighborhoodName,
		City: property.City, District: property.District, Layout: property.Layout, AreaSQM: property.AreaSQM,
		FloorBand: property.FloorBand, FloorDescription: property.FloorDescription, Orientation: property.Orientation,
		ReferenceListingPriceWan: floatClone(property.CurrentListingPriceWan),
	}
}

func assetMarketReference(asset domainasset.Asset) *MarketReferenceSnapshot {
	if asset.ListingSource == nil {
		return nil
	}
	return &MarketReferenceSnapshot{
		SourceListingID: asset.ListingSource.SourceListingID, DataSourceID: asset.ListingSource.DataSourceID,
		DataSourceName: asset.ListingSource.DataSourceName, DataSourceType: asset.ListingSource.DataSourceType,
		SourceRef: asset.ListingSource.SourceRef, CollectionRunID: asset.ListingSource.CollectionRunID,
		SnapshotID: asset.ListingSource.SnapshotID, CollectedAt: asset.ListingSource.CollectedAt,
		ListedAt: asset.ListingSource.ListedAt, QualityStatus: asset.ListingSource.QualityStatus,
	}
}

func listingMarketReference(detail appcommunitymarket.MarketListingDetail) MarketReferenceSnapshot {
	return MarketReferenceSnapshot{
		SourceListingID: detail.RoomID, DataSourceID: detail.Source.DataSourceID,
		DataSourceName: detail.Source.DataSourceName, DataSourceType: detail.Source.DataSourceType,
		SourceRef: detail.Source.SourceRef, CollectionRunID: detail.CollectionRunID, SnapshotID: detail.SnapshotID,
		CollectedAt: detail.CollectedAt, ListedAt: detail.ListedAt, QualityStatus: detail.QualityStatus,
		Freshness: string(detail.Freshness),
	}
}

func selectionPriceDifference(confirmed float64, reference *float64) *float64 {
	if reference == nil {
		return nil
	}
	difference := roundSelection(confirmed - *reference)
	return &difference
}

func validSelectionPrice(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value > 0
}

func roundSelection(value float64) float64 {
	return math.Round(value*100) / 100
}

func floatClone(value *float64) *float64 {
	if value == nil {
		return nil
	}
	return floatPointer(*value)
}

func floatPointer(value float64) *float64 {
	clone := value
	return &clone
}
