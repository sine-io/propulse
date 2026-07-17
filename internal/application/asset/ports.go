package asset

import (
	"context"
	"errors"

	appcommunitymarket "github.com/sine-io/propulse/internal/application/communitymarket"
	appneighborhood "github.com/sine-io/propulse/internal/application/neighborhood"
	domainasset "github.com/sine-io/propulse/internal/domain/asset"
)

var (
	ErrAssetNotFound        = errors.New("property asset not found")
	ErrInvalidCommand       = errors.New("invalid property asset command")
	ErrNeighborhoodNotFound = errors.New("asset neighborhood not found")
	ErrListingNotFound      = errors.New("asset source listing not found")
	ErrListingUnavailable   = errors.New("asset source listing unavailable")
)

type Repository interface {
	Create(context.Context, domainasset.Asset) (domainasset.Asset, error)
	Update(context.Context, domainasset.Asset) (domainasset.Asset, error)
	Find(context.Context, string, string) (domainasset.Asset, error)
	List(context.Context, string, int, int) ([]domainasset.Asset, int, error)
	SoftDelete(context.Context, string, string) error
}

type NeighborhoodReader interface {
	GetNeighborhood(context.Context, appneighborhood.GetNeighborhoodQuery) (appneighborhood.Neighborhood, error)
}

type MarketListingReader interface {
	GetListing(context.Context, appcommunitymarket.GetListingQuery) (appcommunitymarket.MarketListingDetail, error)
}
