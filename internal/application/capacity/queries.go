package capacity

import (
	"context"
	"errors"
	"strings"

	domaincapacity "github.com/sine-io/propulse/internal/domain/capacity"
)

type GetAssumptionsQuery struct {
	City              string
	HomePurchaseOrder domaincapacity.HomePurchaseOrder
	LoanTermMonths    int
}

func (s *Service) GetAssumptions(ctx context.Context, query GetAssumptionsQuery) (AssumptionsView, error) {
	view := AssumptionsView{Legacy: s.assumptions, Disclaimer: domaincapacity.BudgetEstimateDisclaimer}
	if query.City = strings.TrimSpace(query.City); query.City == "" {
		query.City = "天津"
	}
	if query.HomePurchaseOrder == "" {
		query.HomePurchaseOrder = domaincapacity.HomeFirst
	}
	if query.HomePurchaseOrder != domaincapacity.HomeFirst && query.HomePurchaseOrder != domaincapacity.HomeSecond {
		return AssumptionsView{}, domaincapacity.ErrInvalidInput
	}
	if query.LoanTermMonths == 0 {
		query.LoanTermMonths = 360
	}
	if query.LoanTermMonths < 1 || query.LoanTermMonths > 360 {
		return AssumptionsView{}, domaincapacity.ErrInvalidInput
	}
	view.HomePurchaseOrder = query.HomePurchaseOrder
	view.LoanTermMonths = query.LoanTermMonths
	if s.policyRepo == nil {
		return view, nil
	}
	policy, err := s.policyRepo.FindEffective(ctx, query.City, s.now())
	if err != nil {
		if errors.Is(err, ErrPolicyNotFound) {
			return view, nil
		}
		return AssumptionsView{}, err
	}
	view.Policy = &policy
	view.LoanOptions = policyLoanOptions(policy, query.HomePurchaseOrder, query.LoanTermMonths)
	view.Legacy = projectPolicyToLegacyAssumptions(view.Legacy, policy, view.LoanOptions, query.LoanTermMonths)
	return view, nil
}

type ListPolicyVersionsQuery struct {
	City string
}

func (s *Service) ListPolicyVersions(ctx context.Context, query ListPolicyVersionsQuery) ([]domaincapacity.HousingPolicyVersion, error) {
	if s.policyRepo == nil {
		return nil, ErrPolicyNotFound
	}
	return s.policyRepo.List(ctx, strings.TrimSpace(query.City))
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

func policyLoanOptions(policy domaincapacity.HousingPolicyVersion, order domaincapacity.HomePurchaseOrder, term int) []LoanOption {
	down := policy.Rules.DownPayment
	interest := policy.Rules.Interest
	commercialRate := interest.CommercialFirst
	commercialDown := down.CommercialFirst
	providentDown := down.ProvidentFirst
	combinedDown := down.CombinedFirst
	providentRate := interest.ProvidentFirstOverFiveYears
	if term <= 60 {
		providentRate = interest.ProvidentFirstUpToFiveYears
	}
	if order == domaincapacity.HomeSecond {
		commercialRate = interest.CommercialSecond
		commercialDown = down.CommercialSecond
		providentDown = down.ProvidentSecond
		combinedDown = down.CombinedSecond
		providentRate = interest.ProvidentSecondOverFiveYears
		if term <= 60 {
			providentRate = interest.ProvidentSecondUpToFiveYears
		}
	}
	commercial := commercialRate
	provident := providentRate
	return []LoanOption{
		{Type: domaincapacity.LoanCommercial, DownPaymentRate: commercialDown, CommercialAnnualInterestRate: &commercial},
		{Type: domaincapacity.LoanProvidentFund, DownPaymentRate: providentDown, ProvidentAnnualInterestRate: &provident},
		{Type: domaincapacity.LoanCombined, DownPaymentRate: combinedDown, CommercialAnnualInterestRate: &commercial, ProvidentAnnualInterestRate: &provident},
	}
}

// projectPolicyToLegacyAssumptions keeps old clients working without letting
// compatibility configuration override an effective database policy.
func projectPolicyToLegacyAssumptions(
	legacy domaincapacity.Assumptions,
	policy domaincapacity.HousingPolicyVersion,
	options []LoanOption,
	term int,
) domaincapacity.Assumptions {
	legacy.RuleVersion = policy.Version
	legacy.EffectiveDate = policy.EffectiveFrom
	legacy.RuleSource = policy.Name

	if len(options) > 0 {
		commercial := options[0]
		if commercial.CommercialAnnualInterestRate != nil {
			legacy.Loan.AnnualInterestRate = *commercial.CommercialAnnualInterestRate
		}
		legacy.Loan.LoanTermMonths = term
		legacy.LoanSource = policySourceURL(policy, "commercial_rate")
		legacy.LoanOrigin = domaincapacity.OriginConfiguredDefault
		legacy.CityPolicy = domaincapacity.CityPolicy{
			City:            policy.City,
			PolicyName:      policy.Name,
			DownPaymentRate: commercial.DownPaymentRate,
			EffectiveDate:   policy.EffectiveFrom,
			Source:          policySourceURL(policy, "commercial_down_payment"),
			Origin:          domaincapacity.OriginConfiguredDefault,
		}
	}
	return legacy
}

func policySourceURL(policy domaincapacity.HousingPolicyVersion, code string) string {
	for _, source := range policy.Sources {
		if source.Code == code {
			return source.URL
		}
	}
	if len(policy.Sources) > 0 {
		return policy.Sources[0].URL
	}
	return "policy_repository"
}
