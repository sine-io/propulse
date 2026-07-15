package gormrepo

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	appneighborhood "github.com/sine-io/propulse/internal/application/neighborhood"
	domainneighborhood "github.com/sine-io/propulse/internal/domain/neighborhood"
	migraterunner "github.com/sine-io/propulse/internal/infrastructure/migrate"
)

func TestNeighborhoodRepositoryPersistsWatchlistWithoutInventingMetric(t *testing.T) {
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
	t.Cleanup(func() {
		if err := sqlDB.Close(); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	})

	repo := NewNeighborhoodRepository(db)
	name := "青枫花园-" + uuid.NewString()
	userID := "watchlist-user-" + uuid.NewString()
	neighborhood, err := repo.CreateNeighborhood(ctx, appneighborhood.CreateNeighborhoodInput{
		ID:               uuid.NewString(),
		Name:             name,
		City:             "杭州",
		Area:             "滨江核心",
		AvailableLayouts: []string{"三房", "四房"},
	})
	if err != nil {
		t.Fatalf("CreateNeighborhood() error = %v", err)
	}
	if _, err := repo.AddWatchlistItem(ctx, userID, neighborhood.ID, "三房"); err != nil {
		t.Fatalf("AddWatchlistItem() error = %v", err)
	}

	watchlist, err := repo.ListWatchlist(ctx, userID)
	if err != nil {
		t.Fatalf("ListWatchlist() error = %v", err)
	}
	if len(watchlist) == 0 {
		t.Fatal("watchlist is empty")
	}
	if watchlist[0].Name != name || watchlist[0].TargetLayout != "三房" || watchlist[0].HasMetric {
		t.Fatalf("watchlist[0] = %#v", watchlist[0])
	}

	if _, err := repo.LatestMetric(ctx, neighborhood.ID); !errors.Is(err, appneighborhood.ErrMetricNotFound) {
		t.Fatalf("LatestMetric() error = %v, want ErrMetricNotFound", err)
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
	t.Cleanup(func() {
		if err := sqlDB.Close(); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	})

	neighborhoodID := uuid.NewString()
	name := "云澜府-" + uuid.NewString()
	userID := "reader-user-" + uuid.NewString()
	baseRepo := NewNeighborhoodRepository(db)
	neighborhood, err := baseRepo.CreateNeighborhood(ctx, appneighborhood.CreateNeighborhoodInput{
		ID:               neighborhoodID,
		Name:             name,
		City:             "杭州",
		Area:             "城东新区",
		AvailableLayouts: []string{"四房"},
	})
	if err != nil {
		t.Fatalf("CreateNeighborhood() error = %v", err)
	}
	if _, err := baseRepo.AddWatchlistItem(ctx, userID, neighborhood.ID, "四房"); err != nil {
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

	watchlist, err := repo.ListWatchlist(ctx, userID)
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

func TestNeighborhoodRepositorySearchesTrustedCatalogWithStablePagination(t *testing.T) {
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
	t.Cleanup(func() { _ = sqlDB.Close() })

	repo := NewNeighborhoodRepository(db)
	suffix := uuid.NewString()
	cityA := "目录城市甲-" + suffix
	cityB := "目录城市乙-" + suffix
	areaA := "核心板块-" + suffix
	areaB := "次级板块-" + suffix
	firstID := uuid.NewString()
	for _, input := range []appneighborhood.CreateNeighborhoodInput{
		{ID: firstID, Name: "A目录花园", City: cityA, Area: areaA, AvailableLayouts: []string{"三房", "四房"}},
		{ID: uuid.NewString(), Name: "B目录花园", City: cityA, Area: areaA, AvailableLayouts: []string{"三房"}},
		{ID: uuid.NewString(), Name: "伽马花园", City: cityA, Area: areaB, AvailableLayouts: []string{"两房"}},
		{ID: uuid.NewString(), Name: "德尔塔花园", City: cityB, Area: areaA, AvailableLayouts: []string{"三房"}},
	} {
		if _, err := repo.CreateNeighborhood(ctx, input); err != nil {
			t.Fatalf("CreateNeighborhood(%q) error = %v", input.Name, err)
		}
	}
	if err := db.Exec("INSERT INTO neighborhoods (id, name, area) VALUES (?, ?, ?)", uuid.NewString(), "未知城市花园", areaA).Error; err != nil {
		t.Fatalf("insert unknown-city neighborhood: %v", err)
	}

	result, err := repo.SearchNeighborhoods(ctx, appneighborhood.SearchNeighborhoodsInput{
		Query: "花园", City: cityA, Area: areaA, TargetLayout: "三房", Limit: 1,
	})
	if err != nil {
		t.Fatalf("SearchNeighborhoods() error = %v", err)
	}
	if result.Total != 2 || len(result.Items) != 1 || result.Items[0].ID != firstID {
		t.Fatalf("first page = %#v, total %d", result.Items, result.Total)
	}
	if len(result.Items[0].AvailableLayouts) != 2 || result.Items[0].City == nil || *result.Items[0].City != cityA {
		t.Fatalf("search item catalog = %#v", result.Items[0])
	}
	if !containsString(result.Filters.Cities, cityA) || !containsString(result.Filters.Cities, cityB) || !containsAreaFilter(result.Filters.Areas, cityA, areaB) {
		t.Fatalf("unpaginated filters = %#v", result.Filters)
	}

	secondPage, err := repo.SearchNeighborhoods(ctx, appneighborhood.SearchNeighborhoodsInput{
		City: cityA, Area: areaA, TargetLayout: "三房", Limit: 1, Offset: 1,
	})
	if err != nil || len(secondPage.Items) != 1 || secondPage.Items[0].Name != "B目录花园" {
		t.Fatalf("second page = %#v, error %v", secondPage, err)
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func containsAreaFilter(values []appneighborhood.NeighborhoodAreaFilter, city, area string) bool {
	for _, value := range values {
		if value.City == city && value.Area == area {
			return true
		}
	}
	return false
}
