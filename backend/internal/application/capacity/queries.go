package capacity

import "context"

type GetCalculationQuery struct {
	ID string
}

func (s *Service) GetCalculation(ctx context.Context, query GetCalculationQuery) (CalculationRecord, error) {
	return s.repo.Find(ctx, query.ID)
}
