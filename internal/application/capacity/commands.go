package capacity

import (
	"context"
	"time"

	"github.com/google/uuid"
	domaincapacity "github.com/sine-io/propulse/internal/domain/capacity"
)

type CreateCalculationCommand struct {
	UserID string
	Input  domaincapacity.HousingCapacityInput
}

type Service struct {
	repo        CalculationRepository
	assumptions domaincapacity.Assumptions
	now         func() time.Time
	newID       func() string
}

func NewService(
	repo CalculationRepository,
	assumptions domaincapacity.Assumptions,
	now func() time.Time,
	newID func() string,
) *Service {
	if now == nil {
		now = time.Now
	}
	if newID == nil {
		newID = uuid.NewString
	}
	return &Service{
		repo:        repo,
		assumptions: assumptions,
		now:         now,
		newID:       newID,
	}
}

func (s *Service) CreateCalculation(ctx context.Context, command CreateCalculationCommand) (CalculationRecord, error) {
	now := s.now()
	result, err := domaincapacity.Calculate(command.Input, s.assumptions, now)
	if err != nil {
		return CalculationRecord{}, err
	}
	record := CalculationRecord{
		ID:        s.newID(),
		UserID:    command.UserID,
		Input:     command.Input,
		Result:    result,
		CreatedAt: now.UTC(),
	}
	return s.repo.Save(ctx, record)
}
