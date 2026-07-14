package neighborhood

import (
	"context"
	"errors"
	"time"

	domainneighborhood "github.com/sine-io/propulse/internal/domain/neighborhood"
)

var ErrNeighborhoodNotFound = errors.New("neighborhood not found")
var ErrMetricNotFound = errors.New("neighborhood metric not found")

type Repository interface {
	CreateNeighborhood(ctx context.Context, input CreateNeighborhoodInput) (Neighborhood, error)
	GetNeighborhood(ctx context.Context, id string) (Neighborhood, error)
	SearchNeighborhoods(ctx context.Context, input SearchNeighborhoodsInput) (SearchNeighborhoodsResult, error)
	AddWatchlistItem(ctx context.Context, userID string, neighborhoodID string) (WatchlistItem, error)
	ListWatchlist(ctx context.Context, userID string) ([]WatchlistSummary, error)
	LatestMetric(ctx context.Context, neighborhoodID string) (MetricSnapshot, error)
}

// SearchNeighborhoodsInput 是 repository 层的搜索/分页入参（limit/offset 已由 service 归一）。
type SearchNeighborhoodsInput struct {
	Query        string // 模糊匹配名称或区域
	Area         string // 可选：区域过滤
	TargetLayout string // 可选：目标户型过滤
	Limit        int
	Offset       int
}

// SearchNeighborhoodsResult 携带当页结果与匹配总数。
type SearchNeighborhoodsResult struct {
	Items []Neighborhood
	Total int
}

type Neighborhood struct {
	ID           string
	Name         string
	Area         string
	TargetLayout string
	CreatedAt    time.Time
}

type CreateNeighborhoodInput struct {
	ID           string
	Name         string
	Area         string
	TargetLayout string
}

type WatchlistItem struct {
	ID             string
	UserID         string
	NeighborhoodID string
	CreatedAt      time.Time
}

type WatchlistSummary struct {
	ID             string
	NeighborhoodID string
	Name           string
	Area           string
	TargetLayout   string
	HasMetric      bool
	Metric         MetricSnapshot
}

type WatchlistItemSummary struct {
	ID                  string
	NeighborhoodID      string
	Name                string
	Area                string
	TargetLayout        string
	Status              domainneighborhood.NeighborhoodStatus
	ListedHomes         int
	PriceCutHomes       int
	TransactionMomentum domainneighborhood.TransactionMomentum
	Advice              string
}

type MetricSnapshot struct {
	ID                       string
	NeighborhoodID           string
	CollectionRunID          string
	AlgorithmVersion         string
	InventoryCollectionRunID *string
	SourceIDs                []string
	LatestObservedAt         time.Time
	ListedHomes              int
	PriceCutHomes            int
	AvgDaysOnMarket          *float64
	ListingPriceMin          *float64
	ListingPriceMax          *float64
	TransactionPriceMin      *float64
	TransactionPriceMax      *float64
	TransactionMomentum      domainneighborhood.TransactionMomentum
	TransactionEvidence      *domainneighborhood.TransactionMomentumEvidence
	TargetLayoutSupply       int
	ListingSampleCount       int
	TransactionSampleCount   int
	Coverage                 domainneighborhood.Coverage
	Freshness                domainneighborhood.Freshness
	InventoryCollectedAt     *time.Time
	ListedHomesChangePct     *float64
	QualityWarnings          []domainneighborhood.QualityWarning
	QualityState             domainneighborhood.MarketQualityState
	CalculatedAt             time.Time
}

type MetricWithSignal struct {
	Metric MetricSnapshot
	Signal domainneighborhood.SignalResult
}
