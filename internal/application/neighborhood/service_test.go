package neighborhood

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/sine-io/propulse/internal/application/user"
	domainneighborhood "github.com/sine-io/propulse/internal/domain/neighborhood"
)

func TestCreateNeighborhoodPersistsInput(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(repo)

	got, err := service.CreateNeighborhood(context.Background(), CreateNeighborhoodCommand{
		Name:         "青枫花园",
		Area:         "滨江核心",
		TargetLayout: "三房",
	})
	if err != nil {
		t.Fatalf("CreateNeighborhood() error = %v", err)
	}

	if got.ID == "" {
		t.Fatal("ID is empty")
	}
	if got.Name != "青枫花园" || got.Area != "滨江核心" || got.TargetLayout != "三房" {
		t.Fatalf("Neighborhood = %#v", got)
	}
	if len(repo.neighborhoods) != 1 {
		t.Fatalf("saved neighborhoods = %d, want 1", len(repo.neighborhoods))
	}
}

func TestGetNeighborhoodReturnsStoredNeighborhood(t *testing.T) {
	repo := newMemoryRepository()
	repo.neighborhoods["neighborhood_1"] = Neighborhood{
		ID:           "neighborhood_1",
		Name:         "青枫花园",
		Area:         "滨江核心",
		TargetLayout: "三房",
	}
	service := NewService(repo)

	got, err := service.GetNeighborhood(context.Background(), GetNeighborhoodQuery{ID: "neighborhood_1"})
	if err != nil {
		t.Fatalf("GetNeighborhood() error = %v", err)
	}

	if got.ID != "neighborhood_1" {
		t.Fatalf("ID = %q, want neighborhood_1", got.ID)
	}
}

func TestGetNeighborhoodReturnsNotFound(t *testing.T) {
	service := NewService(newMemoryRepository())

	_, err := service.GetNeighborhood(context.Background(), GetNeighborhoodQuery{ID: "missing"})
	if !errors.Is(err, ErrNeighborhoodNotFound) {
		t.Fatalf("GetNeighborhood() error = %v, want ErrNeighborhoodNotFound", err)
	}
}

func TestAddWatchlistItemUsesSingleUser(t *testing.T) {
	repo := newMemoryRepository()
	repo.neighborhoods["neighborhood_1"] = Neighborhood{ID: "neighborhood_1"}
	service := NewService(repo)

	item, err := service.AddWatchlistItem(context.Background(), AddWatchlistItemCommand{
		UserID:         user.SingleUserID,
		NeighborhoodID: "neighborhood_1",
	})
	if err != nil {
		t.Fatalf("AddWatchlistItem() error = %v", err)
	}

	if item.UserID != user.SingleUserID {
		t.Fatalf("UserID = %q, want %q", item.UserID, user.SingleUserID)
	}
	if item.NeighborhoodID != "neighborhood_1" {
		t.Fatalf("NeighborhoodID = %q, want neighborhood_1", item.NeighborhoodID)
	}
}

func TestListWatchlistEvaluatesLatestMetric(t *testing.T) {
	repo := newMemoryRepository()
	repo.watchlist = []WatchlistSummary{
		{
			ID:             "watch_1",
			NeighborhoodID: "neighborhood_1",
			Name:           "青枫花园",
			Area:           "滨江核心",
			TargetLayout:   "三房",
			HasMetric:      true,
			Metric: MetricSnapshot{
				CollectionRunID:          "run_1",
				AlgorithmVersion:         "market-metrics/test.1",
				InventoryCollectionRunID: testStringPtr("run_1"),
				SourceIDs:                []string{"source_1"},
				CollectedAt:              time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC),
				ListedHomes:              42,
				ListedHomesChangePct:     testFloatPtr(18),
				PriceCutHomes:            11,
				AvgDaysOnMarket:          testFloatPtr(78),
				ListingPriceMin:          testFloatPtr(520),
				ListingPriceMax:          testFloatPtr(620),
				TransactionPriceMin:      testFloatPtr(495),
				TransactionPriceMax:      testFloatPtr(545),
				TransactionMomentum:      domainneighborhood.TransactionMomentumWeak,
				TargetLayoutSupply:       12,
				ListingSampleCount:       42,
				TransactionSampleCount:   3,
				Coverage:                 domainneighborhood.CoverageFull,
				Freshness:                domainneighborhood.FreshnessCurrent,
				QualityState:             domainneighborhood.MarketQualitySufficient,
				InventoryCollectedAt:     testTimePtr(time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC)),
				CalculatedAt:             time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC),
			},
		},
	}
	service := NewServiceWithMetricConfig(repo, "market-metrics/test.1", func() time.Time { return time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC) })

	got, err := service.ListWatchlist(context.Background(), ListWatchlistQuery{UserID: user.SingleUserID})
	if err != nil {
		t.Fatalf("ListWatchlist() error = %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("items = %d, want 1", len(got))
	}
	if got[0].Status != domainneighborhood.NeighborhoodStatusBargain {
		t.Fatalf("Status = %q, want %q", got[0].Status, domainneighborhood.NeighborhoodStatusBargain)
	}
	if got[0].Advice != "重点看 495-545 万成交区间附近房源，对挂牌久、降价过的房源试探底价。" {
		t.Fatalf("Advice = %q", got[0].Advice)
	}
}

func TestListWatchlistReturnsNeutralSummaryWithoutMetric(t *testing.T) {
	repo := newMemoryRepository()
	repo.watchlist = []WatchlistSummary{
		{
			ID:             "watch_1",
			NeighborhoodID: "neighborhood_1",
			Name:           "青枫花园",
			Area:           "滨江核心",
			TargetLayout:   "三房",
			HasMetric:      false,
		},
	}
	service := NewService(repo)

	got, err := service.ListWatchlist(context.Background(), ListWatchlistQuery{UserID: user.SingleUserID})
	if err != nil {
		t.Fatalf("ListWatchlist() error = %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("items = %d, want 1", len(got))
	}
	if got[0].Status != domainneighborhood.NeighborhoodStatusInsufficientData {
		t.Fatalf("Status = %q, want %q", got[0].Status, domainneighborhood.NeighborhoodStatusInsufficientData)
	}
	if got[0].Advice != "暂无指标数据，等待导入或计算后再判断。" {
		t.Fatalf("Advice = %q", got[0].Advice)
	}
	if got[0].ListedHomes != 0 || got[0].PriceCutHomes != 0 || got[0].TransactionMomentum != domainneighborhood.TransactionMomentumUnknown || got[0].HasMetric || got[0].QualityState != domainneighborhood.MarketQualityInsufficientData {
		t.Fatalf("metric fields = listed %d, price cuts %d, momentum %q", got[0].ListedHomes, got[0].PriceCutHomes, got[0].TransactionMomentum)
	}
}

func TestListWatchlistRefreshesStaleMetricQualityBeforeEvaluatingSignal(t *testing.T) {
	collectedAt := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)
	repo := newMemoryRepository()
	repo.watchlist = []WatchlistSummary{
		{
			ID:             "watch_1",
			NeighborhoodID: "neighborhood_1",
			Name:           "青枫花园",
			HasMetric:      true,
			Metric: MetricSnapshot{
				CollectionRunID:          "run_1",
				AlgorithmVersion:         "market-metrics/test.1",
				InventoryCollectionRunID: testStringPtr("run_1"),
				CollectedAt:              collectedAt,
				ListedHomes:              20,
				PriceCutHomes:            8,
				TransactionMomentum:      domainneighborhood.TransactionMomentumWeak,
				ListingSampleCount:       20,
				TransactionSampleCount:   3,
				Coverage:                 domainneighborhood.CoverageFull,
				Freshness:                domainneighborhood.FreshnessCurrent,
				InventoryCollectedAt:     &collectedAt,
				QualityState:             domainneighborhood.MarketQualitySufficient,
			},
		},
	}
	service := NewServiceWithMetricConfig(repo, "market-metrics/test.1", func() time.Time {
		return collectedAt.Add(8 * 24 * time.Hour)
	})

	got, err := service.ListWatchlist(context.Background(), ListWatchlistQuery{UserID: user.SingleUserID})
	if err != nil {
		t.Fatalf("ListWatchlist() error = %v", err)
	}
	if len(got) != 1 || got[0].Freshness != domainneighborhood.FreshnessStale || got[0].QualityState != domainneighborhood.MarketQualityLowConfidence {
		t.Fatalf("quality = %#v", got)
	}
	if got[0].Status != domainneighborhood.NeighborhoodStatusInsufficientData || got[0].Advice == "" {
		t.Fatalf("signal = %#v", got[0])
	}
}

func TestLatestMetricEvaluatesSignal(t *testing.T) {
	repo := newMemoryRepository()
	repo.metrics["neighborhood_1"] = MetricSnapshot{
		CollectionRunID:          "run_1",
		AlgorithmVersion:         "market-metrics/test.1",
		InventoryCollectionRunID: testStringPtr("run_1"),
		SourceIDs:                []string{"source_1"},
		CollectedAt:              time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC),
		ListedHomes:              14,
		ListedHomesChangePct:     testFloatPtr(-6),
		PriceCutHomes:            1,
		AvgDaysOnMarket:          testFloatPtr(35),
		ListingPriceMin:          testFloatPtr(700),
		ListingPriceMax:          testFloatPtr(760),
		TransactionPriceMin:      testFloatPtr(690),
		TransactionPriceMax:      testFloatPtr(745),
		TransactionMomentum:      domainneighborhood.TransactionMomentumStrong,
		TargetLayoutSupply:       3,
		ListingSampleCount:       14,
		TransactionSampleCount:   3,
		Coverage:                 domainneighborhood.CoverageFull,
		Freshness:                domainneighborhood.FreshnessCurrent,
		QualityState:             domainneighborhood.MarketQualitySufficient,
		InventoryCollectedAt:     testTimePtr(time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC)),
		CalculatedAt:             time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC),
	}
	service := NewServiceWithMetricConfig(repo, "market-metrics/test.1", func() time.Time { return time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC) })

	got, err := service.LatestMetric(context.Background(), LatestMetricQuery{NeighborhoodID: "neighborhood_1"})
	if err != nil {
		t.Fatalf("LatestMetric() error = %v", err)
	}

	if got.Signal.Status != domainneighborhood.NeighborhoodStatusPriceHard {
		t.Fatalf("Status = %q, want %q", got.Signal.Status, domainneighborhood.NeighborhoodStatusPriceHard)
	}
}

type memoryRepository struct {
	neighborhoods map[string]Neighborhood
	metrics       map[string]MetricSnapshot
	watchlist     []WatchlistSummary
	history       []MetricHistoryRecord
	nextID        int
}

func newMemoryRepository() *memoryRepository {
	return &memoryRepository{
		neighborhoods: map[string]Neighborhood{},
		metrics:       map[string]MetricSnapshot{},
		nextID:        1,
	}
}

func (m *memoryRepository) CreateNeighborhood(_ context.Context, input CreateNeighborhoodInput) (Neighborhood, error) {
	id := "neighborhood_test"
	if input.ID != "" {
		id = input.ID
	}
	neighborhood := Neighborhood{
		ID:           id,
		Name:         input.Name,
		Area:         input.Area,
		TargetLayout: input.TargetLayout,
	}
	m.neighborhoods[neighborhood.ID] = neighborhood
	return neighborhood, nil
}

func (m *memoryRepository) GetNeighborhood(_ context.Context, id string) (Neighborhood, error) {
	neighborhood, ok := m.neighborhoods[id]
	if !ok {
		return Neighborhood{}, ErrNeighborhoodNotFound
	}
	return neighborhood, nil
}

func (m *memoryRepository) SearchNeighborhoods(_ context.Context, input SearchNeighborhoodsInput) (SearchNeighborhoodsResult, error) {
	items := make([]Neighborhood, 0, len(m.neighborhoods))
	for _, n := range m.neighborhoods {
		items = append(items, n)
	}
	total := len(items)
	start := input.Offset
	if start > total {
		start = total
	}
	end := start + input.Limit
	if input.Limit <= 0 || end > total {
		end = total
	}
	return SearchNeighborhoodsResult{Items: items[start:end], Total: total}, nil
}

func (m *memoryRepository) AddWatchlistItem(_ context.Context, userID string, neighborhoodID string) (WatchlistItem, error) {
	if _, ok := m.neighborhoods[neighborhoodID]; !ok {
		return WatchlistItem{}, ErrNeighborhoodNotFound
	}
	item := WatchlistItem{
		ID:             "watch_test",
		UserID:         userID,
		NeighborhoodID: neighborhoodID,
		CreatedAt:      time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC),
	}
	return item, nil
}

func (m *memoryRepository) ListWatchlist(_ context.Context, userID string) ([]WatchlistSummary, error) {
	if userID != user.SingleUserID {
		return nil, nil
	}
	return m.watchlist, nil
}

func (m *memoryRepository) LatestMetric(_ context.Context, neighborhoodID string) (MetricSnapshot, error) {
	metric, ok := m.metrics[neighborhoodID]
	if !ok {
		return MetricSnapshot{}, ErrMetricNotFound
	}
	return metric, nil
}

func (m *memoryRepository) ListMetricHistory(_ context.Context, query MetricHistoryRepositoryQuery) ([]MetricHistoryRecord, error) {
	result := make([]MetricHistoryRecord, 0)
	for _, record := range m.history {
		if record.Batch.CollectedAt.Before(query.From) || record.Batch.CollectedAt.After(query.To) {
			continue
		}
		result = append(result, record)
	}
	return result, nil
}

func testFloatPtr(value float64) *float64 {
	return &value
}

func testStringPtr(value string) *string { return &value }

func testTimePtr(value time.Time) *time.Time { return &value }
