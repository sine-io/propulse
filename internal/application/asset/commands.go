package asset

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	appcommunitymarket "github.com/sine-io/propulse/internal/application/communitymarket"
	appneighborhood "github.com/sine-io/propulse/internal/application/neighborhood"
	domainasset "github.com/sine-io/propulse/internal/domain/asset"
)

type Service struct {
	repo          Repository
	neighborhoods NeighborhoodReader
	listings      MarketListingReader
	now           func() time.Time
	newID         func() string
}

func NewService(repo Repository, neighborhoods NeighborhoodReader, listings MarketListingReader, now func() time.Time, newID func() string) *Service {
	if now == nil {
		now = time.Now
	}
	if newID == nil {
		newID = uuid.NewString
	}
	return &Service{repo: repo, neighborhoods: neighborhoods, listings: listings, now: now, newID: newID}
}

type CreateAssetCommand struct {
	UserID                   string
	Name                     string
	NeighborhoodID           string
	PropertySelection        PropertySelectionInput
	OriginalPurchasePriceWan float64
	PurchasedOn              string
	CurrentLoanBalanceWan    float64
}

func (s *Service) CreateAsset(ctx context.Context, command CreateAssetCommand) (domainasset.Asset, error) {
	purchasedOn, err := parsePurchaseDate(command.PurchasedOn)
	if err != nil {
		return domainasset.Asset{}, ErrInvalidCommand
	}
	property, sourceKind, listingSource, err := s.resolveProperty(ctx, command.NeighborhoodID, command.PropertySelection)
	if err != nil {
		return domainasset.Asset{}, err
	}
	name := strings.TrimSpace(command.Name)
	if name == "" {
		name = strings.TrimSpace(property.NeighborhoodName + " " + property.Layout)
	}
	created, err := domainasset.New(domainasset.Draft{
		ID: s.newID(), UserID: command.UserID, Name: name, Property: property,
		OriginalPurchasePriceWan: command.OriginalPurchasePriceWan, PurchasedOn: purchasedOn,
		CurrentLoanBalanceWan: command.CurrentLoanBalanceWan, SourceKind: sourceKind, ListingSource: listingSource,
	}, s.now())
	if err != nil {
		return domainasset.Asset{}, ErrInvalidCommand
	}
	return s.repo.Create(ctx, created)
}

type UpdateAssetCommand struct {
	UserID                   string
	ID                       string
	Name                     *string
	PropertySelection        *PropertySelectionInput
	OriginalPurchasePriceWan *float64
	PurchasedOn              *string
	CurrentLoanBalanceWan    *float64
}

func (s *Service) UpdateAsset(ctx context.Context, command UpdateAssetCommand) (domainasset.Asset, error) {
	if command.Name == nil && command.PropertySelection == nil && command.OriginalPurchasePriceWan == nil &&
		command.PurchasedOn == nil && command.CurrentLoanBalanceWan == nil {
		return domainasset.Asset{}, ErrInvalidCommand
	}
	current, err := s.GetAsset(ctx, GetAssetQuery{UserID: command.UserID, ID: command.ID})
	if err != nil {
		return domainasset.Asset{}, err
	}

	name := current.Name
	if command.Name != nil {
		name = *command.Name
	}
	property := current.Property
	sourceKind := current.SourceKind
	listingSource := current.ListingSource
	if command.PropertySelection != nil {
		property, sourceKind, listingSource, err = s.resolveProperty(ctx, current.Property.NeighborhoodID, *command.PropertySelection)
		if err != nil {
			return domainasset.Asset{}, err
		}
	}
	originalPrice := current.OriginalPurchasePriceWan
	if command.OriginalPurchasePriceWan != nil {
		originalPrice = *command.OriginalPurchasePriceWan
	}
	purchasedOn := current.PurchasedOn
	if command.PurchasedOn != nil {
		purchasedOn, err = parsePurchaseDate(*command.PurchasedOn)
		if err != nil {
			return domainasset.Asset{}, ErrInvalidCommand
		}
	}
	loanBalance := current.CurrentLoanBalanceWan
	if command.CurrentLoanBalanceWan != nil {
		loanBalance = *command.CurrentLoanBalanceWan
	}

	updated, err := domainasset.New(domainasset.Draft{
		ID: current.ID, UserID: current.UserID, Name: name, Property: property,
		OriginalPurchasePriceWan: originalPrice, PurchasedOn: purchasedOn,
		CurrentLoanBalanceWan: loanBalance, SourceKind: sourceKind, ListingSource: listingSource,
	}, s.now())
	if err != nil {
		return domainasset.Asset{}, ErrInvalidCommand
	}
	updated.CreatedAt = current.CreatedAt
	if err := updated.ValidateAt(s.now()); err != nil {
		return domainasset.Asset{}, ErrInvalidCommand
	}
	return s.repo.Update(ctx, updated)
}

type DeleteAssetCommand struct {
	UserID string
	ID     string
}

func (s *Service) DeleteAsset(ctx context.Context, command DeleteAssetCommand) error {
	userID := strings.TrimSpace(command.UserID)
	id, err := uuid.Parse(strings.TrimSpace(command.ID))
	if err != nil || userID == "" {
		return ErrInvalidCommand
	}
	if err := s.repo.SoftDelete(ctx, userID, id.String()); err != nil {
		return err
	}
	return nil
}

func (s *Service) resolveProperty(ctx context.Context, neighborhoodID string, selection PropertySelectionInput) (domainasset.PropertySnapshot, domainasset.SourceKind, *domainasset.ListingSourceSnapshot, error) {
	parsedID, err := uuid.Parse(strings.TrimSpace(neighborhoodID))
	if err != nil || s.neighborhoods == nil {
		return domainasset.PropertySnapshot{}, "", nil, ErrInvalidCommand
	}
	neighborhood, err := s.neighborhoods.GetNeighborhood(ctx, appneighborhood.GetNeighborhoodQuery{ID: parsedID.String()})
	if err != nil {
		if errors.Is(err, appneighborhood.ErrNeighborhoodNotFound) {
			return domainasset.PropertySnapshot{}, "", nil, ErrNeighborhoodNotFound
		}
		return domainasset.PropertySnapshot{}, "", nil, err
	}
	city := ""
	if neighborhood.City != nil {
		city = *neighborhood.City
	}
	switch selection.Mode {
	case PropertySelectionManual:
		return domainasset.PropertySnapshot{
			NeighborhoodID: neighborhood.ID, NeighborhoodName: neighborhood.Name, City: city, District: neighborhood.Area,
			Layout: selection.Layout, AreaSQM: selection.AreaSQM, FloorBand: selection.FloorBand,
			FloorDescription: selection.FloorDescription, Orientation: selection.Orientation,
			CurrentListingPriceWan: cloneFloat(selection.CurrentListingPriceWan),
		}, domainasset.SourceManual, nil, nil
	case PropertySelectionMarketListing:
		if s.listings == nil {
			return domainasset.PropertySnapshot{}, "", nil, ErrInvalidCommand
		}
		detail, listingErr := s.listings.GetListing(ctx, appcommunitymarket.GetListingQuery{NeighborhoodID: neighborhood.ID, RoomID: selection.RoomID})
		if listingErr != nil {
			switch {
			case errors.Is(listingErr, appcommunitymarket.ErrListingUnavailable):
				return domainasset.PropertySnapshot{}, "", nil, ErrListingUnavailable
			case errors.Is(listingErr, appcommunitymarket.ErrListingNotFound), errors.Is(listingErr, appcommunitymarket.ErrSnapshotNotFound):
				return domainasset.PropertySnapshot{}, "", nil, ErrListingNotFound
			default:
				return domainasset.PropertySnapshot{}, "", nil, listingErr
			}
		}
		price := detail.ListingTotalPriceWan
		return domainasset.PropertySnapshot{
				NeighborhoodID: detail.NeighborhoodID, NeighborhoodName: detail.NeighborhoodName, City: detail.City, District: detail.District,
				Layout: detail.Layout, AreaSQM: detail.AreaSQM, FloorBand: detail.FloorBand,
				FloorDescription: detail.FloorDescription, Orientation: detail.Orientation, CurrentListingPriceWan: &price,
			}, domainasset.SourceMarketListing, &domainasset.ListingSourceSnapshot{
				SourceListingID: detail.RoomID, DataSourceID: detail.Source.DataSourceID,
				DataSourceName: detail.Source.DataSourceName, DataSourceType: detail.Source.DataSourceType,
				SourceRef: detail.Source.SourceRef, CollectionRunID: detail.CollectionRunID, SnapshotID: detail.SnapshotID,
				CollectedAt: detail.CollectedAt, ListedAt: detail.ListedAt, QualityStatus: detail.QualityStatus,
			}, nil
	default:
		return domainasset.PropertySnapshot{}, "", nil, ErrInvalidCommand
	}
}

func parsePurchaseDate(value string) (time.Time, error) {
	parsed, err := time.Parse(time.DateOnly, strings.TrimSpace(value))
	if err != nil {
		return time.Time{}, err
	}
	return parsed.UTC(), nil
}

func cloneFloat(value *float64) *float64 {
	if value == nil {
		return nil
	}
	clone := *value
	return &clone
}
