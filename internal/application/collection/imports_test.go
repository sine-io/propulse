package collection

import (
	"context"
	"errors"
	"testing"
	"time"

	appmetric "github.com/sine-io/propulse/internal/application/metric"
	domainneighborhood "github.com/sine-io/propulse/internal/domain/neighborhood"
)

const (
	testSourceID       = "11111111-1111-1111-1111-111111111111"
	testNeighborhoodID = "22222222-2222-2222-2222-222222222222"
)

func TestCreateDataSourceNormalizesAndPersists(t *testing.T) {
	repo := newFakeRepository()
	now := time.Date(2026, 7, 13, 8, 0, 0, 0, time.UTC)
	service := NewService(repo, func() time.Time { return now }, func() string { return testSourceID })

	source, err := service.CreateDataSource(context.Background(), CreateDataSourceCommand{
		Name: "  链家手工导入  ", SourceType: " manual_json ", City: " 杭州 ", Notes: " 每周导出 ",
	})
	if err != nil {
		t.Fatalf("CreateDataSource() error = %v", err)
	}
	if source.ID != testSourceID || source.Name != "链家手工导入" || source.SourceType != "manual_json" || source.City != "杭州" || source.Notes != "每周导出" {
		t.Fatalf("source = %#v", source)
	}
	if !source.CreatedAt.Equal(now) || !source.UpdatedAt.Equal(now) {
		t.Fatalf("timestamps = %v/%v, want %v", source.CreatedAt, source.UpdatedAt, now)
	}
}

func TestCreateDataSourceRejectsInvalidFieldsWithoutWrite(t *testing.T) {
	repo := newFakeRepository()
	service := NewService(repo, fixedCollectionClock, nil)

	_, err := service.CreateDataSource(context.Background(), CreateDataSourceCommand{SourceType: "INVALID TYPE"})
	var validationErr *ValidationError
	if !errors.As(err, &validationErr) || len(validationErr.Issues) < 3 {
		t.Fatalf("error = %#v, want field validation issues", err)
	}
	if repo.createSourceCalls != 0 {
		t.Fatalf("CreateDataSource calls = %d, want 0", repo.createSourceCalls)
	}
}

func TestImportCollectionRunPersistsNormalizedBatchAndRefreshesExactRun(t *testing.T) {
	repo := newFakeRepository()
	calculator := &fakeMetricCalculator{}
	service := NewServiceWithMetricRefresh(repo, fixedCollectionClock, nil, calculator, nil)
	command := validImportCommand()
	command.CollectedAt = fixedCollectionClock().Add(-24 * time.Hour)

	result, err := service.ImportCollectionRun(context.Background(), command)
	if err != nil {
		t.Fatalf("ImportCollectionRun() error = %v", err)
	}
	if repo.saveCalls != 1 || len(repo.saved.Listings) != 1 || len(repo.saved.Transactions) != 1 {
		t.Fatalf("saved batch = %#v, calls=%d", repo.saved, repo.saveCalls)
	}
	if repo.saved.Run.RawContentType != "application/json" || string(repo.saved.Run.RawPayload) != `{"records":"preserved"}` {
		t.Fatalf("raw traceability = %#v", repo.saved.Run)
	}
	if !repo.saved.Run.CollectedAt.Equal(command.CollectedAt) || !repo.saved.Run.CreatedAt.Equal(fixedCollectionClock()) || !repo.saved.Run.UpdatedAt.Equal(fixedCollectionClock()) {
		t.Fatalf("collection/persistence timestamps = %#v", repo.saved.Run)
	}
	if calculator.calls != 1 || calculator.command.CollectionRunID != repo.saved.Run.ID || calculator.command.NeighborhoodID != testNeighborhoodID {
		t.Fatalf("metric command = %#v, calls=%d", calculator.command, calculator.calls)
	}
	if result.ListingCount != 1 || result.TransactionCount != 1 || result.IdempotentReplay || result.MetricRefreshStatus != MetricStatusCompleted {
		t.Fatalf("result = %#v", result)
	}
}

func TestImportCollectionRunRejectsInvalidRecordWithoutWrite(t *testing.T) {
	repo := newFakeRepository()
	service := NewService(repo, fixedCollectionClock, nil)
	command := validImportCommand()
	command.Records[0].ListingPrice = nil

	_, err := service.ImportCollectionRun(context.Background(), command)
	var validationErr *ValidationError
	if !errors.As(err, &validationErr) || len(validationErr.Issues) == 0 {
		t.Fatalf("error = %#v, want ValidationError", err)
	}
	if repo.saveCalls != 0 || repo.sourceExistsCalls != 0 {
		t.Fatalf("repository calls before validation: source=%d save=%d", repo.sourceExistsCalls, repo.saveCalls)
	}
}

func TestImportCollectionRunReturnsSelectionErrors(t *testing.T) {
	for _, test := range []struct {
		name      string
		configure func(*fakeRepository)
		want      error
	}{
		{name: "source", configure: func(repo *fakeRepository) { repo.sourceExists = false }, want: ErrDataSourceNotFound},
		{name: "neighborhood", configure: func(repo *fakeRepository) { repo.neighborhoodExists = false }, want: ErrNeighborhoodNotFound},
	} {
		t.Run(test.name, func(t *testing.T) {
			repo := newFakeRepository()
			test.configure(repo)
			_, err := NewService(repo, fixedCollectionClock, nil).ImportCollectionRun(context.Background(), validImportCommand())
			if !errors.Is(err, test.want) {
				t.Fatalf("error = %v, want %v", err, test.want)
			}
		})
	}
}

func TestImportCollectionRunKeepsDurableRunAndQueuesRepairWhenRefreshFails(t *testing.T) {
	repo := newFakeRepository()
	calculator := &fakeMetricCalculator{err: errors.New("calculation failed")}
	repair := &fakeMetricRepair{}
	service := NewServiceWithMetricRefresh(repo, fixedCollectionClock, nil, calculator, repair)

	result, err := service.ImportCollectionRun(context.Background(), validImportCommand())
	if err != nil {
		t.Fatalf("ImportCollectionRun() error = %v", err)
	}
	if result.Run.ID == "" || result.MetricRefreshStatus != MetricStatusFailed || repo.updatedStatus != MetricStatusFailed {
		t.Fatalf("result/status = %#v / %q", result, repo.updatedStatus)
	}
	if repair.calls != 1 || repair.collectionRunID != result.Run.ID || repair.sourceID != metricRepairSourceID {
		t.Fatalf("repair = %#v", repair)
	}
}

func TestImportCollectionRunReportsReplayAndRetriesMetric(t *testing.T) {
	repo := newFakeRepository()
	repo.created = false
	calculator := &fakeMetricCalculator{}
	result, err := NewServiceWithMetricRefresh(repo, fixedCollectionClock, nil, calculator, nil).
		ImportCollectionRun(context.Background(), validImportCommand())
	if err != nil {
		t.Fatalf("ImportCollectionRun() error = %v", err)
	}
	if !result.IdempotentReplay || calculator.calls != 1 {
		t.Fatalf("result = %#v, calculator calls=%d", result, calculator.calls)
	}
}

func TestListAndGetCollectionRunsUseRepositoryQueries(t *testing.T) {
	repo := newFakeRepository()
	repo.sources = []DataSource{{ID: testSourceID}}
	repo.detail = CollectionRunDetail{Run: CollectionRun{ID: "33333333-3333-3333-3333-333333333333"}}
	service := NewService(repo, fixedCollectionClock, nil)

	sources, err := service.ListDataSources(context.Background(), ListDataSourcesQuery{})
	if err != nil || len(sources) != 1 {
		t.Fatalf("ListDataSources() = %#v, %v", sources, err)
	}
	detail, err := service.GetCollectionRun(context.Background(), GetCollectionRunQuery{ID: repo.detail.Run.ID})
	if err != nil || detail.Run.ID != repo.detail.Run.ID {
		t.Fatalf("GetCollectionRun() = %#v, %v", detail, err)
	}
}

func TestListMetricRefreshCandidatesNormalizesQuery(t *testing.T) {
	repo := newFakeRepository()
	repo.refreshCandidates = []MetricRefreshCandidate{{CollectionRunID: "run_1", NeighborhoodID: testNeighborhoodID}}
	service := NewService(repo, fixedCollectionClock, nil)

	candidates, err := service.ListMetricRefreshCandidates(context.Background(), ListMetricRefreshCandidatesQuery{})
	if err != nil {
		t.Fatalf("ListMetricRefreshCandidates() error = %v", err)
	}
	if len(candidates) != 1 || repo.refreshLimit != defaultMetricRefreshCandidateLimit || !repo.refreshUpdatedBefore.Equal(fixedCollectionClock()) {
		t.Fatalf("candidates/query = %#v, before=%v limit=%d", candidates, repo.refreshUpdatedBefore, repo.refreshLimit)
	}

	_, err = service.ListMetricRefreshCandidates(context.Background(), ListMetricRefreshCandidatesQuery{Limit: maxMetricRefreshCandidateLimit + 1})
	if !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("oversized query error = %v, want %v", err, ErrInvalidRequest)
	}
}

func validImportCommand() ImportCollectionRunCommand {
	listingPrice := 520.25
	days := 12
	status := ListingStatusActive
	transactionPrice := 505.5
	transactionDate := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	return ImportCollectionRunCommand{
		DataSourceID: testSourceID, NeighborhoodID: testNeighborhoodID, SourceRef: "weekly-2026-07-13",
		CollectedAt: fixedCollectionClock(), Coverage: domainneighborhood.CoverageFull, Format: ImportFormatJSON,
		RawPayload: []byte(`{"records":"preserved"}`), RawContentType: "application/json",
		Records: []ObservationInput{
			{Row: 1, RecordType: RecordTypeListing, SourceRecordID: "listing-1", Layout: "三房", AreaSQM: 89.5, ListingPrice: &listingPrice, DaysOnMarket: &days, Status: &status},
			{Row: 2, RecordType: RecordTypeTransaction, SourceRecordID: "transaction-1", Layout: "三房", AreaSQM: 89.5, TransactionPrice: &transactionPrice, TransactionDate: &transactionDate},
		},
	}
}

func fixedCollectionClock() time.Time {
	return time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC)
}

type fakeRepository struct {
	sourceExists         bool
	neighborhoodExists   bool
	created              bool
	sources              []DataSource
	detail               CollectionRunDetail
	saved                ImportBatch
	updatedStatus        MetricStatus
	refreshCandidates    []MetricRefreshCandidate
	refreshUpdatedBefore time.Time
	refreshLimit         int
	createSourceCalls    int
	sourceExistsCalls    int
	saveCalls            int
}

func newFakeRepository() *fakeRepository {
	return &fakeRepository{sourceExists: true, neighborhoodExists: true, created: true}
}

func (r *fakeRepository) CreateDataSource(_ context.Context, source DataSource) (DataSource, error) {
	r.createSourceCalls++
	return source, nil
}
func (r *fakeRepository) ListDataSources(context.Context) ([]DataSource, error) {
	return r.sources, nil
}
func (r *fakeRepository) DataSourceExists(context.Context, string) (bool, error) {
	r.sourceExistsCalls++
	return r.sourceExists, nil
}
func (r *fakeRepository) NeighborhoodExists(context.Context, string) (bool, error) {
	return r.neighborhoodExists, nil
}
func (r *fakeRepository) SaveCollectionRun(_ context.Context, batch ImportBatch) (SaveCollectionRunResult, error) {
	r.saveCalls++
	r.saved = batch
	return SaveCollectionRunResult{Run: batch.Run, Created: r.created}, nil
}
func (r *fakeRepository) GetCollectionRun(context.Context, string) (CollectionRunDetail, error) {
	return r.detail, nil
}
func (r *fakeRepository) ListMetricRefreshCandidates(_ context.Context, updatedBefore time.Time, limit int) ([]MetricRefreshCandidate, error) {
	r.refreshUpdatedBefore = updatedBefore
	r.refreshLimit = limit
	return r.refreshCandidates, nil
}
func (r *fakeRepository) UpdateMetricStatus(_ context.Context, _ string, status MetricStatus) error {
	r.updatedStatus = status
	return nil
}

type fakeMetricCalculator struct {
	command appmetric.CalculateCollectionRunCommand
	calls   int
	err     error
}

func (c *fakeMetricCalculator) CalculateCollectionRun(_ context.Context, command appmetric.CalculateCollectionRunCommand) error {
	c.calls++
	c.command = command
	return c.err
}

type fakeMetricRepair struct {
	neighborhoodID  string
	collectionRunID string
	sourceID        string
	calls           int
}

func (r *fakeMetricRepair) EnqueueMetricCalculateNeighborhood(_ context.Context, neighborhoodID, collectionRunID, sourceID string) error {
	r.calls++
	r.neighborhoodID = neighborhoodID
	r.collectionRunID = collectionRunID
	r.sourceID = sourceID
	return nil
}
