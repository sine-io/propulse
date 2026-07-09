package router

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	appneighborhood "github.com/propulse/propulse/backend/internal/application/neighborhood"
)

type inMemoryNeighborhoodRepository struct {
	mu            sync.RWMutex
	neighborhoods map[string]appneighborhood.Neighborhood
	metrics       map[string]appneighborhood.MetricSnapshot
	watchlist     map[string]appneighborhood.WatchlistItem
}

func newInMemoryNeighborhoodRepository() *inMemoryNeighborhoodRepository {
	return &inMemoryNeighborhoodRepository{
		neighborhoods: map[string]appneighborhood.Neighborhood{},
		metrics:       map[string]appneighborhood.MetricSnapshot{},
		watchlist:     map[string]appneighborhood.WatchlistItem{},
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
	return item, nil
}

func (r *inMemoryNeighborhoodRepository) ListWatchlist(_ context.Context, userID string) ([]appneighborhood.WatchlistSummary, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	items := []appneighborhood.WatchlistSummary{}
	for _, item := range r.watchlist {
		if item.UserID != userID {
			continue
		}
		neighborhood := r.neighborhoods[item.NeighborhoodID]
		items = append(items, appneighborhood.WatchlistSummary{
			ID:             item.ID,
			NeighborhoodID: item.NeighborhoodID,
			Name:           neighborhood.Name,
			Area:           neighborhood.Area,
			TargetLayout:   neighborhood.TargetLayout,
			Metric:         r.metrics[item.NeighborhoodID],
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
