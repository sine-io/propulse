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
	CashOnHand                float64 `json:"cashOnHand"`
	OldHomeValue              float64 `json:"oldHomeValue"`
	OldLoanBalance            float64 `json:"oldLoanBalance"`
	MonthlyIncome             float64 `json:"monthlyIncome"`
	CurrentMonthlyMortgage    float64 `json:"currentMonthlyMortgage"`
	AcceptableMonthlyMortgage float64 `json:"acceptableMonthlyMortgage"`
	TargetTotalPrice          float64 `json:"targetTotalPrice"`
	RenovationBudget          float64 `json:"renovationBudget"`
	TransactionCosts          float64 `json:"transactionCosts"`
	TransitionRentCost        float64 `json:"transitionRentCost"`
}

type capacityPersistenceResult struct {
	NetOldHomeProceeds          float64                      `json:"netOldHomeProceeds"`
	DeployableCash              float64                      `json:"deployableCash"`
	SafeTotalPrice              float64                      `json:"safeTotalPrice"`
	StrainedTotalPrice          float64                      `json:"strainedTotalPrice"`
	DangerTotalPrice            float64                      `json:"dangerTotalPrice"`
	DownPaymentGap              float64                      `json:"downPaymentGap"`
	MonthlyPayment              float64                      `json:"monthlyPayment"`
	MonthlyPaymentRatio         float64                      `json:"monthlyPaymentRatio"`
	PressureLevel               domaincapacity.PressureLevel `json:"pressureLevel"`
	MinimumSafeOldHomeSalePrice float64                      `json:"minimumSafeOldHomeSalePrice"`
	Strategy                    string                       `json:"strategy"`
	Reasons                     []string                     `json:"reasons"`
}

func newCapacityPersistenceInput(input domaincapacity.HousingCapacityInput) capacityPersistenceInput {
	return capacityPersistenceInput{
		CashOnHand: input.CashOnHand, OldHomeValue: input.OldHomeValue, OldLoanBalance: input.OldLoanBalance,
		MonthlyIncome: input.MonthlyIncome, CurrentMonthlyMortgage: input.CurrentMonthlyMortgage,
		AcceptableMonthlyMortgage: input.AcceptableMonthlyMortgage, TargetTotalPrice: input.TargetTotalPrice,
		RenovationBudget: input.RenovationBudget, TransactionCosts: input.TransactionCosts,
		TransitionRentCost: input.TransitionRentCost,
	}
}

func (input capacityPersistenceInput) domainInput() domaincapacity.HousingCapacityInput {
	return domaincapacity.HousingCapacityInput{
		CashOnHand: input.CashOnHand, OldHomeValue: input.OldHomeValue, OldLoanBalance: input.OldLoanBalance,
		MonthlyIncome: input.MonthlyIncome, CurrentMonthlyMortgage: input.CurrentMonthlyMortgage,
		AcceptableMonthlyMortgage: input.AcceptableMonthlyMortgage, TargetTotalPrice: input.TargetTotalPrice,
		RenovationBudget: input.RenovationBudget, TransactionCosts: input.TransactionCosts,
		TransitionRentCost: input.TransitionRentCost,
	}
}

func newCapacityPersistenceResult(result domaincapacity.HousingCapacityResult) capacityPersistenceResult {
	return capacityPersistenceResult{
		NetOldHomeProceeds: result.NetOldHomeProceeds, DeployableCash: result.DeployableCash,
		SafeTotalPrice: result.SafeTotalPrice, StrainedTotalPrice: result.StrainedTotalPrice,
		DangerTotalPrice: result.DangerTotalPrice, DownPaymentGap: result.DownPaymentGap,
		MonthlyPayment: result.MonthlyPayment, MonthlyPaymentRatio: result.MonthlyPaymentRatio,
		PressureLevel: result.PressureLevel, MinimumSafeOldHomeSalePrice: result.MinimumSafeOldHomeSalePrice,
		Strategy: result.Strategy, Reasons: result.Reasons,
	}
}

func (result capacityPersistenceResult) domainResult() domaincapacity.HousingCapacityResult {
	return domaincapacity.HousingCapacityResult{
		NetOldHomeProceeds: result.NetOldHomeProceeds, DeployableCash: result.DeployableCash,
		SafeTotalPrice: result.SafeTotalPrice, StrainedTotalPrice: result.StrainedTotalPrice,
		DangerTotalPrice: result.DangerTotalPrice, DownPaymentGap: result.DownPaymentGap,
		MonthlyPayment: result.MonthlyPayment, MonthlyPaymentRatio: result.MonthlyPaymentRatio,
		PressureLevel: result.PressureLevel, MinimumSafeOldHomeSalePrice: result.MinimumSafeOldHomeSalePrice,
		Strategy: result.Strategy, Reasons: result.Reasons,
	}
}
