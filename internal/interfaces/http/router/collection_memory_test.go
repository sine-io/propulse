package router

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
	appcollection "github.com/sine-io/propulse/internal/application/collection"
)

var _ appcollection.Repository = (*inMemoryCollectionRepository)(nil)

type inMemoryMarketState struct {
	mu                      sync.RWMutex
	sources                 map[string]appcollection.DataSource
	sourceIdentities        map[string]string
	collectionRuns          map[string]appcollection.CollectionRun
	collectionRunIdentities map[string]string
	runDetails              map[string]appcollection.CollectionRunDetail
	latestRunByNeighborhood map[string]string
}

func newInMemoryMarketState() *inMemoryMarketState {
	return &inMemoryMarketState{
		sources:                 map[string]appcollection.DataSource{},
		sourceIdentities:        map[string]string{},
		collectionRuns:          map[string]appcollection.CollectionRun{},
		collectionRunIdentities: map[string]string{},
		runDetails:              map[string]appcollection.CollectionRunDetail{},
		latestRunByNeighborhood: map[string]string{},
	}
}

type inMemoryCollectionRepository struct {
	neighborhoods *inMemoryNeighborhoodRepository
	marketState   *inMemoryMarketState
}

func newInMemoryCollectionRepository(neighborhoods *inMemoryNeighborhoodRepository, marketState *inMemoryMarketState) *inMemoryCollectionRepository {
	if marketState == nil {
		marketState = newInMemoryMarketState()
	}
	if neighborhoods == nil {
		neighborhoods = newInMemoryNeighborhoodRepository(marketState)
	}
	return &inMemoryCollectionRepository{
		neighborhoods: neighborhoods,
		marketState:   marketState,
	}
}

func (r *inMemoryCollectionRepository) NeighborhoodExists(ctx context.Context, id string) (bool, error) {
	return r.neighborhoods.exists(ctx, id)
}

func (r *inMemoryCollectionRepository) CreateDataSource(_ context.Context, source appcollection.DataSource) (appcollection.DataSource, error) {
	r.marketState.mu.Lock()
	defer r.marketState.mu.Unlock()

	key := dataSourceIdentity(source.Name, source.City)
	if existingID, ok := r.marketState.sourceIdentities[key]; ok {
		return copyDataSource(r.marketState.sources[existingID]), nil
	}
	if source.ID == "" {
		source.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	if source.CreatedAt.IsZero() {
		source.CreatedAt = now
	}
	if source.UpdatedAt.IsZero() {
		source.UpdatedAt = source.CreatedAt
	}
	r.marketState.sources[source.ID] = copyDataSource(source)
	r.marketState.sourceIdentities[key] = source.ID
	return copyDataSource(source), nil
}

func (r *inMemoryCollectionRepository) ListDataSources(_ context.Context) ([]appcollection.DataSource, error) {
	r.marketState.mu.RLock()
	defer r.marketState.mu.RUnlock()

	sources := make([]appcollection.DataSource, 0, len(r.marketState.sources))
	for _, source := range r.marketState.sources {
		sources = append(sources, copyDataSource(source))
	}
	sort.Slice(sources, func(i, j int) bool {
		if sources[i].City != sources[j].City {
			return sources[i].City < sources[j].City
		}
		if sources[i].Name != sources[j].Name {
			return sources[i].Name < sources[j].Name
		}
		return sources[i].ID < sources[j].ID
	})
	return sources, nil
}

func (r *inMemoryCollectionRepository) DataSourceExists(_ context.Context, id string) (bool, error) {
	r.marketState.mu.RLock()
	defer r.marketState.mu.RUnlock()
	_, ok := r.marketState.sources[id]
	return ok, nil
}

func (r *inMemoryCollectionRepository) SaveCollectionRun(ctx context.Context, batch appcollection.ImportBatch) (appcollection.SaveCollectionRunResult, error) {
	neighborhoodExists, err := r.NeighborhoodExists(ctx, batch.Run.NeighborhoodID)
	if err != nil {
		return appcollection.SaveCollectionRunResult{}, err
	}
	if !neighborhoodExists {
		return appcollection.SaveCollectionRunResult{}, appcollection.ErrNeighborhoodNotFound
	}

	r.marketState.mu.Lock()
	defer r.marketState.mu.Unlock()

	source, sourceExists := r.marketState.sources[batch.Run.DataSourceID]
	if !sourceExists {
		return appcollection.SaveCollectionRunResult{}, appcollection.ErrDataSourceNotFound
	}

	key := collectionRunIdentity(batch.Run.DataSourceID, batch.Run.SourceRef, batch.Run.ContentChecksum)
	if existingID, ok := r.marketState.collectionRunIdentities[key]; ok {
		return appcollection.SaveCollectionRunResult{
			Run:     copyCollectionRun(r.marketState.collectionRuns[existingID]),
			Created: false,
		}, nil
	}

	run := copyCollectionRun(batch.Run)
	listings := copyListingObservationsForRun(batch.Run, batch.Listings)
	transactions := copyTransactionObservationsForRun(batch.Run, batch.Transactions)
	detail := appcollection.CollectionRunDetail{
		Run:          copyCollectionRun(run),
		Source:       copyDataSource(source),
		Listings:     copyListingObservations(listings),
		Transactions: copyTransactionObservations(transactions),
	}

	r.marketState.collectionRuns[run.ID] = copyCollectionRun(run)
	r.marketState.collectionRunIdentities[key] = run.ID
	r.marketState.runDetails[run.ID] = detail
	latestID, hasLatest := r.marketState.latestRunByNeighborhood[run.NeighborhoodID]
	if !hasLatest || r.marketState.collectionRuns[latestID].CollectedAt.Before(run.CollectedAt) || (r.marketState.collectionRuns[latestID].CollectedAt.Equal(run.CollectedAt) && latestID < run.ID) {
		r.marketState.latestRunByNeighborhood[run.NeighborhoodID] = run.ID
	}

	return appcollection.SaveCollectionRunResult{Run: copyCollectionRun(run), Created: true}, nil
}

func (r *inMemoryCollectionRepository) GetCollectionRun(_ context.Context, id string) (appcollection.CollectionRunDetail, error) {
	r.marketState.mu.RLock()
	defer r.marketState.mu.RUnlock()

	detail, ok := r.marketState.runDetails[id]
	if !ok {
		return appcollection.CollectionRunDetail{}, appcollection.ErrCollectionRunNotFound
	}
	return copyCollectionRunDetail(detail), nil
}

func (r *inMemoryCollectionRepository) ListMetricRefreshCandidates(_ context.Context, updatedBefore time.Time, limit int) ([]appcollection.MetricRefreshCandidate, error) {
	r.marketState.mu.RLock()
	defer r.marketState.mu.RUnlock()

	runs := make([]appcollection.CollectionRun, 0)
	for _, run := range r.marketState.collectionRuns {
		if (run.MetricStatus == appcollection.MetricStatusPending || run.MetricStatus == appcollection.MetricStatusFailed) && !run.UpdatedAt.After(updatedBefore) {
			runs = append(runs, run)
		}
	}
	sort.Slice(runs, func(i, j int) bool {
		if !runs[i].UpdatedAt.Equal(runs[j].UpdatedAt) {
			return runs[i].UpdatedAt.Before(runs[j].UpdatedAt)
		}
		return runs[i].ID < runs[j].ID
	})
	if len(runs) > limit {
		runs = runs[:limit]
	}
	candidates := make([]appcollection.MetricRefreshCandidate, 0, len(runs))
	for _, run := range runs {
		candidates = append(candidates, appcollection.MetricRefreshCandidate{
			CollectionRunID: run.ID,
			NeighborhoodID:  run.NeighborhoodID,
		})
	}
	return candidates, nil
}

func (r *inMemoryCollectionRepository) UpdateMetricStatus(_ context.Context, id string, status appcollection.MetricStatus) error {
	r.marketState.mu.Lock()
	defer r.marketState.mu.Unlock()

	run, ok := r.marketState.collectionRuns[id]
	if !ok {
		return appcollection.ErrCollectionRunNotFound
	}
	run.MetricStatus = status
	run.UpdatedAt = time.Now().UTC()
	r.marketState.collectionRuns[id] = run
	detail := r.marketState.runDetails[id]
	detail.Run = copyCollectionRun(run)
	r.marketState.runDetails[id] = detail
	return nil
}

func dataSourceIdentity(name string, city string) string {
	return name + "\x00" + city
}

func collectionRunIdentity(dataSourceID string, sourceRef string, checksum string) string {
	return dataSourceID + "\x00" + sourceRef + "\x00" + checksum
}

func copyDataSource(source appcollection.DataSource) appcollection.DataSource {
	return source
}

func copyCollectionRun(run appcollection.CollectionRun) appcollection.CollectionRun {
	run.RawPayload = append([]byte(nil), run.RawPayload...)
	run.ValidationSummary.Issues = append([]appcollection.ValidationIssue(nil), run.ValidationSummary.Issues...)
	return run
}

func copyListingObservationsForRun(run appcollection.CollectionRun, observations []appcollection.ListingObservation) []appcollection.ListingObservation {
	copied := copyListingObservations(observations)
	for i := range copied {
		copied[i].CollectionRunID = run.ID
		copied[i].NeighborhoodID = run.NeighborhoodID
	}
	sort.Slice(copied, func(i, j int) bool {
		if copied[i].SourceRow != copied[j].SourceRow {
			return copied[i].SourceRow < copied[j].SourceRow
		}
		return copied[i].ID < copied[j].ID
	})
	return copied
}

func copyTransactionObservationsForRun(run appcollection.CollectionRun, observations []appcollection.TransactionObservation) []appcollection.TransactionObservation {
	copied := copyTransactionObservations(observations)
	for i := range copied {
		copied[i].CollectionRunID = run.ID
		copied[i].NeighborhoodID = run.NeighborhoodID
	}
	sort.Slice(copied, func(i, j int) bool {
		if copied[i].SourceRow != copied[j].SourceRow {
			return copied[i].SourceRow < copied[j].SourceRow
		}
		return copied[i].ID < copied[j].ID
	})
	return copied
}

func copyListingObservations(observations []appcollection.ListingObservation) []appcollection.ListingObservation {
	copied := make([]appcollection.ListingObservation, 0, len(observations))
	for _, observation := range observations {
		observation.Attributes = copyStringMap(observation.Attributes)
		copied = append(copied, observation)
	}
	return copied
}

func copyTransactionObservations(observations []appcollection.TransactionObservation) []appcollection.TransactionObservation {
	copied := make([]appcollection.TransactionObservation, 0, len(observations))
	for _, observation := range observations {
		if observation.OriginalListingRef != nil {
			originalListingRef := *observation.OriginalListingRef
			observation.OriginalListingRef = &originalListingRef
		}
		copied = append(copied, observation)
	}
	return copied
}

func copyCollectionRunDetail(detail appcollection.CollectionRunDetail) appcollection.CollectionRunDetail {
	return appcollection.CollectionRunDetail{
		Run:          copyCollectionRun(detail.Run),
		Source:       copyDataSource(detail.Source),
		Listings:     copyListingObservations(detail.Listings),
		Transactions: copyTransactionObservations(detail.Transactions),
	}
}

func copyStringMap(values map[string]string) map[string]string {
	if values == nil {
		return map[string]string{}
	}
	copied := make(map[string]string, len(values))
	for key, value := range values {
		copied[key] = value
	}
	return copied
}
