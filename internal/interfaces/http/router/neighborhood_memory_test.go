package router

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	appneighborhood "github.com/sine-io/propulse/internal/application/neighborhood"
)

type inMemoryNeighborhoodRepository struct {
	mu            sync.RWMutex
	neighborhoods map[string]appneighborhood.Neighborhood
	metrics       map[string]appneighborhood.MetricSnapshot
	watchlist     map[string]appneighborhood.WatchlistItem
	watchlistKeys []string
	marketState   *inMemoryMarketState
}

func newInMemoryNeighborhoodRepository(marketState ...*inMemoryMarketState) *inMemoryNeighborhoodRepository {
	var state *inMemoryMarketState
	if len(marketState) > 0 {
		state = marketState[0]
	}
	if state == nil {
		state = newInMemoryMarketState()
	}
	return &inMemoryNeighborhoodRepository{
		neighborhoods: map[string]appneighborhood.Neighborhood{},
		metrics:       map[string]appneighborhood.MetricSnapshot{},
		watchlist:     map[string]appneighborhood.WatchlistItem{},
		watchlistKeys: []string{},
		marketState:   state,
	}
}

func (r *inMemoryNeighborhoodRepository) CreateNeighborhood(_ context.Context, input appneighborhood.CreateNeighborhoodInput) (appneighborhood.Neighborhood, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	id := input.ID
	if id == "" {
		id = uuid.NewString()
	}
	neighborhood := appneighborhood.Neighborhood{
		ID:           id,
		Name:         input.Name,
		Area:         input.Area,
		TargetLayout: input.TargetLayout,
		CreatedAt:    time.Now().UTC(),
	}
	r.neighborhoods[id] = neighborhood
	return neighborhood, nil
}

func (r *inMemoryNeighborhoodRepository) GetNeighborhood(_ context.Context, id string) (appneighborhood.Neighborhood, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	neighborhood, ok := r.neighborhoods[id]
	if !ok {
		return appneighborhood.Neighborhood{}, appneighborhood.ErrNeighborhoodNotFound
	}
	return neighborhood, nil
}

func (r *inMemoryNeighborhoodRepository) SearchNeighborhoods(_ context.Context, input appneighborhood.SearchNeighborhoodsInput) (appneighborhood.SearchNeighborhoodsResult, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	matched := make([]appneighborhood.Neighborhood, 0, len(r.neighborhoods))
	for _, n := range r.neighborhoods {
		if input.Query != "" &&
			!strings.Contains(n.Name, input.Query) && !strings.Contains(n.Area, input.Query) {
			continue
		}
		if input.Area != "" && n.Area != input.Area {
			continue
		}
		if input.TargetLayout != "" && n.TargetLayout != input.TargetLayout {
			continue
		}
		matched = append(matched, n)
	}

	sort.Slice(matched, func(i, j int) bool { return matched[i].Name < matched[j].Name })
	total := len(matched)

	start := input.Offset
	if start > total {
		start = total
	}
	end := start + input.Limit
	if input.Limit <= 0 || end > total {
		end = total
	}

	return appneighborhood.SearchNeighborhoodsResult{Items: matched[start:end], Total: total}, nil
}

func (r *inMemoryNeighborhoodRepository) exists(_ context.Context, id string) (bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.neighborhoods[id]
	return ok, nil
}

func (r *inMemoryNeighborhoodRepository) AddWatchlistItem(_ context.Context, userID string, neighborhoodID string) (appneighborhood.WatchlistItem, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.neighborhoods[neighborhoodID]; !ok {
		return appneighborhood.WatchlistItem{}, appneighborhood.ErrNeighborhoodNotFound
	}
	key := userID + ":" + neighborhoodID
	if item, ok := r.watchlist[key]; ok {
		return item, nil
	}
	item := appneighborhood.WatchlistItem{
		ID:             uuid.NewString(),
		UserID:         userID,
		NeighborhoodID: neighborhoodID,
		CreatedAt:      time.Now().UTC(),
	}
	r.watchlist[key] = item
	r.watchlistKeys = append(r.watchlistKeys, key)
	return item, nil
}

func (r *inMemoryNeighborhoodRepository) ListWatchlist(_ context.Context, userID string) ([]appneighborhood.WatchlistSummary, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	items := []appneighborhood.WatchlistSummary{}
	for _, key := range r.watchlistKeys {
		item := r.watchlist[key]
		if item.UserID != userID {
			continue
		}
		neighborhood := r.neighborhoods[item.NeighborhoodID]
		metric, hasMetric := r.metrics[item.NeighborhoodID]
		items = append(items, appneighborhood.WatchlistSummary{
			ID:             item.ID,
			NeighborhoodID: item.NeighborhoodID,
			Name:           neighborhood.Name,
			Area:           neighborhood.Area,
			TargetLayout:   neighborhood.TargetLayout,
			HasMetric:      hasMetric,
			Metric:         metric,
		})
	}
	return items, nil
}

func (r *inMemoryNeighborhoodRepository) LatestMetric(_ context.Context, neighborhoodID string) (appneighborhood.MetricSnapshot, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	metric, ok := r.metrics[neighborhoodID]
	if !ok {
		return appneighborhood.MetricSnapshot{}, appneighborhood.ErrMetricNotFound
	}
	return metric, nil
}
