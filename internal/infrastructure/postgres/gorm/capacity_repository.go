package gormrepo

import (
	"context"
	"encoding/json"
	"errors"

	appcapacity "github.com/sine-io/propulse/internal/application/capacity"
	"gorm.io/gorm"
)

type CapacityRepository struct {
	db *gorm.DB
}

func NewCapacityRepository(db *gorm.DB) *CapacityRepository {
	return &CapacityRepository{db: db}
}

func (r *CapacityRepository) Save(ctx context.Context, record appcapacity.CalculationRecord) (appcapacity.CalculationRecord, error) {
	input, err := json.Marshal(record.Input)
	if err != nil {
		return appcapacity.CalculationRecord{}, err
	}
	result, err := json.Marshal(record.Result)
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
	var record appcapacity.CalculationRecord
	if err := json.Unmarshal(model.Input, &record.Input); err != nil {
		return appcapacity.CalculationRecord{}, err
	}
	if err := json.Unmarshal(model.Result, &record.Result); err != nil {
		return appcapacity.CalculationRecord{}, err
	}
	record.ID = model.ID
	record.UserID = model.UserID
	record.CreatedAt = model.CreatedAt
	return record, nil
}
