package communitymarket

import (
	"encoding/json"
	"errors"
	"time"

	domaincommunitymarket "github.com/sine-io/propulse/internal/domain/communitymarket"
	domainneighborhood "github.com/sine-io/propulse/internal/domain/neighborhood"
)

const MaxRawPayloadBytes = 2 * 1024 * 1024

var (
	ErrDataSourceNotFound   = errors.New("data_source_not_found")
	ErrNeighborhoodNotFound = errors.New("neighborhood_not_found")
	ErrSnapshotNotFound     = errors.New("community_market_snapshot_not_found")
	ErrListingNotFound      = errors.New("market_listing_not_found")
	ErrListingUnavailable   = errors.New("market_listing_unavailable")
	ErrImportFailed         = errors.New("community_market_import_failed")
	ErrInvalidQuery         = errors.New("invalid_community_market_query")
)

const FangjianBundleSchemaVersion = "fangjian.bundle/v1"

type ValidationIssue struct {
	Row     *int   `json:"row,omitempty"`
	Field   string `json:"field"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

type ValidationError struct {
	Issues []ValidationIssue
}

func (e *ValidationError) Error() string {
	return "one or more community market fields are invalid"
}

type ImportSnapshotCommand struct {
	DataSourceID   string
	NeighborhoodID string
	SourceRef      string
	CollectedAt    time.Time
	RawPayload     []byte
	RawContentType string
	Data           domaincommunitymarket.SnapshotData
}

type Snapshot struct {
	ID              string
	DataSourceID    string
	NeighborhoodID  string
	SourceRef       string
	CollectedAt     time.Time
	ContentChecksum string
	RawPayload      []byte
	RawContentType  string
	CollectionRunID *string
	QualityStatus   string
	Data            domaincommunitymarket.SnapshotData
	CreatedAt       time.Time
}

type SaveSnapshotResult struct {
	Snapshot Snapshot
	Created  bool
}

type ImportSnapshotResult struct {
	Snapshot         Snapshot
	IdempotentReplay bool
}

type LatestSnapshotQuery struct {
	NeighborhoodID string
}

type MarketListing struct {
	RoomID               string    `json:"roomId"`
	Layout               string    `json:"layout"`
	AreaSQM              float64   `json:"areaSqm"`
	ListingTotalPriceWan float64   `json:"listingTotalPriceWan"`
	ListingUnitPrice     float64   `json:"listingUnitPrice"`
	ListedAt             time.Time `json:"listedAt"`
	DaysOnMarket         int       `json:"daysOnMarket"`
	FloorBand            string    `json:"floorBand"`
	FloorDescription     string    `json:"floorDescription"`
	Orientation          string    `json:"orientation"`
	AdjustmentCount      int       `json:"adjustmentCount"`
	FollowCount          int       `json:"followCount"`
	LookCount30Days      int       `json:"lookCount30Days"`
}

type MarketSource struct {
	DataSourceID   string `json:"dataSourceId"`
	DataSourceName string `json:"dataSourceName"`
	DataSourceType string `json:"dataSourceType"`
	SourceRef      string `json:"sourceRef"`
}

type MarketListingDetail struct {
	MarketListing
	NeighborhoodID   string                       `json:"neighborhoodId"`
	NeighborhoodName string                       `json:"neighborhoodName"`
	City             string                       `json:"city"`
	District         string                       `json:"district"`
	Status           string                       `json:"status"`
	SnapshotID       string                       `json:"snapshotId"`
	CollectionRunID  string                       `json:"collectionRunId"`
	CollectedAt      time.Time                    `json:"collectedAt"`
	Source           MarketSource                 `json:"source"`
	QualityStatus    string                       `json:"qualityStatus"`
	Freshness        domainneighborhood.Freshness `json:"freshness"`
}

type MarketTransaction struct {
	RoomID               string    `json:"roomId"`
	Layout               string    `json:"layout"`
	AreaSQM              float64   `json:"areaSqm"`
	ListingTotalPriceWan float64   `json:"listingTotalPriceWan"`
	TradeTotalPriceWan   float64   `json:"tradeTotalPriceWan"`
	TradeUnitPrice       float64   `json:"tradeUnitPrice"`
	TradeDate            time.Time `json:"tradeDate"`
	NegotiationWan       float64   `json:"negotiationWan"`
	NegotiationPercent   float64   `json:"negotiationPercent"`
	FloorBand            string    `json:"floorBand"`
	FloorDescription     string    `json:"floorDescription"`
	Orientation          string    `json:"orientation"`
	AdjustmentCount      int       `json:"adjustmentCount"`
}

type ListingAdjustment struct {
	ID             string    `json:"id,omitempty"`
	RoomID         string    `json:"roomId"`
	AdjustedAt     time.Time `json:"adjustedAt"`
	PriceBeforeWan float64   `json:"priceBeforeWan"`
	PriceAfterWan  float64   `json:"priceAfterWan"`
	AmountWan      float64   `json:"amountWan"`
}

type BundleQuality struct {
	Status   string   `json:"status"`
	Warnings []string `json:"warnings"`
}

type FangjianBundle struct {
	SchemaVersion string                             `json:"schemaVersion"`
	CollectedAt   time.Time                          `json:"collectedAt"`
	Community     domaincommunitymarket.SnapshotData `json:"community"`
	Listings      []MarketListing                    `json:"listings"`
	Transactions  []MarketTransaction                `json:"transactions"`
	Adjustments   []ListingAdjustment                `json:"adjustments"`
	Quality       BundleQuality                      `json:"quality"`
}

type ImportFangjianCommand struct {
	DataSourceID   string
	NeighborhoodID string
	SourceRef      string
	RawPayload     []byte
	Bundle         FangjianBundle
}

type ImportFangjianResult struct {
	Snapshot         Snapshot
	CollectionRunID  string
	ListingCount     int
	TransactionCount int
	AdjustmentCount  int
	IdempotentReplay bool
}

type FangjianImportBatch struct {
	Snapshot        Snapshot
	CollectionRunID string
	Listings        []MarketListing
	Transactions    []MarketTransaction
	Adjustments     []ListingAdjustment
}

type SaveFangjianResult struct {
	Snapshot Snapshot
	Created  bool
}

type Page[T any] struct {
	Items    []T `json:"items"`
	Total    int `json:"total"`
	Page     int `json:"page"`
	PageSize int `json:"pageSize"`
}

type MarketListQuery struct {
	NeighborhoodID string
	Layout         string
	Floor          string
	MinPriceWan    *float64
	MaxPriceWan    *float64
	SortBy         string
	SortOrder      string
	Page           int
	PageSize       int
}

type ListingAdjustmentsQuery struct {
	NeighborhoodID string
	RoomID         string
}

type GetListingQuery struct {
	NeighborhoodID string
	RoomID         string
}

type ComparisonQuery struct {
	NeighborhoodID     string
	PeerNeighborhoodID string
}

type ComparisonMetric struct {
	Primary *float64 `json:"primary"`
	Peer    *float64 `json:"peer"`
	Delta   *float64 `json:"delta"`
}

type Comparison struct {
	Primary           Snapshot         `json:"primary"`
	Peer              Snapshot         `json:"peer"`
	ListingUnitPrice  ComparisonMetric `json:"listingUnitPrice"`
	Supply            ComparisonMetric `json:"supply"`
	RecentTrades      ComparisonMetric `json:"recentTrades"`
	ListingTradeGap   ComparisonMetric `json:"listingTradeGap"`
	AverageTradeCycle ComparisonMetric `json:"averageTradeCycle"`
}

type ArchiveManifest struct {
	SchemaVersion string            `json:"schemaVersion"`
	CollectedAt   time.Time         `json:"collectedAt"`
	CommunityID   string            `json:"communityId"`
	CommunityName string            `json:"communityName"`
	Files         map[string]string `json:"files"`
	Endpoints     []string          `json:"endpoints"`
}

type CollectedCommunity struct {
	Slug      string
	Bundle    FangjianBundle
	Raw       map[string]json.RawMessage
	Endpoints []string
}
