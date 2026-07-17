package collection

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	appmetric "github.com/sine-io/propulse/internal/application/metric"
)

const metricRepairSourceID = "import.retry"
const defaultMetricRefreshCandidateLimit = 100
const maxMetricRefreshCandidateLimit = 500
const defaultCollectionRunPageSize = 20
const maxCollectionRunPageSize = 100

var sourceTypePattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)

type Service struct {
	repo             Repository
	metricCalculator MetricCalculator
	metricRepair     MetricRepairEnqueuer
	algorithmVersion string
	now              func() time.Time
	newID            func() string
}

func NewService(repo Repository, now func() time.Time, newID func() string, algorithmVersion string) *Service {
	if now == nil {
		now = time.Now
	}
	if newID == nil {
		newID = uuid.NewString
	}
	return &Service{repo: repo, now: now, newID: newID, algorithmVersion: strings.TrimSpace(algorithmVersion)}
}

func NewServiceWithMetricRefresh(
	repo Repository,
	now func() time.Time,
	newID func() string,
	calculator MetricCalculator,
	repair MetricRepairEnqueuer,
	algorithmVersion string,
) *Service {
	service := NewService(repo, now, newID, algorithmVersion)
	service.metricCalculator = calculator
	service.metricRepair = repair
	return service
}

func (s *Service) CreateDataSource(ctx context.Context, command CreateDataSourceCommand) (DataSource, error) {
	command.Name = strings.TrimSpace(command.Name)
	command.SourceType = strings.TrimSpace(command.SourceType)
	command.City = strings.TrimSpace(command.City)
	command.Notes = strings.TrimSpace(command.Notes)

	issues := validateDataSource(command)
	if len(issues) > 0 {
		return DataSource{}, &ValidationError{Issues: issues}
	}

	now := s.now().UTC()
	return s.repo.CreateDataSource(ctx, DataSource{
		ID:         s.newID(),
		Name:       command.Name,
		SourceType: command.SourceType,
		City:       command.City,
		Notes:      command.Notes,
		CreatedAt:  now,
		UpdatedAt:  now,
	})
}

type ListDataSourcesQuery struct{}

func (s *Service) ListDataSources(ctx context.Context, _ ListDataSourcesQuery) ([]DataSource, error) {
	return s.repo.ListDataSources(ctx)
}

func (s *Service) GetCollectionRun(ctx context.Context, query GetCollectionRunQuery) (CollectionRunDetail, error) {
	if _, err := uuid.Parse(strings.TrimSpace(query.ID)); err != nil {
		return CollectionRunDetail{}, ErrInvalidRequest
	}
	return s.repo.GetCollectionRun(ctx, strings.TrimSpace(query.ID))
}

func (s *Service) ListCollectionRuns(ctx context.Context, query ListCollectionRunsQuery) (CollectionRunsPage, error) {
	filter := CollectionRunFilter{
		DataSourceID: strings.TrimSpace(query.DataSourceID), NeighborhoodID: strings.TrimSpace(query.NeighborhoodID),
		Status: query.Status, MetricStatus: query.MetricStatus, From: query.From, To: query.To,
		Page: query.Page, PageSize: query.PageSize,
	}
	if filter.Page == 0 {
		filter.Page = 1
	}
	if filter.PageSize == 0 {
		filter.PageSize = defaultCollectionRunPageSize
	}
	if filter.Page < 1 || filter.PageSize < 1 || filter.PageSize > maxCollectionRunPageSize {
		return CollectionRunsPage{}, ErrInvalidRequest
	}
	for _, id := range []string{filter.DataSourceID, filter.NeighborhoodID} {
		if id != "" {
			if _, err := uuid.Parse(id); err != nil {
				return CollectionRunsPage{}, ErrInvalidRequest
			}
		}
	}
	if filter.Status != "" && filter.Status != CollectionRunStatusCompleted {
		return CollectionRunsPage{}, ErrInvalidRequest
	}
	if filter.MetricStatus != "" && filter.MetricStatus != MetricStatusPending && filter.MetricStatus != MetricStatusCompleted && filter.MetricStatus != MetricStatusFailed {
		return CollectionRunsPage{}, ErrInvalidRequest
	}
	if filter.From != nil {
		from := filter.From.UTC()
		filter.From = &from
	}
	if filter.To != nil {
		to := filter.To.UTC()
		filter.To = &to
	}
	if filter.From != nil && filter.To != nil && filter.To.Before(*filter.From) {
		return CollectionRunsPage{}, ErrInvalidRequest
	}
	return s.repo.ListCollectionRuns(ctx, filter)
}

func (s *Service) ListMetricRefreshCandidates(ctx context.Context, query ListMetricRefreshCandidatesQuery) ([]MetricRefreshCandidate, error) {
	updatedBefore := query.UpdatedBefore.UTC()
	if query.UpdatedBefore.IsZero() {
		updatedBefore = s.now().UTC()
	}
	limit := query.Limit
	if limit == 0 {
		limit = defaultMetricRefreshCandidateLimit
	}
	if limit < 0 || limit > maxMetricRefreshCandidateLimit {
		return nil, ErrInvalidRequest
	}
	if s.algorithmVersion == "" {
		return nil, ErrInvalidRequest
	}
	return s.repo.ListMetricRefreshCandidates(ctx, MetricRefreshCandidateFilter{
		AlgorithmVersion: s.algorithmVersion,
		UpdatedBefore:    updatedBefore,
		Limit:            limit,
	})
}

func (s *Service) ImportCollectionRun(ctx context.Context, command ImportCollectionRunCommand) (ImportCollectionRunResult, error) {
	now := s.now().UTC()
	normalized, issues := validateAndNormalize(command, now)
	if len(issues) > 0 {
		return ImportCollectionRunResult{}, &ValidationError{Issues: issues}
	}

	exists, err := s.repo.DataSourceExists(ctx, normalized.DataSourceID)
	if err != nil {
		return ImportCollectionRunResult{}, fmt.Errorf("%w: %w", ErrImportFailed, err)
	}
	if !exists {
		return ImportCollectionRunResult{}, ErrDataSourceNotFound
	}

	exists, err = s.repo.NeighborhoodExists(ctx, normalized.NeighborhoodID)
	if err != nil {
		return ImportCollectionRunResult{}, fmt.Errorf("%w: %w", ErrImportFailed, err)
	}
	if !exists {
		return ImportCollectionRunResult{}, ErrNeighborhoodNotFound
	}

	saved, err := s.repo.SaveCollectionRun(ctx, normalized.NewBatch(s.newID(), now, s.newID))
	if err != nil {
		return ImportCollectionRunResult{}, fmt.Errorf("%w: %w", ErrImportFailed, err)
	}

	response := ImportCollectionRunResult{
		Run:                 saved.Run,
		ListingCount:        len(normalized.Listings),
		TransactionCount:    len(normalized.Transactions),
		IdempotentReplay:    !saved.Created,
		MetricRefreshStatus: saved.Run.MetricStatus,
	}
	if response.MetricRefreshStatus == "" {
		response.MetricRefreshStatus = MetricStatusPending
	}
	if s.metricCalculator == nil {
		return response, nil
	}

	err = s.metricCalculator.CalculateCollectionRun(ctx, appmetric.CalculateCollectionRunCommand{
		NeighborhoodID:  normalized.NeighborhoodID,
		CollectionRunID: saved.Run.ID,
	})
	if err == nil {
		response.MetricRefreshStatus = MetricStatusCompleted
		response.Run.MetricStatus = MetricStatusCompleted
		return response, nil
	}

	refreshStatus := MetricStatusFailed
	if updateErr := s.repo.UpdateMetricStatus(ctx, saved.Run.ID, MetricStatusFailed); updateErr != nil {
		refreshStatus = MetricStatusPending
	}
	if s.metricRepair != nil {
		_ = s.metricRepair.EnqueueMetricCalculateNeighborhood(ctx, normalized.NeighborhoodID, saved.Run.ID, metricRepairSourceID)
	}
	response.MetricRefreshStatus = refreshStatus
	response.Run.MetricStatus = refreshStatus
	return response, nil
}

func validateDataSource(command CreateDataSourceCommand) []ValidationIssue {
	issues := make([]ValidationIssue, 0, 4)
	if length := utf8.RuneCountInString(command.Name); length < 1 || length > 128 {
		issues = appendIssue(issues, nil, "name", "invalid_length", "name must contain 1 to 128 characters")
	}
	if !sourceTypePattern.MatchString(command.SourceType) {
		issues = appendIssue(issues, nil, "sourceType", "invalid", "sourceType must be a lowercase slug")
	}
	if length := utf8.RuneCountInString(command.City); length < 1 || length > 128 {
		issues = appendIssue(issues, nil, "city", "invalid_length", "city must contain 1 to 128 characters")
	}
	if utf8.RuneCountInString(command.Notes) > 2048 {
		issues = appendIssue(issues, nil, "notes", "too_long", "notes must be at most 2048 characters")
	}
	return issues
}
