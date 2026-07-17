package gormrepo

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	appcapacity "github.com/sine-io/propulse/internal/application/capacity"
	domaincapacity "github.com/sine-io/propulse/internal/domain/capacity"
	"gorm.io/gorm"
)

type CapacityPolicyRepository struct {
	db *gorm.DB
}

func NewCapacityPolicyRepository(db *gorm.DB) *CapacityPolicyRepository {
	return &CapacityPolicyRepository{db: db}
}

func (r *CapacityPolicyRepository) FindEffective(ctx context.Context, city string, asOf time.Time) (domaincapacity.HousingPolicyVersion, error) {
	var model CapacityPolicyVersionModel
	date := asOf.Format(time.DateOnly)
	err := r.db.WithContext(ctx).
		Where("city = ? AND enabled = ? AND effective_from <= ? AND (effective_to IS NULL OR effective_to > ?)", strings.TrimSpace(city), true, date, date).
		Order("effective_from DESC, created_at DESC, id DESC").
		First(&model).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domaincapacity.HousingPolicyVersion{}, appcapacity.ErrPolicyNotFound
		}
		return domaincapacity.HousingPolicyVersion{}, err
	}
	return capacityPolicyFromModel(model)
}

func (r *CapacityPolicyRepository) List(ctx context.Context, city string) ([]domaincapacity.HousingPolicyVersion, error) {
	query := r.db.WithContext(ctx).Order("effective_from DESC, created_at DESC, id DESC")
	if city = strings.TrimSpace(city); city != "" {
		query = query.Where("city = ?", city)
	}
	var models []CapacityPolicyVersionModel
	if err := query.Find(&models).Error; err != nil {
		return nil, err
	}
	policies := make([]domaincapacity.HousingPolicyVersion, 0, len(models))
	for _, model := range models {
		policy, err := capacityPolicyFromModel(model)
		if err != nil {
			return nil, err
		}
		policies = append(policies, policy)
	}
	return policies, nil
}

func (r *CapacityPolicyRepository) Create(ctx context.Context, policy domaincapacity.HousingPolicyVersion) (domaincapacity.HousingPolicyVersion, error) {
	model, err := capacityPolicyModel(policy)
	if err != nil {
		return domaincapacity.HousingPolicyVersion{}, err
	}
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if model.Enabled {
			if err := tx.Exec("SELECT pg_advisory_xact_lock(hashtext(?))", "capacity-policy:"+model.City).Error; err != nil {
				return err
			}
			query := tx.Model(&CapacityPolicyVersionModel{}).
				Where("city = ? AND enabled = ? AND (effective_to IS NULL OR effective_to > ?)", model.City, true, model.EffectiveFrom)
			if model.EffectiveTo != nil {
				query = query.Where("effective_from < ?", *model.EffectiveTo)
			}
			var overlaps []CapacityPolicyVersionModel
			if err := query.Order("effective_from DESC").Find(&overlaps).Error; err != nil {
				return err
			}
			if len(overlaps) > 0 {
				predecessor := overlaps[0]
				canClosePredecessor := len(overlaps) == 1 && predecessor.EffectiveTo == nil && predecessor.EffectiveFrom.Before(model.EffectiveFrom)
				if !canClosePredecessor {
					return appcapacity.ErrPolicyConflict
				}
				if err := tx.Model(&CapacityPolicyVersionModel{}).
					Where("id = ? AND effective_to IS NULL", predecessor.ID).
					Update("effective_to", model.EffectiveFrom).Error; err != nil {
					return err
				}
			}
		}
		if err := tx.Create(&model).Error; err != nil {
			if isUniqueOrExclusionViolation(err) {
				return appcapacity.ErrPolicyConflict
			}
			return err
		}
		return nil
	})
	if err != nil {
		return domaincapacity.HousingPolicyVersion{}, err
	}
	return capacityPolicyFromModel(model)
}

func capacityPolicyModel(policy domaincapacity.HousingPolicyVersion) (CapacityPolicyVersionModel, error) {
	from, err := time.Parse(time.DateOnly, policy.EffectiveFrom)
	if err != nil {
		return CapacityPolicyVersionModel{}, err
	}
	var to *time.Time
	if policy.EffectiveTo != nil {
		parsed, parseErr := time.Parse(time.DateOnly, *policy.EffectiveTo)
		if parseErr != nil {
			return CapacityPolicyVersionModel{}, parseErr
		}
		to = &parsed
	}
	rules, err := json.Marshal(policy.Rules)
	if err != nil {
		return CapacityPolicyVersionModel{}, err
	}
	sources, err := json.Marshal(policy.Sources)
	if err != nil {
		return CapacityPolicyVersionModel{}, err
	}
	return CapacityPolicyVersionModel{
		ID: policy.ID, City: policy.City, Version: policy.Version, Name: policy.Name,
		EffectiveFrom: from, EffectiveTo: to, Enabled: policy.Enabled,
		Rules: rules, Sources: sources, CreatedAt: policy.CreatedAt,
	}, nil
}

func capacityPolicyFromModel(model CapacityPolicyVersionModel) (domaincapacity.HousingPolicyVersion, error) {
	var rules domaincapacity.HousingPolicyRules
	if err := json.Unmarshal(model.Rules, &rules); err != nil {
		return domaincapacity.HousingPolicyVersion{}, err
	}
	var sources []domaincapacity.PolicySource
	if err := json.Unmarshal(model.Sources, &sources); err != nil {
		return domaincapacity.HousingPolicyVersion{}, err
	}
	var effectiveTo *string
	if model.EffectiveTo != nil {
		value := model.EffectiveTo.Format(time.DateOnly)
		effectiveTo = &value
	}
	policy := domaincapacity.HousingPolicyVersion{
		ID: model.ID, City: model.City, Version: model.Version, Name: model.Name,
		EffectiveFrom: model.EffectiveFrom.Format(time.DateOnly), EffectiveTo: effectiveTo,
		Enabled: model.Enabled, Rules: rules, Sources: sources, CreatedAt: model.CreatedAt,
	}
	if err := policy.Validate(); err != nil {
		return domaincapacity.HousingPolicyVersion{}, err
	}
	return policy, nil
}

func isUniqueOrExclusionViolation(err error) bool {
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "duplicate key") || strings.Contains(message, "exclusion constraint") || strings.Contains(message, "unique constraint")
}
