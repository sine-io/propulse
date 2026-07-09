package metric

import (
	"context"
	"errors"
	"time"

	domainneighborhood "github.com/sine-io/propulse/backend/internal/domain/neighborhood"
)

var ErrNeighborhoodNotFound = errors.New("neighborhood not found")

type Repository interface {
	GetNeighborhood(ctx context.Context, id string) (Neighborhood, error)
	AggregateListingSnapshots(ctx context.Context, neighborhoodID string, targetLayout string) (ListingSnapshotAggregate, error)
	InsertNeighborhoodMetric(ctx context.Context, snapshot MetricSnapshot) (MetricSnapshot, error)
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
	ID                  string
	NeighborhoodID      string
	ListedHomes         int
	PriceCutHomes       int
	AvgDaysOnMarket     float64
	ListingPriceMin     float64
	ListingPriceMax     float64
	TransactionPriceMin float64
	TransactionPriceMax float64
	TransactionMomentum domainneighborhood.TransactionMomentum
	TargetLayoutSupply  int
	CalculatedAt        time.Time
}
