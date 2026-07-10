package metric

import (
	"context"
	"errors"
	"time"

	domainneighborhood "github.com/sine-io/propulse/internal/domain/neighborhood"
)

type Service struct {
	repo Repository
	now  func() time.Time
}

func NewService(repo Repository) *Service {
	return NewServiceWithClock(repo, time.Now)
}

func NewServiceWithClock(repo Repository, now func() time.Time) *Service {
	return &Service{repo: repo, now: now}
}

func (s *Service) CalculateNeighborhood(ctx context.Context, neighborhoodID string) error {
	neighborhood, err := s.repo.GetNeighborhood(ctx, neighborhoodID)
	if err != nil {
		return err
	}

	aggregate, err := s.repo.AggregateListingSnapshots(ctx, neighborhoodID, neighborhood.TargetLayout)
	if err != nil {
		return err
	}

	_, err = s.repo.InsertNeighborhoodMetric(ctx, MetricSnapshot{
		NeighborhoodID:      neighborhoodID,
		ListedHomes:         aggregate.ListedHomes,
		PriceCutHomes:       aggregate.PriceCutHomes,
		AvgDaysOnMarket:     floatValue(aggregate.AvgDaysOnMarket),
		ListingPriceMin:     floatValue(aggregate.ListingPriceMin),
		ListingPriceMax:     floatValue(aggregate.ListingPriceMax),
		TransactionPriceMin: floatValue(aggregate.TransactionPriceMin),
		TransactionPriceMax: floatValue(aggregate.TransactionPriceMax),
		TransactionMomentum: calculateMomentum(aggregate.ListedHomes, aggregate.PriceCutHomes),
		TargetLayoutSupply:  aggregate.TargetLayoutSupply,
	})
	return err
}

func (s *Service) CalculateCollectionRun(ctx context.Context, command CalculateCollectionRunCommand) error {
	neighborhood, err := s.repo.GetNeighborhood(ctx, command.NeighborhoodID)
	if err != nil {
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
		TargetLayout:   neighborhood.TargetLayout,
	})
	if err != nil {
		return err
	}

	quality := domainneighborhood.AssessQuality(domainneighborhood.QualityInput{
		Now:                    s.now(),
		InventoryCollectedAt:   aggregate.InventoryCollectedAt,
		LatestCoverage:         aggregate.Coverage,
		HasFullInventory:       aggregate.InventoryCollectionRunID != nil,
		ListingSampleCount:     aggregate.ListingSampleCount,
		TransactionSampleCount: aggregate.TransactionSampleCount,
	})

	snapshot := MetricSnapshot{
		NeighborhoodID:           run.NeighborhoodID,
		CollectionRunID:          run.ID,
		InventoryCollectionRunID: aggregate.InventoryCollectionRunID,
		SourceIDs:                aggregate.SourceIDs,
		LatestObservedAt:         aggregate.LatestObservedAt,
		ListedHomes:              aggregate.ListedHomes,
		PriceCutHomes:            aggregate.PriceCutHomes,
		AvgDaysOnMarket:          aggregate.AvgDaysOnMarket,
		ListingPriceMin:          aggregate.ListingPriceMin,
		ListingPriceMax:          aggregate.ListingPriceMax,
		TransactionPriceMin:      aggregate.TransactionPriceMin,
		TransactionPriceMax:      aggregate.TransactionPriceMax,
		TransactionMomentum:      calculateTransactionMomentum(aggregate.TransactionSampleCount, aggregate.LastThirtyDayTransactionCount, aggregate.PrecedingSixtyDayTransactionCount),
		TargetLayoutSupply:       aggregate.TargetLayoutSupply,
		ListingSampleCount:       aggregate.ListingSampleCount,
		TransactionSampleCount:   aggregate.TransactionSampleCount,
		Coverage:                 quality.Coverage,
		Freshness:                quality.Freshness,
		InventoryCollectedAt:     aggregate.InventoryCollectedAt,
		ListedHomesChangePct:     aggregate.ListedHomesChangePct,
		QualityWarnings:          quality.Warnings,
		QualityState:             quality.State,
	}
	if _, err := s.repo.UpsertNeighborhoodMetric(ctx, snapshot); err != nil {
		return err
	}
	return s.repo.MarkCollectionRunMetricCompleted(ctx, run.ID)
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

func calculateTransactionMomentum(sampleCount, lastThirty, precedingSixty int) domainneighborhood.TransactionMomentum {
	if sampleCount < 3 {
		return domainneighborhood.TransactionMomentumUnknown
	}
	baseline := float64(precedingSixty) / 2
	if float64(lastThirty) > baseline*1.2 {
		return domainneighborhood.TransactionMomentumStrong
	}
	if float64(lastThirty) < baseline*0.8 {
		return domainneighborhood.TransactionMomentumWeak
	}
	return domainneighborhood.TransactionMomentumStable
}

func calculateMomentum(listedHomes int, priceCutHomes int) domainneighborhood.TransactionMomentum {
	priceCutShare := 0.0
	if listedHomes > 0 {
		priceCutShare = float64(priceCutHomes) / float64(listedHomes)
	}

	if listedHomes >= 40 && priceCutShare >= 0.2 {
		return domainneighborhood.TransactionMomentumWeak
	}
	if listedHomes < 20 && priceCutShare < 0.1 {
		return domainneighborhood.TransactionMomentumStrong
	}
	return domainneighborhood.TransactionMomentumStable
}

func floatValue(value float64) *float64 {
	return &value
}
