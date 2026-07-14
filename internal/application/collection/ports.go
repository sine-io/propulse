package collection

import (
	"context"
	"errors"

	appmetric "github.com/sine-io/propulse/internal/application/metric"
)

var ErrInvalidRequest = errors.New("invalid_request")
var ErrNeighborhoodNotFound = errors.New("neighborhood_not_found")
var ErrImportFailed = errors.New("import_failed")

type Repository interface {
	CreateDataSource(context.Context, DataSource) (DataSource, error)
	ListDataSources(context.Context) ([]DataSource, error)
	DataSourceExists(context.Context, string) (bool, error)
	NeighborhoodExists(context.Context, string) (bool, error)
	SaveCollectionRun(context.Context, ImportBatch) (SaveCollectionRunResult, error)
	GetCollectionRun(context.Context, string) (CollectionRunDetail, error)
	ListMetricRefreshCandidates(context.Context, MetricRefreshCandidateFilter) ([]MetricRefreshCandidate, error)
	UpdateMetricStatus(context.Context, string, MetricStatus) error
}

type MetricCalculator interface {
	CalculateCollectionRun(context.Context, appmetric.CalculateCollectionRunCommand) error
}

type MetricRepairEnqueuer interface {
	EnqueueMetricCalculateNeighborhood(ctx context.Context, neighborhoodID string, collectionRunID string, sourceID string) error
}
