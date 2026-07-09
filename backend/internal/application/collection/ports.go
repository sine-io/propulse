package collection

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

var ErrInvalidRequest = errors.New("invalid_request")
var ErrNeighborhoodNotFound = errors.New("neighborhood_not_found")
var ErrImportFailed = errors.New("import_failed")

type Repository interface {
	NeighborhoodExists(ctx context.Context, id string) (bool, error)
	SaveImport(ctx context.Context, raw RawCollectionRecord, snapshots []ListingSnapshot) error
}

type MetricCalculator interface {
	CalculateNeighborhood(ctx context.Context, neighborhoodID string) error
}

type RawCollectionRecord struct {
	ID          string
	SourceType  string
	SourceRef   string
	Payload     json.RawMessage
	CollectedAt time.Time
}

type ListingSnapshot struct {
	ID               string
	CollectionRunID  string
	NeighborhoodID   string
	ListingPrice     float64
	TransactionPrice *float64
	PriceCut         bool
	DaysOnMarket     int
	Layout           string
	CapturedAt       time.Time
}
