package sqlmetric

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

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

func (r *Repository) GetCompletedCollectionRun(ctx context.Context, id string) (appmetric.CompletedCollectionRun, error) {
	runID, err := uuidParam(id)
	if err != nil {
		return appmetric.CompletedCollectionRun{}, err
	}
	row, err := r.queries.GetCompletedCollectionRun(ctx, runID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return appmetric.CompletedCollectionRun{}, appmetric.ErrCollectionRunNotFound
		}
		return appmetric.CompletedCollectionRun{}, err
	}
	return completedRunFromGetRow(row), nil
}

func (r *Repository) LatestCompletedCollectionRun(ctx context.Context, neighborhoodID string) (appmetric.CompletedCollectionRun, error) {
	id, err := uuidParam(neighborhoodID)
	if err != nil {
		return appmetric.CompletedCollectionRun{}, err
	}
	row, err := r.queries.LatestCompletedCollectionRun(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return appmetric.CompletedCollectionRun{}, appmetric.ErrCollectionRunNotFound
		}
		return appmetric.CompletedCollectionRun{}, err
	}
	return completedRunFromLatestRow(row), nil
}

func (r *Repository) AggregateMarketObservations(ctx context.Context, params appmetric.AggregateMarketParams) (appmetric.MarketAggregate, error) {
	neighborhoodID, err := uuidParam(params.NeighborhoodID)
	if err != nil {
		return appmetric.MarketAggregate{}, err
	}
	triggerRunID, err := uuidParam(params.TriggerRunID)
	if err != nil {
		return appmetric.MarketAggregate{}, err
	}

	row, err := r.queries.AggregateMarketObservations(ctx, sqlc.AggregateMarketObservationsParams{
		NeighborhoodID: neighborhoodID,
		TriggerRunID:   triggerRunID,
		TargetLayout:   params.TargetLayout,
	})
	if err != nil {
		return appmetric.MarketAggregate{}, err
	}
	return marketAggregateFromRow(row)
}

func (r *Repository) UpsertNeighborhoodMetric(ctx context.Context, snapshot appmetric.MetricSnapshot) (appmetric.MetricSnapshot, error) {
	neighborhoodID, err := uuidParam(snapshot.NeighborhoodID)
	if err != nil {
		return appmetric.MetricSnapshot{}, err
	}
	collectionRunID, err := uuidParam(snapshot.CollectionRunID)
	if err != nil {
		return appmetric.MetricSnapshot{}, err
	}
	inventoryRunID, err := optionalUUIDParam(snapshot.InventoryCollectionRunID)
	if err != nil {
		return appmetric.MetricSnapshot{}, err
	}
	sourceIDs, err := json.Marshal(snapshot.SourceIDs)
	if err != nil {
		return appmetric.MetricSnapshot{}, err
	}
	warnings, err := json.Marshal(snapshot.QualityWarnings)
	if err != nil {
		return appmetric.MetricSnapshot{}, err
	}
	row, err := r.queries.UpsertNeighborhoodMetric(ctx, sqlc.UpsertNeighborhoodMetricParams{
		NeighborhoodID:           neighborhoodID,
		ListedHomes:              int32(snapshot.ListedHomes),
		PriceCutHomes:            int32(snapshot.PriceCutHomes),
		AvgDaysOnMarket:          numericPtrParam(snapshot.AvgDaysOnMarket),
		ListingPriceMin:          numericPtrParam(snapshot.ListingPriceMin),
		ListingPriceMax:          numericPtrParam(snapshot.ListingPriceMax),
		TransactionPriceMin:      numericPtrParam(snapshot.TransactionPriceMin),
		TransactionPriceMax:      numericPtrParam(snapshot.TransactionPriceMax),
		TransactionMomentum:      string(snapshot.TransactionMomentum),
		TargetLayoutSupply:       int32(snapshot.TargetLayoutSupply),
		CollectionRunID:          collectionRunID,
		InventoryCollectionRunID: inventoryRunID,
		SourceIds:                sourceIDs,
		ListingSampleCount:       int32(snapshot.ListingSampleCount),
		TransactionSampleCount:   int32(snapshot.TransactionSampleCount),
		ListedHomesChangePct:     numericPtrParam(snapshot.ListedHomesChangePct),
		Coverage:                 string(snapshot.Coverage),
		Freshness:                string(snapshot.Freshness),
		QualityState:             string(snapshot.QualityState),
		LatestObservedAt:         timeParam(snapshot.LatestObservedAt),
		InventoryCollectedAt:     optionalTimeParam(snapshot.InventoryCollectedAt),
		QualityWarnings:          warnings,
	})
	if err != nil {
		return appmetric.MetricSnapshot{}, err
	}
	return metricFromRow(row), nil
}

func (r *Repository) MarkCollectionRunMetricCompleted(ctx context.Context, collectionRunID string) error {
	id, err := uuidParam(collectionRunID)
	if err != nil {
		return err
	}
	_, err = r.queries.MarkCollectionRunMetricCompleted(ctx, id)
	return err
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

func (r *Repository) ListMetricHistory(ctx context.Context, neighborhoodID string, since time.Time) ([]appneighborhood.MetricSnapshot, error) {
	id, err := uuidParam(neighborhoodID)
	if err != nil {
		return nil, err
	}
	rows, err := r.queries.ListNeighborhoodMetricHistory(ctx, sqlc.ListNeighborhoodMetricHistoryParams{
		NeighborhoodID: id,
		CollectedAt:    pgtype.Timestamptz{Time: since, Valid: true},
	})
	if err != nil {
		return nil, err
	}
	metrics := make([]appneighborhood.MetricSnapshot, 0, len(rows))
	for _, row := range rows {
		metrics = append(metrics, neighborhoodMetricFromRow(row))
	}
	return metrics, nil
}

func metricFromRow(row sqlc.NeighborhoodMetric) appmetric.MetricSnapshot {
	return appmetric.MetricSnapshot{
		ID:                       uuidString(row.ID),
		NeighborhoodID:           uuidString(row.NeighborhoodID),
		CollectionRunID:          uuidString(row.CollectionRunID),
		InventoryCollectionRunID: uuidStringPtr(row.InventoryCollectionRunID),
		SourceIDs:                stringSlice(row.SourceIds),
		LatestObservedAt:         row.LatestObservedAt.Time,
		ListedHomes:              int(row.ListedHomes),
		PriceCutHomes:            int(row.PriceCutHomes),
		AvgDaysOnMarket:          numericFloatPtr(row.AvgDaysOnMarket),
		ListingPriceMin:          numericFloatPtr(row.ListingPriceMin),
		ListingPriceMax:          numericFloatPtr(row.ListingPriceMax),
		TransactionPriceMin:      numericFloatPtr(row.TransactionPriceMin),
		TransactionPriceMax:      numericFloatPtr(row.TransactionPriceMax),
		TransactionMomentum:      domainneighborhood.TransactionMomentum(row.TransactionMomentum),
		TargetLayoutSupply:       int(row.TargetLayoutSupply),
		ListingSampleCount:       int(row.ListingSampleCount),
		TransactionSampleCount:   int(row.TransactionSampleCount),
		Coverage:                 domainneighborhood.Coverage(row.Coverage),
		Freshness:                domainneighborhood.Freshness(row.Freshness),
		InventoryCollectedAt:     timePtr(row.InventoryCollectedAt),
		ListedHomesChangePct:     numericFloatPtr(row.ListedHomesChangePct),
		QualityWarnings:          qualityWarnings(row.QualityWarnings),
		QualityState:             domainneighborhood.MarketQualityState(row.QualityState),
		CalculatedAt:             row.CalculatedAt.Time,
	}
}

func neighborhoodMetricFromRow(row sqlc.NeighborhoodMetric) appneighborhood.MetricSnapshot {
	return appneighborhood.MetricSnapshot{
		ID:                       uuidString(row.ID),
		NeighborhoodID:           uuidString(row.NeighborhoodID),
		CollectionRunID:          uuidString(row.CollectionRunID),
		InventoryCollectionRunID: uuidStringPtr(row.InventoryCollectionRunID),
		SourceIDs:                stringSlice(row.SourceIds),
		LatestObservedAt:         row.LatestObservedAt.Time,
		ListedHomes:              int(row.ListedHomes),
		PriceCutHomes:            int(row.PriceCutHomes),
		AvgDaysOnMarket:          numericFloatPtr(row.AvgDaysOnMarket),
		ListingPriceMin:          numericFloatPtr(row.ListingPriceMin),
		ListingPriceMax:          numericFloatPtr(row.ListingPriceMax),
		TransactionPriceMin:      numericFloatPtr(row.TransactionPriceMin),
		TransactionPriceMax:      numericFloatPtr(row.TransactionPriceMax),
		TransactionMomentum:      domainneighborhood.TransactionMomentum(row.TransactionMomentum),
		TargetLayoutSupply:       int(row.TargetLayoutSupply),
		ListingSampleCount:       int(row.ListingSampleCount),
		TransactionSampleCount:   int(row.TransactionSampleCount),
		Coverage:                 domainneighborhood.Coverage(row.Coverage),
		Freshness:                domainneighborhood.Freshness(row.Freshness),
		InventoryCollectedAt:     timePtr(row.InventoryCollectedAt),
		ListedHomesChangePct:     numericFloatPtr(row.ListedHomesChangePct),
		QualityWarnings:          qualityWarnings(row.QualityWarnings),
		QualityState:             domainneighborhood.MarketQualityState(row.QualityState),
		CalculatedAt:             row.CalculatedAt.Time,
	}
}

func completedRunFromGetRow(row sqlc.GetCompletedCollectionRunRow) appmetric.CompletedCollectionRun {
	return appmetric.CompletedCollectionRun{
		ID:             uuidString(row.ID),
		DataSourceID:   uuidString(row.DataSourceID),
		NeighborhoodID: uuidString(row.NeighborhoodID),
		CollectedAt:    row.CollectedAt.Time,
		Coverage:       domainneighborhood.Coverage(row.Coverage),
	}
}

func completedRunFromLatestRow(row sqlc.LatestCompletedCollectionRunRow) appmetric.CompletedCollectionRun {
	return appmetric.CompletedCollectionRun{
		ID:             uuidString(row.ID),
		DataSourceID:   uuidString(row.DataSourceID),
		NeighborhoodID: uuidString(row.NeighborhoodID),
		CollectedAt:    row.CollectedAt.Time,
		Coverage:       domainneighborhood.Coverage(row.Coverage),
	}
}

func marketAggregateFromRow(row sqlc.AggregateMarketObservationsRow) (appmetric.MarketAggregate, error) {
	listedChange, err := numericAnyFloatPtr(row.ListedHomesChangePct)
	if err != nil {
		return appmetric.MarketAggregate{}, err
	}
	return appmetric.MarketAggregate{
		CollectionRunID:                   uuidString(row.CollectionRunID),
		InventoryCollectionRunID:          uuidStringPtr(row.InventoryCollectionRunID),
		SourceIDs:                         stringSliceAny(row.SourceIds),
		LatestObservedAt:                  row.LatestObservedAt.Time,
		InventoryCollectedAt:              timePtr(row.InventoryCollectedAt),
		Coverage:                          domainneighborhood.Coverage(row.Coverage),
		ListedHomes:                       int(row.ListedHomes),
		PriceCutHomes:                     int(row.PriceCutHomes),
		AvgDaysOnMarket:                   numericFloatPtr(row.AvgDaysOnMarket),
		ListingPriceMin:                   numericFloatPtr(row.ListingPriceMin),
		ListingPriceMax:                   numericFloatPtr(row.ListingPriceMax),
		TransactionPriceMin:               numericFloatPtr(row.TransactionPriceMin),
		TransactionPriceMax:               numericFloatPtr(row.TransactionPriceMax),
		TargetLayoutSupply:                int(row.TargetLayoutSupply),
		ListingSampleCount:                int(row.ListingSampleCount),
		TransactionSampleCount:            int(row.TransactionSampleCount),
		LastThirtyDayTransactionCount:     int(row.LastThirtyDayTransactionCount),
		PrecedingSixtyDayTransactionCount: int(row.PrecedingSixtyDayTransactionCount),
		ListedHomesChangePct:              listedChange,
	}, nil
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

func uuidStringPtr(value pgtype.UUID) *string {
	if !value.Valid {
		return nil
	}
	result := uuidString(value)
	return &result
}

func optionalUUIDParam(value *string) (pgtype.UUID, error) {
	if value == nil || *value == "" {
		return pgtype.UUID{}, nil
	}
	return uuidParam(*value)
}

func numericParam(value float64) pgtype.Numeric {
	var numeric pgtype.Numeric
	_ = numeric.Scan(strconv.FormatFloat(value, 'f', -1, 64))
	return numeric
}

func numericPtrParam(value *float64) pgtype.Numeric {
	if value == nil {
		return pgtype.Numeric{}
	}
	return numericParam(*value)
}

func numericFloatPtr(value pgtype.Numeric) *float64 {
	floatValue, err := value.Float64Value()
	if err != nil || !floatValue.Valid {
		return nil
	}
	return &floatValue.Float64
}

func numericAnyFloatPtr(value any) (*float64, error) {
	switch v := value.(type) {
	case nil:
		return nil, nil
	case pgtype.Numeric:
		return numericFloatPtr(v), nil
	case []byte:
		parsed, err := strconv.ParseFloat(string(v), 64)
		if err != nil {
			return nil, err
		}
		return &parsed, nil
	case string:
		parsed, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return nil, err
		}
		return &parsed, nil
	default:
		return nil, fmt.Errorf("unsupported numeric value %T", value)
	}
}

func timeParam(value time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: value, Valid: !value.IsZero()}
}

func optionalTimeParam(value *time.Time) pgtype.Timestamptz {
	if value == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: *value, Valid: true}
}

func timePtr(value pgtype.Timestamptz) *time.Time {
	if !value.Valid {
		return nil
	}
	return &value.Time
}

func stringSlice(raw []byte) []string {
	if len(raw) == 0 {
		return nil
	}
	var values []string
	_ = json.Unmarshal(raw, &values)
	return values
}

func stringSliceAny(raw any) []string {
	switch v := raw.(type) {
	case nil:
		return nil
	case []byte:
		return stringSlice(v)
	case string:
		return stringSlice([]byte(v))
	default:
		encoded, _ := json.Marshal(v)
		return stringSlice(encoded)
	}
}

func qualityWarnings(raw []byte) []domainneighborhood.QualityWarning {
	if len(raw) == 0 {
		return nil
	}
	var warnings []domainneighborhood.QualityWarning
	_ = json.Unmarshal(raw, &warnings)
	return warnings
}
