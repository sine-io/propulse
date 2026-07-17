package asset

import (
	"errors"
	"math"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

var ErrInvalidAsset = errors.New("invalid property asset")

type SourceKind string

const (
	SourceManual        SourceKind = "manual"
	SourceMarketListing SourceKind = "market_listing"
)

type PropertySnapshot struct {
	NeighborhoodID         string
	NeighborhoodName       string
	City                   string
	District               string
	Layout                 string
	AreaSQM                float64
	FloorBand              string
	FloorDescription       string
	Orientation            string
	CurrentListingPriceWan *float64
}

type ListingSourceSnapshot struct {
	SourceListingID string
	DataSourceID    string
	DataSourceName  string
	DataSourceType  string
	SourceRef       string
	CollectionRunID string
	SnapshotID      string
	CollectedAt     time.Time
	ListedAt        time.Time
	QualityStatus   string
}

type Asset struct {
	ID                       string
	UserID                   string
	Name                     string
	Property                 PropertySnapshot
	OriginalPurchasePriceWan float64
	PurchasedOn              time.Time
	CurrentLoanBalanceWan    float64
	SourceKind               SourceKind
	ListingSource            *ListingSourceSnapshot
	CreatedAt                time.Time
	UpdatedAt                time.Time
	DeletedAt                *time.Time
}

type Draft struct {
	ID                       string
	UserID                   string
	Name                     string
	Property                 PropertySnapshot
	OriginalPurchasePriceWan float64
	PurchasedOn              time.Time
	CurrentLoanBalanceWan    float64
	SourceKind               SourceKind
	ListingSource            *ListingSourceSnapshot
}

func New(draft Draft, now time.Time) (Asset, error) {
	draft = normalizeDraft(draft)
	asset := Asset{
		ID:                       draft.ID,
		UserID:                   draft.UserID,
		Name:                     draft.Name,
		Property:                 draft.Property,
		OriginalPurchasePriceWan: draft.OriginalPurchasePriceWan,
		PurchasedOn:              dateOnly(draft.PurchasedOn),
		CurrentLoanBalanceWan:    draft.CurrentLoanBalanceWan,
		SourceKind:               draft.SourceKind,
		ListingSource:            cloneListingSource(draft.ListingSource),
		CreatedAt:                now.UTC(),
		UpdatedAt:                now.UTC(),
	}
	if err := asset.ValidateAt(now); err != nil {
		return Asset{}, err
	}
	return asset, nil
}

func (asset Asset) ValidateAt(asOf time.Time) error {
	if _, err := uuid.Parse(asset.ID); err != nil {
		return ErrInvalidAsset
	}
	if strings.TrimSpace(asset.UserID) == "" || utf8.RuneCountInString(asset.UserID) > 256 ||
		strings.TrimSpace(asset.Name) == "" || utf8.RuneCountInString(asset.Name) > 128 {
		return ErrInvalidAsset
	}
	if err := asset.Property.validate(); err != nil {
		return err
	}
	if !finitePositive(asset.OriginalPurchasePriceWan) ||
		!finiteNonNegative(asset.CurrentLoanBalanceWan) ||
		asset.PurchasedOn.IsZero() || asset.PurchasedOn.Before(time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC)) ||
		asset.PurchasedOn.After(dateOnly(asOf)) {
		return ErrInvalidAsset
	}
	if asset.CreatedAt.IsZero() || asset.UpdatedAt.IsZero() || asset.UpdatedAt.Before(asset.CreatedAt) {
		return ErrInvalidAsset
	}
	switch asset.SourceKind {
	case SourceManual:
		if asset.ListingSource != nil {
			return ErrInvalidAsset
		}
	case SourceMarketListing:
		if asset.ListingSource == nil || asset.Property.CurrentListingPriceWan == nil ||
			asset.ListingSource.validate(asOf) != nil {
			return ErrInvalidAsset
		}
	default:
		return ErrInvalidAsset
	}
	return nil
}

func (asset Asset) HoldingYears(asOf time.Time) int {
	if asset.PurchasedOn.IsZero() || asOf.Before(asset.PurchasedOn) {
		return 0
	}
	years := asOf.Year() - asset.PurchasedOn.Year()
	anniversary := time.Date(asOf.Year(), asset.PurchasedOn.Month(), asset.PurchasedOn.Day(), 0, 0, 0, 0, asOf.Location())
	if asOf.Before(anniversary) {
		years--
	}
	if years < 0 {
		return 0
	}
	return years
}

func (property PropertySnapshot) validate() error {
	if _, err := uuid.Parse(property.NeighborhoodID); err != nil {
		return ErrInvalidAsset
	}
	for _, field := range []struct {
		value    string
		max      int
		required bool
	}{
		{property.NeighborhoodName, 256, true},
		{property.City, 128, true},
		{property.District, 128, true},
		{property.Layout, 64, true},
		{property.FloorBand, 64, false},
		{property.FloorDescription, 128, false},
		{property.Orientation, 128, false},
	} {
		length := utf8.RuneCountInString(strings.TrimSpace(field.value))
		if (field.required && length == 0) || length > field.max {
			return ErrInvalidAsset
		}
	}
	if !finitePositive(property.AreaSQM) || property.AreaSQM > 10000 {
		return ErrInvalidAsset
	}
	if property.CurrentListingPriceWan != nil && !finitePositive(*property.CurrentListingPriceWan) {
		return ErrInvalidAsset
	}
	return nil
}

func (source ListingSourceSnapshot) validate(asOf time.Time) error {
	for _, field := range []struct {
		value string
		max   int
	}{
		{source.SourceListingID, 128},
		{source.DataSourceName, 128},
		{source.DataSourceType, 64},
		{source.SourceRef, 256},
		{source.QualityStatus, 32},
	} {
		length := utf8.RuneCountInString(strings.TrimSpace(field.value))
		if length == 0 || length > field.max {
			return ErrInvalidAsset
		}
	}
	for _, value := range []string{source.DataSourceID, source.CollectionRunID, source.SnapshotID} {
		if _, err := uuid.Parse(value); err != nil {
			return ErrInvalidAsset
		}
	}
	if source.CollectedAt.IsZero() || source.CollectedAt.After(asOf.Add(5*time.Minute)) ||
		source.ListedAt.IsZero() || source.ListedAt.After(source.CollectedAt) || source.QualityStatus != "complete" {
		return ErrInvalidAsset
	}
	return nil
}

func normalizeDraft(draft Draft) Draft {
	draft.ID = strings.TrimSpace(draft.ID)
	draft.UserID = strings.TrimSpace(draft.UserID)
	draft.Name = strings.TrimSpace(draft.Name)
	draft.Property.NeighborhoodID = strings.TrimSpace(draft.Property.NeighborhoodID)
	draft.Property.NeighborhoodName = strings.TrimSpace(draft.Property.NeighborhoodName)
	draft.Property.City = strings.TrimSpace(draft.Property.City)
	draft.Property.District = strings.TrimSpace(draft.Property.District)
	draft.Property.Layout = strings.TrimSpace(draft.Property.Layout)
	draft.Property.FloorBand = strings.TrimSpace(draft.Property.FloorBand)
	draft.Property.FloorDescription = strings.TrimSpace(draft.Property.FloorDescription)
	draft.Property.Orientation = strings.TrimSpace(draft.Property.Orientation)
	if draft.ListingSource != nil {
		source := *draft.ListingSource
		source.SourceListingID = strings.TrimSpace(source.SourceListingID)
		source.DataSourceID = strings.TrimSpace(source.DataSourceID)
		source.DataSourceName = strings.TrimSpace(source.DataSourceName)
		source.DataSourceType = strings.TrimSpace(source.DataSourceType)
		source.SourceRef = strings.TrimSpace(source.SourceRef)
		source.CollectionRunID = strings.TrimSpace(source.CollectionRunID)
		source.SnapshotID = strings.TrimSpace(source.SnapshotID)
		source.QualityStatus = strings.TrimSpace(source.QualityStatus)
		draft.ListingSource = &source
	}
	return draft
}

func cloneListingSource(source *ListingSourceSnapshot) *ListingSourceSnapshot {
	if source == nil {
		return nil
	}
	clone := *source
	return &clone
}

func dateOnly(value time.Time) time.Time {
	if value.IsZero() {
		return time.Time{}
	}
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, time.UTC)
}

func finitePositive(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value > 0
}

func finiteNonNegative(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= 0
}
