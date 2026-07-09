package capacity

import (
	"context"
	"time"

	"github.com/google/uuid"
	domaincapacity "github.com/propulse/propulse/backend/internal/domain/capacity"
)

type CreateCalculationCommand struct {
	Input domaincapacity.HousingCapacityInput
}

type Service struct {
	repo  CalculationRepository
	now   func() time.Time
	newID func() string
}

func NewService(repo CalculationRepository, now func() time.Time, newID func() string) *Service {
	if now == nil {
		now = time.Now
	}
	if newID == nil {
		newID = uuid.NewString
	}
	return &Service{
		repo:  repo,
		now:   now,
		newID: newID,
	}
}

func (s *Service) CreateCalculation(ctx context.Context, command CreateCalculationCommand) (CalculationRecord, error) {
	record := CalculationRecord{
		ID:        s.newID(),
		Input:     command.Input,
		Result:    domaincapacity.Calculate(command.Input),
		CreatedAt: s.now().UTC(),
	}
	return s.repo.Save(ctx, record)
}
