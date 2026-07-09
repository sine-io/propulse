package metric

import (
	"context"

	domainneighborhood "github.com/sine-io/propulse/internal/domain/neighborhood"
)

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
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
		AvgDaysOnMarket:     aggregate.AvgDaysOnMarket,
		ListingPriceMin:     aggregate.ListingPriceMin,
		ListingPriceMax:     aggregate.ListingPriceMax,
		TransactionPriceMin: aggregate.TransactionPriceMin,
		TransactionPriceMax: aggregate.TransactionPriceMax,
		TransactionMomentum: calculateMomentum(aggregate.ListedHomes, aggregate.PriceCutHomes),
		TargetLayoutSupply:  aggregate.TargetLayoutSupply,
	})
	return err
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
