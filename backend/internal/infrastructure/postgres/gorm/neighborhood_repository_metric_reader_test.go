package gormrepo

import (
	"context"
	"testing"

	appneighborhood "github.com/sine-io/propulse/backend/internal/application/neighborhood"
	domainneighborhood "github.com/sine-io/propulse/backend/internal/domain/neighborhood"
)

func TestNeighborhoodRepositoryLatestMetricUsesConfiguredReader(t *testing.T) {
	reader := &recordingMetricReader{
		metric: appneighborhood.MetricSnapshot{
			ID:                  "metric-from-sqlc",
			NeighborhoodID:      "neighborhood-1",
			ListedHomes:         42,
			TransactionMomentum: domainneighborhood.TransactionMomentumWeak,
		},
	}
	repo := NewNeighborhoodRepositoryWithMetricReader(nil, reader)

	got, err := repo.LatestMetric(context.Background(), "neighborhood-1")
	if err != nil {
		t.Fatalf("LatestMetric() error = %v", err)
	}

	if reader.calledWith != "neighborhood-1" {
		t.Fatalf("reader called with %q, want neighborhood-1", reader.calledWith)
	}
	if got.ID != "metric-from-sqlc" {
		t.Fatalf("metric ID = %q, want metric-from-sqlc", got.ID)
	}
}

type recordingMetricReader struct {
	calledWith string
	metric     appneighborhood.MetricSnapshot
	err        error
}

func (r *recordingMetricReader) LatestMetric(_ context.Context, neighborhoodID string) (appneighborhood.MetricSnapshot, error) {
	r.calledWith = neighborhoodID
	return r.metric, r.err
}
