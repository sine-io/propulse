package sqlmetric

import (
	"context"
	"errors"
	"strconv"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	appmetric "github.com/sine-io/propulse/internal/application/metric"
	appneighborhood "github.com/sine-io/propulse/internal/application/neighborhood"
	domainneighborhood "github.com/sine-io/propulse/internal/domain/neighborhood"
	"github.com/sine-io/propulse/internal/infrastructure/postgres/sqlc"
)

type Repository struct {
	db      sqlc.DBTX
	queries *sqlc.Queries
}

var _ appmetric.Repository = (*Repository)(nil)

func NewRepository(db sqlc.DBTX) *Repository {
	return &Repository{
		db:      db,
		queries: sqlc.New(db),
	}
}

func (r *Repository) GetNeighborhood(ctx context.Context, id string) (appmetric.Neighborhood, error) {
	neighborhoodID, err := uuidParam(id)
	if err != nil {
		return appmetric.Neighborhood{}, err
	}

	var row sqlc.Neighborhood
	err = r.db.QueryRow(ctx, `
SELECT id, name, area, target_layout, created_at
FROM neighborhoods
WHERE id = $1
`, neighborhoodID).Scan(
		&row.ID,
		&row.Name,
		&row.Area,
		&row.TargetLayout,
		&row.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return appmetric.Neighborhood{}, appmetric.ErrNeighborhoodNotFound
		}
		return appmetric.Neighborhood{}, err
	}

	return appmetric.Neighborhood{
		ID:           uuidString(row.ID),
		TargetLayout: row.TargetLayout,
	}, nil
}

func (r *Repository) AggregateListingSnapshots(ctx context.Context, neighborhoodID string, targetLayout string) (appmetric.ListingSnapshotAggregate, error) {
	id, err := uuidParam(neighborhoodID)
	if err != nil {
		return appmetric.ListingSnapshotAggregate{}, err
	}

	row, err := r.queries.AggregateListingSnapshots(ctx, sqlc.AggregateListingSnapshotsParams{
		TargetLayout:   targetLayout,
		NeighborhoodID: id,
	})
	if err != nil {
		return appmetric.ListingSnapshotAggregate{}, err
	}

	return appmetric.ListingSnapshotAggregate{
		ListedHomes:         int(row.ListedHomes),
		PriceCutHomes:       int(row.PriceCutHomes),
		AvgDaysOnMarket:     numericFloat(row.AvgDaysOnMarket),
		ListingPriceMin:     numericFloat(row.ListingPriceMin),
		ListingPriceMax:     numericFloat(row.ListingPriceMax),
		TransactionPriceMin: numericFloat(row.TransactionPriceMin),
		TransactionPriceMax: numericFloat(row.TransactionPriceMax),
		TargetLayoutSupply:  int(row.TargetLayoutSupply),
	}, nil
}

func (r *Repository) InsertNeighborhoodMetric(ctx context.Context, snapshot appmetric.MetricSnapshot) (appmetric.MetricSnapshot, error) {
	neighborhoodID, err := uuidParam(snapshot.NeighborhoodID)
	if err != nil {
		return appmetric.MetricSnapshot{}, err
	}

	row, err := r.queries.InsertNeighborhoodMetric(ctx, sqlc.InsertNeighborhoodMetricParams{
		NeighborhoodID:      neighborhoodID,
		ListedHomes:         int32(snapshot.ListedHomes),
		PriceCutHomes:       int32(snapshot.PriceCutHomes),
		AvgDaysOnMarket:     numericParam(snapshot.AvgDaysOnMarket),
		ListingPriceMin:     numericParam(snapshot.ListingPriceMin),
		ListingPriceMax:     numericParam(snapshot.ListingPriceMax),
		TransactionPriceMin: numericParam(snapshot.TransactionPriceMin),
		TransactionPriceMax: numericParam(snapshot.TransactionPriceMax),
		TransactionMomentum: string(snapshot.TransactionMomentum),
		TargetLayoutSupply:  int32(snapshot.TargetLayoutSupply),
	})
	if err != nil {
		return appmetric.MetricSnapshot{}, err
	}

	return metricFromRow(row), nil
}

func (r *Repository) LatestMetric(ctx context.Context, neighborhoodID string) (appneighborhood.MetricSnapshot, error) {
	id, err := uuidParam(neighborhoodID)
	if err != nil {
		return appneighborhood.MetricSnapshot{}, err
	}

	row, err := r.queries.LatestNeighborhoodMetric(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return appneighborhood.MetricSnapshot{}, appneighborhood.ErrMetricNotFound
		}
		return appneighborhood.MetricSnapshot{}, err
	}

	return neighborhoodMetricFromRow(row), nil
}

func metricFromRow(row sqlc.NeighborhoodMetric) appmetric.MetricSnapshot {
	return appmetric.MetricSnapshot{
		ID:                  uuidString(row.ID),
		NeighborhoodID:      uuidString(row.NeighborhoodID),
		ListedHomes:         int(row.ListedHomes),
		PriceCutHomes:       int(row.PriceCutHomes),
		AvgDaysOnMarket:     numericFloat(row.AvgDaysOnMarket),
		ListingPriceMin:     numericFloat(row.ListingPriceMin),
		ListingPriceMax:     numericFloat(row.ListingPriceMax),
		TransactionPriceMin: numericFloat(row.TransactionPriceMin),
		TransactionPriceMax: numericFloat(row.TransactionPriceMax),
		TransactionMomentum: domainneighborhood.TransactionMomentum(row.TransactionMomentum),
		TargetLayoutSupply:  int(row.TargetLayoutSupply),
		CalculatedAt:        row.CalculatedAt.Time,
	}
}

func neighborhoodMetricFromRow(row sqlc.NeighborhoodMetric) appneighborhood.MetricSnapshot {
	return appneighborhood.MetricSnapshot{
		ID:                  uuidString(row.ID),
		NeighborhoodID:      uuidString(row.NeighborhoodID),
		ListedHomes:         int(row.ListedHomes),
		PriceCutHomes:       int(row.PriceCutHomes),
		AvgDaysOnMarket:     numericFloat(row.AvgDaysOnMarket),
		ListingPriceMin:     numericFloat(row.ListingPriceMin),
		ListingPriceMax:     numericFloat(row.ListingPriceMax),
		TransactionPriceMin: numericFloat(row.TransactionPriceMin),
		TransactionPriceMax: numericFloat(row.TransactionPriceMax),
		TransactionMomentum: domainneighborhood.TransactionMomentum(row.TransactionMomentum),
		TargetLayoutSupply:  int(row.TargetLayoutSupply),
		CalculatedAt:        row.CalculatedAt.Time,
	}
}

func uuidParam(value string) (pgtype.UUID, error) {
	parsed, err := uuid.Parse(value)
	if err != nil {
		return pgtype.UUID{}, err
	}
	return pgtype.UUID{Bytes: [16]byte(parsed), Valid: true}, nil
}

func uuidString(value pgtype.UUID) string {
	if !value.Valid {
		return ""
	}
	return uuid.UUID(value.Bytes).String()
}

func numericParam(value float64) pgtype.Numeric {
	var numeric pgtype.Numeric
	_ = numeric.Scan(strconv.FormatFloat(value, 'f', -1, 64))
	return numeric
}

func numericFloat(value pgtype.Numeric) float64 {
	floatValue, err := value.Float64Value()
	if err != nil || !floatValue.Valid {
		return 0
	}
	return floatValue.Float64
}
