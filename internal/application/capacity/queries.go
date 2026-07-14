package capacity

import (
	"context"

	domaincapacity "github.com/sine-io/propulse/internal/domain/capacity"
)

type GetAssumptionsQuery struct{}

func (s *Service) GetAssumptions(_ context.Context, _ GetAssumptionsQuery) (domaincapacity.Assumptions, error) {
	return s.assumptions, nil
}

type GetCalculationQuery struct {
	ID string
}

func (s *Service) GetCalculation(ctx context.Context, query GetCalculationQuery) (CalculationRecord, error) {
	return s.repo.Find(ctx, query.ID)
}

type LatestCalculationQuery struct {
	UserID string
}

func (s *Service) LatestCalculation(ctx context.Context, query LatestCalculationQuery) (CalculationRecord, error) {
	return s.repo.FindLatestByUser(ctx, query.UserID)
}
