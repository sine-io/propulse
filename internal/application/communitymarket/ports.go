package communitymarket

import "context"

import "encoding/json"

type Repository interface {
	DataSourceExists(context.Context, string) (bool, error)
	NeighborhoodExists(context.Context, string) (bool, error)
	SaveSnapshot(context.Context, Snapshot) (SaveSnapshotResult, error)
	SaveFangjian(context.Context, FangjianImportBatch) (SaveFangjianResult, error)
	LatestSnapshot(context.Context, string) (Snapshot, error)
	LatestListings(context.Context, string) ([]MarketListing, error)
	LatestListing(context.Context, string, string) (MarketListingDetail, error)
	LatestTransactions(context.Context, string) ([]MarketTransaction, error)
	LatestAdjustments(context.Context, string, string) ([]ListingAdjustment, error)
}

type FangjianClient interface {
	Get(context.Context, string) (json.RawMessage, error)
	Post(context.Context, string, any) (json.RawMessage, error)
}

type FangjianArchive interface {
	Write(context.Context, CollectedCommunity) (string, error)
}
