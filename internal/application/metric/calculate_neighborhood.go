package metric

import (
	"context"
	"errors"
	"strings"
	"time"

	domainneighborhood "github.com/sine-io/propulse/internal/domain/neighborhood"
)

type Service struct {
	repo             Repository
	algorithmVersion string
	now              func() time.Time
}

func NewService(repo Repository, algorithmVersion string) *Service {
	return NewServiceWithClock(repo, algorithmVersion, time.Now)
}

func NewServiceWithClock(repo Repository, algorithmVersion string, now func() time.Time) *Service {
	return &Service{repo: repo, algorithmVersion: strings.TrimSpace(algorithmVersion), now: now}
}

func (s *Service) CalculateCollectionRun(ctx context.Context, command CalculateCollectionRunCommand) error {
	if s.algorithmVersion == "" {
		return ErrInvalidAlgorithmVersion
	}
	if _, err := s.repo.GetNeighborhood(ctx, command.NeighborhoodID); err != nil {
		return err
	}

	run, err := s.resolveCollectionRun(ctx, command)
	if err != nil {
		return err
	}
	if run.NeighborhoodID != command.NeighborhoodID {
		return ErrCollectionRunNeighborhoodMismatch
	}

	aggregate, err := s.repo.AggregateMarketObservations(ctx, AggregateMarketParams{
		NeighborhoodID: run.NeighborhoodID,
		TriggerRunID:   run.ID,
	})
	if err != nil {
		return err
	}
	evidence := domainneighborhood.NewTransactionMomentumEvidence(
		run.CollectedAt,
		aggregate.LastThirtyDayTransactionCount,
		aggregate.PrecedingSixtyDayTransactionCount,
	)
	if aggregate.TransactionSampleCount != evidence.SampleCount {
		return ErrInconsistentTransactionEvidence
	}

	quality := domainneighborhood.AssessQuality(domainneighborhood.QualityInput{
		Now:                    s.now(),
		InventoryCollectedAt:   aggregate.InventoryCollectedAt,
		LatestCoverage:         aggregate.Coverage,
		HasFullInventory:       aggregate.InventoryCollectionRunID != nil,
		ListingSampleCount:     aggregate.ListingSampleCount,
		TransactionSampleCount: evidence.SampleCount,
	})

	snapshot := MetricSnapshot{
		NeighborhoodID:             run.NeighborhoodID,
		CollectionRunID:            run.ID,
		AlgorithmVersion:           s.algorithmVersion,
		InventoryCollectionRunID:   aggregate.InventoryCollectionRunID,
		SourceIDs:                  aggregate.SourceIDs,
		LatestObservedAt:           aggregate.LatestObservedAt,
		ListedHomes:                aggregate.ListedHomes,
		PriceCutHomes:              aggregate.PriceCutHomes,
		AvgDaysOnMarket:            aggregate.AvgDaysOnMarket,
		ListingPriceMin:            aggregate.ListingPriceMin,
		ListingPriceMax:            aggregate.ListingPriceMax,
		TransactionPriceMin:        aggregate.TransactionPriceMin,
		TransactionPriceMax:        aggregate.TransactionPriceMax,
		TransactionMomentum:        domainneighborhood.CalculateTransactionMomentum(evidence),
		TransactionEvidence:        &evidence,
		TargetLayoutSupplyByLayout: cloneLayoutSupply(aggregate.TargetLayoutSupplyByLayout),
		ListingSampleCount:         aggregate.ListingSampleCount,
		TransactionSampleCount:     evidence.SampleCount,
		Coverage:                   quality.Coverage,
		Freshness:                  quality.Freshness,
		InventoryCollectedAt:       aggregate.InventoryCollectedAt,
		ListedHomesChangePct:       aggregate.ListedHomesChangePct,
		QualityWarnings:            quality.Warnings,
		QualityState:               quality.State,
	}
	if _, err := s.repo.UpsertNeighborhoodMetric(ctx, snapshot); err != nil {
		return err
	}
	return s.repo.MarkCollectionRunMetricCompleted(ctx, run.ID)
}

func cloneLayoutSupply(source map[string]int) map[string]int {
	result := make(map[string]int, len(source))
	for layout, supply := range source {
		result[layout] = supply
	}
	return result
}

func (s *Service) resolveCollectionRun(ctx context.Context, command CalculateCollectionRunCommand) (CompletedCollectionRun, error) {
	var (
		run CompletedCollectionRun
		err error
	)
	if command.CollectionRunID == "" {
		run, err = s.repo.LatestCompletedCollectionRun(ctx, command.NeighborhoodID)
	} else {
		run, err = s.repo.GetCompletedCollectionRun(ctx, command.CollectionRunID)
	}
	if errors.Is(err, ErrCollectionRunNotFound) {
		return CompletedCollectionRun{}, ErrCollectionRunNotFound
	}
	return run, err
}
