package gormrepo

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	appneighborhood "github.com/sine-io/propulse/backend/internal/application/neighborhood"
	domainneighborhood "github.com/sine-io/propulse/backend/internal/domain/neighborhood"
	migraterunner "github.com/sine-io/propulse/backend/internal/infrastructure/migrate"
)

func TestNeighborhoodRepositoryPersistsWatchlistAndLatestMetric(t *testing.T) {
	databaseURL := os.Getenv("PROPULSE_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("PROPULSE_TEST_DATABASE_URL is not set")
	}

	ctx := context.Background()
	if err := migraterunner.Run(ctx, databaseURL, "up"); err != nil {
		t.Fatalf("Run(up) error = %v", err)
	}

	db, sqlDB, err := Open(databaseURL)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer sqlDB.Close()

	repo := NewNeighborhoodRepository(db)
	neighborhood, err := repo.CreateNeighborhood(ctx, appneighborhood.CreateNeighborhoodInput{
		ID:           uuid.NewString(),
		Name:         "青枫花园",
		Area:         "滨江核心",
		TargetLayout: "三房",
	})
	if err != nil {
		t.Fatalf("CreateNeighborhood() error = %v", err)
	}
	if _, err := repo.AddWatchlistItem(ctx, "demo-user", neighborhood.ID); err != nil {
		t.Fatalf("AddWatchlistItem() error = %v", err)
	}

	metricID := uuid.NewString()
	if err := db.WithContext(ctx).Create(&NeighborhoodMetricModel{
		ID:                  metricID,
		NeighborhoodID:      neighborhood.ID,
		ListedHomes:         42,
		PriceCutHomes:       11,
		AvgDaysOnMarket:     78,
		ListingPriceMin:     520,
		ListingPriceMax:     620,
		TransactionPriceMin: 495,
		TransactionPriceMax: 545,
		TransactionMomentum: string(domainneighborhood.TransactionMomentumWeak),
		TargetLayoutSupply:  12,
	}).Error; err != nil {
		t.Fatalf("Create(metric) error = %v", err)
	}

	watchlist, err := repo.ListWatchlist(ctx, "demo-user")
	if err != nil {
		t.Fatalf("ListWatchlist() error = %v", err)
	}
	if len(watchlist) == 0 {
		t.Fatal("watchlist is empty")
	}
	if watchlist[0].Name != "青枫花园" || watchlist[0].Metric.ID != metricID {
		t.Fatalf("watchlist[0] = %#v", watchlist[0])
	}

	metric, err := repo.LatestMetric(ctx, neighborhood.ID)
	if err != nil {
		t.Fatalf("LatestMetric() error = %v", err)
	}
	if metric.TransactionMomentum != domainneighborhood.TransactionMomentumWeak {
		t.Fatalf("TransactionMomentum = %q, want weak", metric.TransactionMomentum)
	}
}

func TestNeighborhoodRepositoryListWatchlistUsesConfiguredMetricReader(t *testing.T) {
	databaseURL := os.Getenv("PROPULSE_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("PROPULSE_TEST_DATABASE_URL is not set")
	}

	ctx := context.Background()
	if err := migraterunner.Run(ctx, databaseURL, "up"); err != nil {
		t.Fatalf("Run(up) error = %v", err)
	}

	db, sqlDB, err := Open(databaseURL)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer sqlDB.Close()

	neighborhoodID := uuid.NewString()
	baseRepo := NewNeighborhoodRepository(db)
	neighborhood, err := baseRepo.CreateNeighborhood(ctx, appneighborhood.CreateNeighborhoodInput{
		ID:           neighborhoodID,
		Name:         "云澜府",
		Area:         "城东新区",
		TargetLayout: "四房",
	})
	if err != nil {
		t.Fatalf("CreateNeighborhood() error = %v", err)
	}
	if _, err := baseRepo.AddWatchlistItem(ctx, "reader-user", neighborhood.ID); err != nil {
		t.Fatalf("AddWatchlistItem() error = %v", err)
	}

	reader := &recordingMetricReader{
		metric: appneighborhood.MetricSnapshot{
			ID:                  "metric-from-reader",
			NeighborhoodID:      neighborhood.ID,
			ListedHomes:         14,
			PriceCutHomes:       1,
			TransactionMomentum: domainneighborhood.TransactionMomentumStrong,
			CalculatedAt:        time.Date(2026, 7, 9, 9, 0, 0, 0, time.UTC),
		},
	}
	repo := NewNeighborhoodRepositoryWithMetricReader(db, reader)

	watchlist, err := repo.ListWatchlist(ctx, "reader-user")
	if err != nil {
		t.Fatalf("ListWatchlist() error = %v", err)
	}
	if len(watchlist) != 1 {
		t.Fatalf("len(watchlist) = %d, want 1", len(watchlist))
	}
	if reader.calledWith != neighborhood.ID {
		t.Fatalf("reader called with %q, want %q", reader.calledWith, neighborhood.ID)
	}
	if !watchlist[0].HasMetric || watchlist[0].Metric.ID != "metric-from-reader" {
		t.Fatalf("watchlist[0] metric = %#v", watchlist[0])
	}
}
