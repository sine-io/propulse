package metric

import (
	"context"
	"errors"
	"time"

	domainneighborhood "github.com/sine-io/propulse/internal/domain/neighborhood"
)

var ErrNeighborhoodNotFound = errors.New("neighborhood not found")
var ErrCollectionRunNotFound = errors.New("collection_run_not_found")
var ErrCollectionRunNeighborhoodMismatch = errors.New("collection_run_neighborhood_mismatch")

type Repository interface {
	GetNeighborhood(ctx context.Context, id string) (Neighborhood, error)
	GetCompletedCollectionRun(context.Context, string) (CompletedCollectionRun, error)
	LatestCompletedCollectionRun(context.Context, string) (CompletedCollectionRun, error)
	AggregateListingSnapshots(ctx context.Context, neighborhoodID string, targetLayout string) (ListingSnapshotAggregate, error)
	InsertNeighborhoodMetric(ctx context.Context, snapshot MetricSnapshot) (MetricSnapshot, error)
	AggregateMarketObservations(context.Context, AggregateMarketParams) (MarketAggregate, error)
	UpsertNeighborhoodMetric(context.Context, MetricSnapshot) (MetricSnapshot, error)
	MarkCollectionRunMetricCompleted(context.Context, string) error
}

type Neighborhood struct {
	ID           string
	TargetLayout string
}

type ListingSnapshotAggregate struct {
	ListedHomes         int
	PriceCutHomes       int
	AvgDaysOnMarket     float64
	ListingPriceMin     float64
	ListingPriceMax     float64
	TransactionPriceMin float64
	TransactionPriceMax float64
	TargetLayoutSupply  int
}

type MetricSnapshot struct {
	ID                       string
	NeighborhoodID           string
	CollectionRunID          string
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

type CalculateCollectionRunCommand struct {
	NeighborhoodID  string
	CollectionRunID string
}

type AggregateMarketParams struct {
	NeighborhoodID string
	TriggerRunID   string
	TargetLayout   string
}

type CompletedCollectionRun struct {
	ID             string
	DataSourceID   string
	NeighborhoodID string
	CollectedAt    time.Time
	Coverage       domainneighborhood.Coverage
}

type MarketAggregate struct {
	CollectionRunID                   string
	InventoryCollectionRunID          *string
	SourceIDs                         []string
	LatestObservedAt                  time.Time
	InventoryCollectedAt              *time.Time
	Coverage                          domainneighborhood.Coverage
	ListedHomes                       int
	PriceCutHomes                     int
	AvgDaysOnMarket                   *float64
	ListingPriceMin                   *float64
	ListingPriceMax                   *float64
	TransactionPriceMin               *float64
	TransactionPriceMax               *float64
	TargetLayoutSupply                int
	ListingSampleCount                int
	TransactionSampleCount            int
	LastThirtyDayTransactionCount     int
	PrecedingSixtyDayTransactionCount int
	ListedHomesChangePct              *float64
}
