package neighborhood

import (
	"context"
	"errors"
	"time"

	domainneighborhood "github.com/propulse/propulse/backend/internal/domain/neighborhood"
)

var ErrNeighborhoodNotFound = errors.New("neighborhood not found")
var ErrMetricNotFound = errors.New("neighborhood metric not found")

type Repository interface {
	CreateNeighborhood(ctx context.Context, input CreateNeighborhoodInput) (Neighborhood, error)
	GetNeighborhood(ctx context.Context, id string) (Neighborhood, error)
	AddWatchlistItem(ctx context.Context, userID string, neighborhoodID string) (WatchlistItem, error)
	ListWatchlist(ctx context.Context, userID string) ([]WatchlistSummary, error)
	ListWatchlistNeighborhoodIDs(ctx context.Context) ([]string, error)
	LatestMetric(ctx context.Context, neighborhoodID string) (MetricSnapshot, error)
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
	ID                   string
	NeighborhoodID       string
	ListedHomes          int
	ListedHomesChangePct float64
	PriceCutHomes        int
	AvgDaysOnMarket      float64
	ListingPriceMin      float64
	ListingPriceMax      float64
	TransactionPriceMin  float64
	TransactionPriceMax  float64
	TransactionMomentum  domainneighborhood.TransactionMomentum
	TargetLayoutSupply   int
	CalculatedAt         time.Time
}

type MetricWithSignal struct {
	Metric MetricSnapshot
	Signal domainneighborhood.SignalResult
}
