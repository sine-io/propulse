package capacity

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	domaincapacity "github.com/sine-io/propulse/internal/domain/capacity"
)

type CreateCalculationCommand struct {
	UserID              string
	Input               domaincapacity.HousingCapacityInput
	OldHomeSelection    *OldHomeSelectionInput
	TargetHomeSelection *TargetHomeSelectionInput
}

type Service struct {
	repo                CalculationRepository
	policyRepo          PolicyRepository
	assetReader         AssetReader
	targetListingReader TargetListingReader
	assumptions         domaincapacity.Assumptions
	now                 func() time.Time
	newID               func() string
}

func NewServiceWithPoliciesAndSelections(
	repo CalculationRepository,
	policyRepo PolicyRepository,
	assetReader AssetReader,
	targetListingReader TargetListingReader,
	assumptions domaincapacity.Assumptions,
	now func() time.Time,
	newID func() string,
) *Service {
	service := NewServiceWithPolicies(repo, policyRepo, assumptions, now, newID)
	service.assetReader = assetReader
	service.targetListingReader = targetListingReader
	return service
}

func NewServiceWithPolicies(
	repo CalculationRepository,
	policyRepo PolicyRepository,
	assumptions domaincapacity.Assumptions,
	now func() time.Time,
	newID func() string,
) *Service {
	service := NewService(repo, assumptions, now, newID)
	service.policyRepo = policyRepo
	return service
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
	resolvedInput, selectionContext, err := s.resolveSelections(
		ctx, command.UserID, command.Input, command.OldHomeSelection, command.TargetHomeSelection, now,
	)
	if err != nil {
		return CalculationRecord{}, err
	}
	command.Input = resolvedInput
	var result domaincapacity.HousingCapacityResult
	err = nil
	if command.Input.TransactionScenario != nil || command.Input.LoanPlan != nil {
		if command.Input.TransactionScenario == nil || command.Input.LoanPlan == nil || s.policyRepo == nil {
			return CalculationRecord{}, domaincapacity.ErrInvalidInput
		}
		policy, findErr := s.policyRepo.FindEffective(ctx, strings.TrimSpace(command.Input.TransactionScenario.City), now)
		if findErr != nil {
			if errors.Is(findErr, ErrPolicyNotFound) {
				return CalculationRecord{}, domaincapacity.ErrInvalidAssumptions
			}
			return CalculationRecord{}, findErr
		}
		result, err = domaincapacity.CalculateWithPolicy(command.Input, s.assumptions, policy, now)
	} else {
		result, err = domaincapacity.Calculate(command.Input, s.assumptions, now)
	}
	if err != nil {
		return CalculationRecord{}, err
	}
	record := CalculationRecord{
		ID:               s.newID(),
		UserID:           command.UserID,
		Input:            command.Input,
		Result:           result,
		SelectionContext: selectionContext,
		CreatedAt:        now.UTC(),
	}
	return s.repo.Save(ctx, record)
}

type CreatePolicyVersionCommand struct {
	Policy domaincapacity.HousingPolicyVersion
}

func (s *Service) CreatePolicyVersion(ctx context.Context, command CreatePolicyVersionCommand) (domaincapacity.HousingPolicyVersion, error) {
	if s.policyRepo == nil {
		return domaincapacity.HousingPolicyVersion{}, ErrPolicyNotFound
	}
	policy := command.Policy
	policy.ID = strings.TrimSpace(policy.ID)
	if policy.ID == "" {
		policy.ID = s.newID()
	}
	policy.City = strings.TrimSpace(policy.City)
	policy.Version = strings.TrimSpace(policy.Version)
	policy.Name = strings.TrimSpace(policy.Name)
	policy.EffectiveFrom = strings.TrimSpace(policy.EffectiveFrom)
	if policy.EffectiveTo != nil {
		trimmed := strings.TrimSpace(*policy.EffectiveTo)
		if trimmed == "" {
			policy.EffectiveTo = nil
		} else {
			policy.EffectiveTo = &trimmed
		}
	}
	for index := range policy.Sources {
		policy.Sources[index].Code = strings.TrimSpace(policy.Sources[index].Code)
		policy.Sources[index].Title = strings.TrimSpace(policy.Sources[index].Title)
		policy.Sources[index].Issuer = strings.TrimSpace(policy.Sources[index].Issuer)
		policy.Sources[index].URL = strings.TrimSpace(policy.Sources[index].URL)
		policy.Sources[index].EffectiveDate = strings.TrimSpace(policy.Sources[index].EffectiveDate)
	}
	if err := policy.Validate(); err != nil {
		return domaincapacity.HousingPolicyVersion{}, ErrInvalidPolicy
	}
	policy.CreatedAt = s.now().UTC()
	return s.policyRepo.Create(ctx, policy)
}
