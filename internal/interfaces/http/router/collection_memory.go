package router

import (
	"context"
	"sync"

	appcollection "github.com/sine-io/propulse/internal/application/collection"
)

type inMemoryCollectionRepository struct {
	mu            sync.RWMutex
	neighborhoods map[string]bool
	rawRecords    map[string]appcollection.RawCollectionRecord
	snapshots     map[string]appcollection.ListingSnapshot
}

func newInMemoryCollectionRepository() *inMemoryCollectionRepository {
	return &inMemoryCollectionRepository{
		neighborhoods: map[string]bool{},
		rawRecords:    map[string]appcollection.RawCollectionRecord{},
		snapshots:     map[string]appcollection.ListingSnapshot{},
	}
}

func (r *inMemoryCollectionRepository) NeighborhoodExists(_ context.Context, id string) (bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.neighborhoods[id], nil
}

func (r *inMemoryCollectionRepository) SaveImport(_ context.Context, raw appcollection.RawCollectionRecord, snapshots []appcollection.ListingSnapshot) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.rawRecords[raw.ID] = raw
	for _, snapshot := range snapshots {
		r.snapshots[snapshot.ID] = snapshot
	}
	return nil
}
