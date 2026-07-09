package sqlmetric

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	appneighborhood "github.com/sine-io/propulse/backend/internal/application/neighborhood"
	domainneighborhood "github.com/sine-io/propulse/backend/internal/domain/neighborhood"
)

func TestRepositoryLatestMetricMapsSqlcRow(t *testing.T) {
	metricID := uuid.New()
	neighborhoodID := uuid.New()
	calculatedAt := time.Date(2026, 7, 9, 10, 11, 12, 0, time.UTC)

	db := &latestMetricDB{
		row: latestMetricRow{
			values: []any{
				uuidValue(metricID),
				uuidValue(neighborhoodID),
				int32(42),
				int32(11),
				numericValue(t, "78.5"),
				numericValue(t, "520.25"),
				numericValue(t, "620.75"),
				numericValue(t, "495.5"),
				numericValue(t, "545.5"),
				string(domainneighborhood.TransactionMomentumWeak),
				int32(12),
				pgtype.Timestamptz{Time: calculatedAt, Valid: true},
			},
		},
	}

	got, err := NewRepository(db).LatestMetric(context.Background(), neighborhoodID.String())
	if err != nil {
		t.Fatalf("LatestMetric() error = %v", err)
	}

	if db.neighborhoodID != uuidValue(neighborhoodID) {
		t.Fatalf("query neighborhoodID = %#v, want %#v", db.neighborhoodID, uuidValue(neighborhoodID))
	}
	if got.ID != metricID.String() || got.NeighborhoodID != neighborhoodID.String() {
		t.Fatalf("metric IDs = (%q, %q), want (%q, %q)", got.ID, got.NeighborhoodID, metricID.String(), neighborhoodID.String())
	}
	if got.ListedHomes != 42 || got.PriceCutHomes != 11 || got.TargetLayoutSupply != 12 {
		t.Fatalf("metric counts = %#v", got)
	}
	if got.AvgDaysOnMarket != 78.5 || got.ListingPriceMin != 520.25 || got.TransactionPriceMax != 545.5 {
		t.Fatalf("metric prices/days = %#v", got)
	}
	if got.TransactionMomentum != domainneighborhood.TransactionMomentumWeak {
		t.Fatalf("TransactionMomentum = %q, want weak", got.TransactionMomentum)
	}
	if !got.CalculatedAt.Equal(calculatedAt) {
		t.Fatalf("CalculatedAt = %s, want %s", got.CalculatedAt, calculatedAt)
	}
}

func TestRepositoryLatestMetricMapsNoRows(t *testing.T) {
	_, err := NewRepository(&latestMetricDB{row: latestMetricRow{err: pgx.ErrNoRows}}).LatestMetric(context.Background(), uuid.NewString())
	if !errors.Is(err, appneighborhood.ErrMetricNotFound) {
		t.Fatalf("LatestMetric() error = %v, want ErrMetricNotFound", err)
	}
}

func TestRepositoryAggregateListingSnapshotsUsesLatestCollectionRun(t *testing.T) {
	neighborhoodID := uuid.New()
	db := &latestMetricDB{
		row: latestMetricRow{
			values: []any{
				int32(2),
				int32(1),
				numericValue(t, "14"),
				numericValue(t, "520"),
				numericValue(t, "610"),
				numericValue(t, "495"),
				numericValue(t, "545"),
				int32(2),
			},
		},
	}

	_, err := NewRepository(db).AggregateListingSnapshots(context.Background(), neighborhoodID.String(), "三房")
	if err != nil {
		t.Fatalf("AggregateListingSnapshots() error = %v", err)
	}

	if !strings.Contains(db.query, "collection_run_id") {
		t.Fatalf("aggregate query does not scope by collection_run_id:\n%s", db.query)
	}
	if !strings.Contains(db.query, "MAX(captured_at)") {
		t.Fatalf("aggregate query does not select the latest captured run:\n%s", db.query)
	}
	if db.targetLayout != "三房" {
		t.Fatalf("targetLayout arg = %q, want 三房", db.targetLayout)
	}
	if db.neighborhoodID != uuidValue(neighborhoodID) {
		t.Fatalf("neighborhoodID arg = %#v, want %#v", db.neighborhoodID, uuidValue(neighborhoodID))
	}
}

type latestMetricDB struct {
	row            latestMetricRow
	query          string
	targetLayout   string
	neighborhoodID pgtype.UUID
}

func (db *latestMetricDB) Exec(context.Context, string, ...interface{}) (pgconn.CommandTag, error) {
	panic("unexpected Exec")
}

func (db *latestMetricDB) Query(context.Context, string, ...interface{}) (pgx.Rows, error) {
	panic("unexpected Query")
}

func (db *latestMetricDB) QueryRow(_ context.Context, query string, args ...interface{}) pgx.Row {
	db.query = query
	if len(args) == 1 {
		db.neighborhoodID = args[0].(pgtype.UUID)
	}
	if len(args) == 2 {
		db.targetLayout = args[0].(string)
		db.neighborhoodID = args[1].(pgtype.UUID)
	}
	return db.row
}

type latestMetricRow struct {
	values []any
	err    error
}

func (r latestMetricRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	for i, value := range r.values {
		switch target := dest[i].(type) {
		case *pgtype.UUID:
			*target = value.(pgtype.UUID)
		case *int32:
			*target = value.(int32)
		case *pgtype.Numeric:
			*target = value.(pgtype.Numeric)
		case *string:
			*target = value.(string)
		case *pgtype.Timestamptz:
			*target = value.(pgtype.Timestamptz)
		default:
			panic("unsupported scan target")
		}
	}
	return nil
}

func uuidValue(value uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: [16]byte(value), Valid: true}
}

func numericValue(t *testing.T, value string) pgtype.Numeric {
	t.Helper()

	var numeric pgtype.Numeric
	if err := numeric.Scan(value); err != nil {
		t.Fatalf("numeric.Scan(%q) error = %v", value, err)
	}
	return numeric
}
