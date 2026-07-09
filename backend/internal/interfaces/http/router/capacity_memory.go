package router

import (
	"context"
	"sync"

	appcapacity "github.com/sine-io/propulse/backend/internal/application/capacity"
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

func (r *inMemoryCalculationRepository) Find(_ context.Context, id string) (appcapacity.CalculationRecord, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	record, ok := r.records[id]
	if !ok {
		return appcapacity.CalculationRecord{}, appcapacity.ErrCalculationNotFound
	}
	return record, nil
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
