package gormrepo

import (
	"context"
	"encoding/json"
	"errors"

	appcapacity "github.com/sine-io/propulse/internal/application/capacity"
	domaincapacity "github.com/sine-io/propulse/internal/domain/capacity"
	"gorm.io/gorm"
)

type CapacityRepository struct {
	db *gorm.DB
}

func NewCapacityRepository(db *gorm.DB) *CapacityRepository {
	return &CapacityRepository{db: db}
}

func (r *CapacityRepository) Save(ctx context.Context, record appcapacity.CalculationRecord) (appcapacity.CalculationRecord, error) {
	input, err := json.Marshal(newCapacityPersistenceInput(record.Input))
	if err != nil {
		return appcapacity.CalculationRecord{}, err
	}
	result, err := json.Marshal(newCapacityPersistenceResult(record.Result))
	if err != nil {
		return appcapacity.CalculationRecord{}, err
	}

	model := CapacityCalculationModel{
		ID:        record.ID,
		UserID:    record.UserID,
		Input:     input,
		Result:    result,
		CreatedAt: record.CreatedAt,
	}
	if err := r.db.WithContext(ctx).Create(&model).Error; err != nil {
		return appcapacity.CalculationRecord{}, err
	}

	return record, nil
}

func (r *CapacityRepository) Find(ctx context.Context, id string) (appcapacity.CalculationRecord, error) {
	var model CapacityCalculationModel
	err := r.db.WithContext(ctx).First(&model, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return appcapacity.CalculationRecord{}, appcapacity.ErrCalculationNotFound
		}
		return appcapacity.CalculationRecord{}, err
	}

	return capacityCalculationFromModel(model)
}

func (r *CapacityRepository) FindLatestByUser(ctx context.Context, userID string) (appcapacity.CalculationRecord, error) {
	var model CapacityCalculationModel
	err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("created_at DESC").
		First(&model).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return appcapacity.CalculationRecord{}, appcapacity.ErrCalculationNotFound
		}
		return appcapacity.CalculationRecord{}, err
	}

	return capacityCalculationFromModel(model)
}

func capacityCalculationFromModel(model CapacityCalculationModel) (appcapacity.CalculationRecord, error) {
	var input capacityPersistenceInput
	if err := json.Unmarshal(model.Input, &input); err != nil {
		return appcapacity.CalculationRecord{}, err
	}
	var result capacityPersistenceResult
	if err := json.Unmarshal(model.Result, &result); err != nil {
		return appcapacity.CalculationRecord{}, err
	}
	return appcapacity.CalculationRecord{
		ID: model.ID, UserID: model.UserID, Input: input.domainInput(), Result: result.domainResult(), CreatedAt: model.CreatedAt,
	}, nil
}

type capacityPersistenceInput struct {
	CashOnHand                float64                        `json:"cashOnHand"`
	OldHomeValue              float64                        `json:"oldHomeValue"`
	OldLoanBalance            float64                        `json:"oldLoanBalance"`
	MonthlyIncome             float64                        `json:"monthlyIncome"`
	CurrentMonthlyMortgage    float64                        `json:"currentMonthlyMortgage"`
	AcceptableMonthlyMortgage float64                        `json:"acceptableMonthlyMortgage"`
	TargetTotalPrice          float64                        `json:"targetTotalPrice"`
	RenovationBudget          float64                        `json:"renovationBudget"`
	TransactionCosts          float64                        `json:"transactionCosts"`
	TransitionRentCost        float64                        `json:"transitionRentCost"`
	LoanOverride              *capacityPersistenceLoanParams `json:"loanOverride,omitempty"`
	CityPolicyOverride        *capacityPersistenceCityPolicy `json:"cityPolicyOverride,omitempty"`
}

type capacityPersistenceResult struct {
	NetOldHomeProceeds          float64                           `json:"netOldHomeProceeds"`
	DeployableCash              float64                           `json:"deployableCash"`
	SafeTotalPrice              float64                           `json:"safeTotalPrice"`
	StrainedTotalPrice          float64                           `json:"strainedTotalPrice"`
	DangerTotalPrice            float64                           `json:"dangerTotalPrice"`
	DownPaymentGap              float64                           `json:"downPaymentGap"`
	MonthlyPayment              float64                           `json:"monthlyPayment"`
	MonthlyPaymentRatio         float64                           `json:"monthlyPaymentRatio"`
	PressureLevel               domaincapacity.PressureLevel      `json:"pressureLevel"`
	MinimumSafeOldHomeSalePrice float64                           `json:"minimumSafeOldHomeSalePrice"`
	Strategy                    string                            `json:"strategy"`
	Reasons                     []string                          `json:"reasons"`
	RuleVersion                 string                            `json:"ruleVersion,omitempty"`
	EffectiveDate               string                            `json:"effectiveDate,omitempty"`
	TraceabilityStatus          domaincapacity.TraceabilityStatus `json:"traceabilityStatus,omitempty"`
	AppliedAssumptions          *capacityPersistenceAssumptions   `json:"appliedAssumptions,omitempty"`
}

type capacityPersistenceLoanParams struct {
	AnnualInterestRate float64                        `json:"annualInterestRate"`
	LoanTermMonths     int                            `json:"loanTermMonths"`
	RepaymentMethod    domaincapacity.RepaymentMethod `json:"repaymentMethod"`
}

type capacityPersistenceCityPolicy struct {
	City            string                          `json:"city"`
	PolicyName      string                          `json:"policyName"`
	DownPaymentRate float64                         `json:"downPaymentRate"`
	EffectiveDate   string                          `json:"effectiveDate"`
	Source          string                          `json:"source"`
	Origin          domaincapacity.AssumptionOrigin `json:"origin,omitempty"`
}

type capacityPersistencePressureThresholds struct {
	SafeRatio        float64 `json:"safeRatio"`
	StrainedRatio    float64 `json:"strainedRatio"`
	DangerRatio      float64 `json:"dangerRatio"`
	DangerMultiplier float64 `json:"dangerMultiplier"`
}

type capacityPersistenceAssumptions struct {
	RuleVersion           string                                `json:"ruleVersion"`
	EffectiveDate         string                                `json:"effectiveDate"`
	RuleSource            string                                `json:"ruleSource"`
	Loan                  capacityPersistenceLoanParams         `json:"loan"`
	LoanSource            string                                `json:"loanSource"`
	LoanOrigin            domaincapacity.AssumptionOrigin       `json:"loanOrigin"`
	CityPolicy            capacityPersistenceCityPolicy         `json:"cityPolicy"`
	ReserveMonths         float64                               `json:"reserveMonths"`
	PressureThresholds    capacityPersistencePressureThresholds `json:"pressureThresholds"`
	OldHomeShareThreshold float64                               `json:"oldHomeShareThreshold"`
}

func newCapacityPersistenceInput(input domaincapacity.HousingCapacityInput) capacityPersistenceInput {
	return capacityPersistenceInput{
		CashOnHand: input.CashOnHand, OldHomeValue: input.OldHomeValue, OldLoanBalance: input.OldLoanBalance,
		MonthlyIncome: input.MonthlyIncome, CurrentMonthlyMortgage: input.CurrentMonthlyMortgage,
		AcceptableMonthlyMortgage: input.AcceptableMonthlyMortgage, TargetTotalPrice: input.TargetTotalPrice,
		RenovationBudget: input.RenovationBudget, TransactionCosts: input.TransactionCosts,
		TransitionRentCost: input.TransitionRentCost, LoanOverride: newCapacityPersistenceLoanParams(input.LoanOverride),
		CityPolicyOverride: newCapacityPersistenceCityPolicy(input.CityPolicyOverride),
	}
}

func (input capacityPersistenceInput) domainInput() domaincapacity.HousingCapacityInput {
	return domaincapacity.HousingCapacityInput{
		CashOnHand: input.CashOnHand, OldHomeValue: input.OldHomeValue, OldLoanBalance: input.OldLoanBalance,
		MonthlyIncome: input.MonthlyIncome, CurrentMonthlyMortgage: input.CurrentMonthlyMortgage,
		AcceptableMonthlyMortgage: input.AcceptableMonthlyMortgage, TargetTotalPrice: input.TargetTotalPrice,
		RenovationBudget: input.RenovationBudget, TransactionCosts: input.TransactionCosts,
		TransitionRentCost: input.TransitionRentCost, LoanOverride: input.LoanOverride.domainPtr(),
		CityPolicyOverride: input.CityPolicyOverride.domainPtr(),
	}
}

func newCapacityPersistenceResult(result domaincapacity.HousingCapacityResult) capacityPersistenceResult {
	return capacityPersistenceResult{
		NetOldHomeProceeds: result.NetOldHomeProceeds, DeployableCash: result.DeployableCash,
		SafeTotalPrice: result.SafeTotalPrice, StrainedTotalPrice: result.StrainedTotalPrice,
		DangerTotalPrice: result.DangerTotalPrice, DownPaymentGap: result.DownPaymentGap,
		MonthlyPayment: result.MonthlyPayment, MonthlyPaymentRatio: result.MonthlyPaymentRatio,
		PressureLevel: result.PressureLevel, MinimumSafeOldHomeSalePrice: result.MinimumSafeOldHomeSalePrice,
		Strategy: result.Strategy, Reasons: result.Reasons, RuleVersion: result.RuleVersion,
		EffectiveDate: result.EffectiveDate, TraceabilityStatus: result.TraceabilityStatus,
		AppliedAssumptions: newCapacityPersistenceAssumptions(result.AppliedAssumptions),
	}
}

func (result capacityPersistenceResult) domainResult() domaincapacity.HousingCapacityResult {
	domainResult := domaincapacity.HousingCapacityResult{
		NetOldHomeProceeds: result.NetOldHomeProceeds, DeployableCash: result.DeployableCash,
		SafeTotalPrice: result.SafeTotalPrice, StrainedTotalPrice: result.StrainedTotalPrice,
		DangerTotalPrice: result.DangerTotalPrice, DownPaymentGap: result.DownPaymentGap,
		MonthlyPayment: result.MonthlyPayment, MonthlyPaymentRatio: result.MonthlyPaymentRatio,
		PressureLevel: result.PressureLevel, MinimumSafeOldHomeSalePrice: result.MinimumSafeOldHomeSalePrice,
		Strategy: result.Strategy, Reasons: result.Reasons, RuleVersion: result.RuleVersion,
		EffectiveDate: result.EffectiveDate, TraceabilityStatus: result.TraceabilityStatus,
		AppliedAssumptions: result.AppliedAssumptions.domainPtr(),
	}
	if domainResult.AppliedAssumptions == nil {
		domainResult.TraceabilityStatus = domaincapacity.TraceabilityLegacyUnversioned
	} else if domainResult.TraceabilityStatus == "" {
		domainResult.TraceabilityStatus = domaincapacity.TraceabilityComplete
	}
	return domainResult
}

func newCapacityPersistenceLoanParams(loan *domaincapacity.LoanParams) *capacityPersistenceLoanParams {
	if loan == nil {
		return nil
	}
	return &capacityPersistenceLoanParams{
		AnnualInterestRate: loan.AnnualInterestRate,
		LoanTermMonths:     loan.LoanTermMonths,
		RepaymentMethod:    loan.RepaymentMethod,
	}
}

func (loan *capacityPersistenceLoanParams) domainPtr() *domaincapacity.LoanParams {
	if loan == nil {
		return nil
	}
	return &domaincapacity.LoanParams{
		AnnualInterestRate: loan.AnnualInterestRate,
		LoanTermMonths:     loan.LoanTermMonths,
		RepaymentMethod:    loan.RepaymentMethod,
	}
}

func newCapacityPersistenceCityPolicy(policy *domaincapacity.CityPolicy) *capacityPersistenceCityPolicy {
	if policy == nil {
		return nil
	}
	return &capacityPersistenceCityPolicy{
		City: policy.City, PolicyName: policy.PolicyName, DownPaymentRate: policy.DownPaymentRate,
		EffectiveDate: policy.EffectiveDate, Source: policy.Source, Origin: policy.Origin,
	}
}

func (policy *capacityPersistenceCityPolicy) domainPtr() *domaincapacity.CityPolicy {
	if policy == nil {
		return nil
	}
	return &domaincapacity.CityPolicy{
		City: policy.City, PolicyName: policy.PolicyName, DownPaymentRate: policy.DownPaymentRate,
		EffectiveDate: policy.EffectiveDate, Source: policy.Source, Origin: policy.Origin,
	}
}

func newCapacityPersistenceAssumptions(assumptions *domaincapacity.Assumptions) *capacityPersistenceAssumptions {
	if assumptions == nil {
		return nil
	}
	return &capacityPersistenceAssumptions{
		RuleVersion: assumptions.RuleVersion, EffectiveDate: assumptions.EffectiveDate, RuleSource: assumptions.RuleSource,
		Loan: capacityPersistenceLoanParams{
			AnnualInterestRate: assumptions.Loan.AnnualInterestRate,
			LoanTermMonths:     assumptions.Loan.LoanTermMonths,
			RepaymentMethod:    assumptions.Loan.RepaymentMethod,
		},
		LoanSource: assumptions.LoanSource, LoanOrigin: assumptions.LoanOrigin,
		CityPolicy: capacityPersistenceCityPolicy{
			City: assumptions.CityPolicy.City, PolicyName: assumptions.CityPolicy.PolicyName,
			DownPaymentRate: assumptions.CityPolicy.DownPaymentRate, EffectiveDate: assumptions.CityPolicy.EffectiveDate,
			Source: assumptions.CityPolicy.Source, Origin: assumptions.CityPolicy.Origin,
		},
		ReserveMonths: assumptions.ReserveMonths,
		PressureThresholds: capacityPersistencePressureThresholds{
			SafeRatio: assumptions.PressureThresholds.SafeRatio, StrainedRatio: assumptions.PressureThresholds.StrainedRatio,
			DangerRatio: assumptions.PressureThresholds.DangerRatio, DangerMultiplier: assumptions.PressureThresholds.DangerMultiplier,
		},
		OldHomeShareThreshold: assumptions.OldHomeShareThreshold,
	}
}

func (assumptions *capacityPersistenceAssumptions) domainPtr() *domaincapacity.Assumptions {
	if assumptions == nil {
		return nil
	}
	return &domaincapacity.Assumptions{
		RuleVersion: assumptions.RuleVersion, EffectiveDate: assumptions.EffectiveDate, RuleSource: assumptions.RuleSource,
		Loan: domaincapacity.LoanParams{
			AnnualInterestRate: assumptions.Loan.AnnualInterestRate,
			LoanTermMonths:     assumptions.Loan.LoanTermMonths,
			RepaymentMethod:    assumptions.Loan.RepaymentMethod,
		},
		LoanSource: assumptions.LoanSource, LoanOrigin: assumptions.LoanOrigin,
		CityPolicy: domaincapacity.CityPolicy{
			City: assumptions.CityPolicy.City, PolicyName: assumptions.CityPolicy.PolicyName,
			DownPaymentRate: assumptions.CityPolicy.DownPaymentRate, EffectiveDate: assumptions.CityPolicy.EffectiveDate,
			Source: assumptions.CityPolicy.Source, Origin: assumptions.CityPolicy.Origin,
		},
		ReserveMonths: assumptions.ReserveMonths,
		PressureThresholds: domaincapacity.PressureThresholds{
			SafeRatio: assumptions.PressureThresholds.SafeRatio, StrainedRatio: assumptions.PressureThresholds.StrainedRatio,
			DangerRatio: assumptions.PressureThresholds.DangerRatio, DangerMultiplier: assumptions.PressureThresholds.DangerMultiplier,
		},
		OldHomeShareThreshold: assumptions.OldHomeShareThreshold,
	}
}
