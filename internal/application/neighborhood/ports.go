package neighborhood

import (
	"context"
	"errors"
	"time"

	domainneighborhood "github.com/sine-io/propulse/internal/domain/neighborhood"
)

var ErrNeighborhoodNotFound = errors.New("neighborhood not found")
var ErrMetricNotFound = errors.New("neighborhood metric not found")
var ErrInvalidNeighborhood = errors.New("invalid neighborhood")
var ErrInvalidNeighborhoodID = errors.New("invalid neighborhood id")
var ErrInvalidTargetLayout = errors.New("invalid target layout")
var ErrWatchlistItemExists = errors.New("watchlist item exists")

type Repository interface {
	CreateNeighborhood(ctx context.Context, input CreateNeighborhoodInput) (Neighborhood, error)
	GetNeighborhood(ctx context.Context, id string) (Neighborhood, error)
	SearchNeighborhoods(ctx context.Context, input SearchNeighborhoodsInput) (SearchNeighborhoodsResult, error)
	AddWatchlistItem(ctx context.Context, userID string, neighborhoodID string, targetLayout string) (WatchlistItem, error)
	ListWatchlist(ctx context.Context, userID string) ([]WatchlistSummary, error)
	LatestMetric(ctx context.Context, neighborhoodID string) (MetricSnapshot, error)
	ListMetricHistory(ctx context.Context, query MetricHistoryRepositoryQuery) ([]MetricHistoryRecord, error)
}

// SearchNeighborhoodsInput 是 repository 层的搜索/分页入参（limit/offset 已由 service 归一）。
type SearchNeighborhoodsInput struct {
	Query        string // 模糊匹配名称或区域
	City         string
	Area         string // 可选：区域过滤
	TargetLayout string // 可选：目标户型过滤
	Limit        int
	Offset       int
}

// SearchNeighborhoodsResult 携带当页结果与匹配总数。
type SearchNeighborhoodsResult struct {
	Items   []Neighborhood
	Total   int
	Filters NeighborhoodSearchFilters
}

type Neighborhood struct {
	ID               string
	Name             string
	City             *string
	Area             string
	AvailableLayouts []string
	CreatedAt        time.Time
}

type CreateNeighborhoodInput struct {
	ID               string
	Name             string
	City             string
	Area             string
	AvailableLayouts []string
}

type NeighborhoodAreaFilter struct {
	City string
	Area string
}

type NeighborhoodSearchFilters struct {
	Cities []string
	Areas  []NeighborhoodAreaFilter
}

type WatchlistItem struct {
	ID             string
	UserID         string
	NeighborhoodID string
	TargetLayout   string
	CreatedAt      time.Time
}

type WatchlistSummary struct {
	ID             string
	NeighborhoodID string
	Name           string
	City           *string
	Area           string
	TargetLayout   string
	HasMetric      bool
	Metric         MetricSnapshot
}

type WatchlistItemSummary struct {
	ID                     string
	NeighborhoodID         string
	Name                   string
	City                   *string
	Area                   string
	TargetLayout           string
	Status                 domainneighborhood.NeighborhoodStatus
	ListedHomes            int
	PriceCutHomes          int
	TransactionMomentum    domainneighborhood.TransactionMomentum
	TargetLayoutSupply     int
	TargetLayoutScarcity   domainneighborhood.Scarcity
	Advice                 string
	HasMetric              bool
	CollectionRunID        string
	AlgorithmVersion       string
	SourceIDs              []string
	CollectedAt            *time.Time
	TransactionSampleCount int
	Coverage               domainneighborhood.Coverage
	Freshness              domainneighborhood.Freshness
	QualityState           domainneighborhood.MarketQualityState
	QualityWarnings        []domainneighborhood.QualityWarning
	WeeklyComparison       *MetricComparison
}

type MetricSnapshot struct {
	ID                         string
	NeighborhoodID             string
	CollectionRunID            string
	AlgorithmVersion           string
	InventoryCollectionRunID   *string
	SourceIDs                  []string
	LatestObservedAt           time.Time
	CollectedAt                time.Time
	ListedHomes                int
	PriceCutHomes              int
	AvgDaysOnMarket            *float64
	ListingPriceMin            *float64
	ListingPriceMax            *float64
	TransactionPriceMin        *float64
	TransactionPriceMax        *float64
	TransactionMomentum        domainneighborhood.TransactionMomentum
	TransactionEvidence        *domainneighborhood.TransactionMomentumEvidence
	TargetLayout               string
	TargetLayoutSupply         int
	TargetLayoutSupplyByLayout map[string]int
	ListingSampleCount         int
	TransactionSampleCount     int
	Coverage                   domainneighborhood.Coverage
	Freshness                  domainneighborhood.Freshness
	InventoryCollectedAt       *time.Time
	ListedHomesChangePct       *float64
	QualityWarnings            []domainneighborhood.QualityWarning
	QualityState               domainneighborhood.MarketQualityState
	CalculatedAt               time.Time
}

type MetricHistoryRepositoryQuery struct {
	NeighborhoodID string
	From           time.Time
	To             time.Time
}

type CollectionRunReference struct {
	CollectionRunID string
	DataSourceID    string
	SourceRef       string
	CollectedAt     time.Time
	Coverage        domainneighborhood.Coverage
}

type MetricHistoryRecord struct {
	Metric MetricSnapshot
	Batch  CollectionRunReference
}

type MetricComparison struct {
	Status                      domainneighborhood.MetricComparisonStatus
	Reason                      domainneighborhood.MetricComparisonReason
	CurrentBatch                CollectionRunReference
	BaselineBatch               *CollectionRunReference
	ListedHomes                 *domainneighborhood.MetricChangeValue
	PriceCutHomes               *domainneighborhood.MetricChangeValue
	RecentThirtyDayTransactions *domainneighborhood.MetricChangeValue
}

type MetricHistoryPoint struct {
	Metric            MetricSnapshot
	Batch             CollectionRunReference
	WeeklyComparison  MetricComparison
	MonthlyComparison MetricComparison
}

type MetricHistoryStatus string

const (
	MetricHistoryReady MetricHistoryStatus = "ready"
	MetricHistoryEmpty MetricHistoryStatus = "empty"
)

type MetricHistoryResult struct {
	Status           MetricHistoryStatus
	NeighborhoodID   string
	TargetLayout     string
	AlgorithmVersion string
	From             time.Time
	To               time.Time
	Items            []MetricHistoryPoint
}

type MetricWithSignal struct {
	Metric MetricSnapshot
	Signal domainneighborhood.SignalResult
}
