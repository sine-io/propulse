package router

import (
	"context"
	"sync"

	appcapacity "github.com/sine-io/propulse/internal/application/capacity"
)

type inMemoryCalculationRepository struct {
	mu      sync.RWMutex
	records map[string]appcapacity.CalculationRecord
}

func newInMemoryCalculationRepository() *inMemoryCalculationRepository {
	return &inMemoryCalculationRepository{
		records: map[string]appcapacity.CalculationRecord{},
	}
}

func (r *inMemoryCalculationRepository) Save(_ context.Context, record appcapacity.CalculationRecord) (appcapacity.CalculationRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.records[record.ID] = record
	return record, nil
}

func (r *inMemoryCalculationRepository) FindByUser(_ context.Context, userID, id string) (appcapacity.CalculationRecord, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	record, ok := r.records[id]
	if !ok || record.UserID != userID {
		return appcapacity.CalculationRecord{}, appcapacity.ErrCalculationNotFound
	}
	return record, nil
}

func (r *inMemoryCalculationRepository) ListByUser(_ context.Context, filter appcapacity.CalculationListFilter) (appcapacity.CalculationHistoryPage, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	items := make([]appcapacity.CalculationSummary, 0)
	for _, record := range r.records {
		if record.UserID != filter.UserID {
			continue
		}
		items = append(items, appcapacity.CalculationSummary{
			ID: record.ID, CreatedAt: record.CreatedAt, PressureLevel: record.Result.PressureLevel,
			TargetTotalPrice: record.Input.TargetTotalPrice,
		})
	}
	return appcapacity.CalculationHistoryPage{Items: items, Total: int64(len(items)), Page: filter.Page, PageSize: filter.PageSize}, nil
}

func (r *inMemoryCalculationRepository) FindLatestByUser(_ context.Context, userID string) (appcapacity.CalculationRecord, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var latest appcapacity.CalculationRecord
	for _, record := range r.records {
		if record.UserID != userID {
			continue
		}
		if latest.ID == "" || record.CreatedAt.After(latest.CreatedAt) {
			latest = record
		}
	}
	if latest.ID == "" {
		return appcapacity.CalculationRecord{}, appcapacity.ErrCalculationNotFound
	}
	return latest, nil
}
