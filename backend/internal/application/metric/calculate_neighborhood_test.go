package metric

import (
	"context"
	"testing"
	"time"

	domainneighborhood "github.com/sine-io/propulse/backend/internal/domain/neighborhood"
)

func TestCalculateNeighborhoodAggregatesSnapshotsAndWritesMetric(t *testing.T) {
	repo := &memoryRepository{
		neighborhood: Neighborhood{
			ID:           "neighborhood_1",
			TargetLayout: "三房",
		},
		aggregate: ListingSnapshotAggregate{
			ListedHomes:         42,
			PriceCutHomes:       11,
			AvgDaysOnMarket:     78,
			ListingPriceMin:     520,
			ListingPriceMax:     620,
			TransactionPriceMin: 495,
			TransactionPriceMax: 545,
			TargetLayoutSupply:  12,
		},
		insertedMetric: MetricSnapshot{
			ID:           "metric_1",
			CalculatedAt: time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC),
		},
	}
	service := NewService(repo)

	err := service.CalculateNeighborhood(context.Background(), "neighborhood_1")
	if err != nil {
		t.Fatalf("CalculateNeighborhood() error = %v", err)
	}

	if repo.aggregateNeighborhoodID != "neighborhood_1" {
		t.Fatalf("aggregateNeighborhoodID = %q, want neighborhood_1", repo.aggregateNeighborhoodID)
	}
	if repo.aggregateTargetLayout != "三房" {
		t.Fatalf("aggregateTargetLayout = %q, want 三房", repo.aggregateTargetLayout)
	}
	if repo.insertCount != 1 {
		t.Fatalf("insertCount = %d, want 1", repo.insertCount)
	}
	got := repo.lastInserted
	if got.NeighborhoodID != "neighborhood_1" {
		t.Fatalf("NeighborhoodID = %q, want neighborhood_1", got.NeighborhoodID)
	}
	if got.ListedHomes != 42 || got.PriceCutHomes != 11 || got.TargetLayoutSupply != 12 {
		t.Fatalf("metric counts = listed %d, cuts %d, target %d", got.ListedHomes, got.PriceCutHomes, got.TargetLayoutSupply)
	}
	if got.AvgDaysOnMarket != 78 || got.ListingPriceMin != 520 || got.TransactionPriceMax != 545 {
		t.Fatalf("metric values = %#v", got)
	}
	if got.TransactionMomentum != domainneighborhood.TransactionMomentumWeak {
		t.Fatalf("TransactionMomentum = %q, want weak", got.TransactionMomentum)
	}
}

func TestCalculateNeighborhoodMapsStrongMomentum(t *testing.T) {
	repo := &memoryRepository{
		neighborhood: Neighborhood{ID: "neighborhood_1", TargetLayout: "四房"},
		aggregate: ListingSnapshotAggregate{
			ListedHomes:        14,
			PriceCutHomes:      1,
			AvgDaysOnMarket:    35,
			ListingPriceMin:    700,
			ListingPriceMax:    760,
			TargetLayoutSupply: 3,
		},
	}
	service := NewService(repo)

	if err := service.CalculateNeighborhood(context.Background(), "neighborhood_1"); err != nil {
		t.Fatalf("CalculateNeighborhood() error = %v", err)
	}

	if repo.lastInserted.TransactionMomentum != domainneighborhood.TransactionMomentumStrong {
		t.Fatalf("TransactionMomentum = %q, want strong", repo.lastInserted.TransactionMomentum)
	}
}

type memoryRepository struct {
	neighborhood Neighborhood
	aggregate    ListingSnapshotAggregate

	aggregateNeighborhoodID string
	aggregateTargetLayout   string
	lastInserted            MetricSnapshot
	insertedMetric          MetricSnapshot
	insertCount             int
}

func (m *memoryRepository) GetNeighborhood(_ context.Context, id string) (Neighborhood, error) {
	if m.neighborhood.ID != id {
		return Neighborhood{}, ErrNeighborhoodNotFound
	}
	return m.neighborhood, nil
}

func (m *memoryRepository) AggregateListingSnapshots(_ context.Context, neighborhoodID string, targetLayout string) (ListingSnapshotAggregate, error) {
	m.aggregateNeighborhoodID = neighborhoodID
	m.aggregateTargetLayout = targetLayout
	return m.aggregate, nil
}

func (m *memoryRepository) InsertNeighborhoodMetric(_ context.Context, snapshot MetricSnapshot) (MetricSnapshot, error) {
	m.insertCount++
	m.lastInserted = snapshot
	if m.insertedMetric.ID != "" {
		snapshot.ID = m.insertedMetric.ID
		snapshot.CalculatedAt = m.insertedMetric.CalculatedAt
	}
	return snapshot, nil
}
