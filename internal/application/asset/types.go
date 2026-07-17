package asset

import domainasset "github.com/sine-io/propulse/internal/domain/asset"

type PropertySelectionMode string

const (
	PropertySelectionMarketListing PropertySelectionMode = "market_listing"
	PropertySelectionManual        PropertySelectionMode = "manual"
)

type PropertySelectionInput struct {
	Mode                   PropertySelectionMode
	RoomID                 string
	Layout                 string
	AreaSQM                float64
	FloorBand              string
	FloorDescription       string
	Orientation            string
	CurrentListingPriceWan *float64
}

type Page struct {
	Items    []domainasset.Asset
	Total    int
	Page     int
	PageSize int
}
