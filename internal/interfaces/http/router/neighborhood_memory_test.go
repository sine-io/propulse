package router

import (
	"context"
	"slices"
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
		ID:               id,
		Name:             input.Name,
		City:             memoryStringPtr(input.City),
		Area:             input.Area,
		AvailableLayouts: append([]string(nil), input.AvailableLayouts...),
		CreatedAt:        time.Now().UTC(),
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
		if n.City == nil || len(n.AvailableLayouts) == 0 {
			continue
		}
		if input.Query != "" && !strings.Contains(n.Name, input.Query) {
			continue
		}
		if input.City != "" && *n.City != input.City {
			continue
		}
		if input.Area != "" && n.Area != input.Area {
			continue
		}
		if input.TargetLayout != "" && !slices.Contains(n.AvailableLayouts, input.TargetLayout) {
			continue
		}
		matched = append(matched, n)
	}

	sort.Slice(matched, func(i, j int) bool {
		left := []string{*matched[i].City, matched[i].Area, matched[i].Name, matched[i].ID}
		right := []string{*matched[j].City, matched[j].Area, matched[j].Name, matched[j].ID}
		for index := range left {
			if left[index] != right[index] {
				return left[index] < right[index]
			}
		}
		return false
	})
	total := len(matched)

	start := input.Offset
	if start > total {
		start = total
	}
	end := start + input.Limit
	if input.Limit <= 0 || end > total {
		end = total
	}

	filters := appneighborhood.NeighborhoodSearchFilters{Cities: []string{}, Areas: []appneighborhood.NeighborhoodAreaFilter{}}
	citySeen := map[string]struct{}{}
	areaSeen := map[string]struct{}{}
	for _, n := range r.neighborhoods {
		if n.City == nil || len(n.AvailableLayouts) == 0 {
			continue
		}
		if _, ok := citySeen[*n.City]; !ok {
			citySeen[*n.City] = struct{}{}
			filters.Cities = append(filters.Cities, *n.City)
		}
		key := *n.City + "\x00" + n.Area
		if _, ok := areaSeen[key]; !ok {
			areaSeen[key] = struct{}{}
			filters.Areas = append(filters.Areas, appneighborhood.NeighborhoodAreaFilter{City: *n.City, Area: n.Area})
		}
	}
	sort.Strings(filters.Cities)
	sort.Slice(filters.Areas, func(i, j int) bool {
		if filters.Areas[i].City != filters.Areas[j].City {
			return filters.Areas[i].City < filters.Areas[j].City
		}
		return filters.Areas[i].Area < filters.Areas[j].Area
	})
	return appneighborhood.SearchNeighborhoodsResult{Items: matched[start:end], Total: total, Filters: filters}, nil
}

func (r *inMemoryNeighborhoodRepository) exists(_ context.Context, id string) (bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.neighborhoods[id]
	return ok, nil
}

func (r *inMemoryNeighborhoodRepository) AddWatchlistItem(_ context.Context, userID string, neighborhoodID string, targetLayout string) (appneighborhood.WatchlistItem, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	neighborhood, ok := r.neighborhoods[neighborhoodID]
	if !ok {
		return appneighborhood.WatchlistItem{}, appneighborhood.ErrNeighborhoodNotFound
	}
	if !slices.Contains(neighborhood.AvailableLayouts, targetLayout) {
		return appneighborhood.WatchlistItem{}, appneighborhood.ErrInvalidTargetLayout
	}
	key := userID + ":" + neighborhoodID
	if _, ok := r.watchlist[key]; ok {
		return appneighborhood.WatchlistItem{}, appneighborhood.ErrWatchlistItemExists
	}
	item := appneighborhood.WatchlistItem{
		ID:             uuid.NewString(),
		UserID:         userID,
		NeighborhoodID: neighborhoodID,
		TargetLayout:   targetLayout,
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
			City:           neighborhood.City,
			Area:           neighborhood.Area,
			TargetLayout:   item.TargetLayout,
			HasMetric:      hasMetric,
			Metric:         metric,
		})
	}
	return items, nil
}

func memoryStringPtr(value string) *string { return &value }

func (r *inMemoryNeighborhoodRepository) LatestMetric(_ context.Context, neighborhoodID string) (appneighborhood.MetricSnapshot, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	metric, ok := r.metrics[neighborhoodID]
	if !ok {
		return appneighborhood.MetricSnapshot{}, appneighborhood.ErrMetricNotFound
	}
	return metric, nil
}

func (r *inMemoryNeighborhoodRepository) ListMetricHistory(_ context.Context, query appneighborhood.MetricHistoryRepositoryQuery) ([]appneighborhood.MetricHistoryRecord, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	metric, ok := r.metrics[query.NeighborhoodID]
	if !ok || metric.CollectedAt.Before(query.From) || metric.CollectedAt.After(query.To) {
		return []appneighborhood.MetricHistoryRecord{}, nil
	}
	return []appneighborhood.MetricHistoryRecord{{
		Metric: metric,
		Batch: appneighborhood.CollectionRunReference{
			CollectionRunID: metric.CollectionRunID,
			CollectedAt:     metric.CollectedAt,
			Coverage:        metric.Coverage,
		},
	}}, nil
}
