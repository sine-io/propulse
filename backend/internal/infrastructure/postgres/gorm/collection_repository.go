package gormrepo

import (
	"context"
	"errors"

	appcollection "github.com/sine-io/propulse/backend/internal/application/collection"
	"gorm.io/gorm"
)

type CollectionRepository struct {
	db *gorm.DB
}

func NewCollectionRepository(db *gorm.DB) *CollectionRepository {
	return &CollectionRepository{db: db}
}

func (r *CollectionRepository) NeighborhoodExists(ctx context.Context, id string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&NeighborhoodModel{}).
		Where("id = ?", id).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *CollectionRepository) SaveImport(ctx context.Context, raw appcollection.RawCollectionRecord, snapshots []appcollection.ListingSnapshot) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		rawModel := RawCollectionRecordModel{
			ID:          raw.ID,
			SourceType:  raw.SourceType,
			SourceRef:   raw.SourceRef,
			Payload:     raw.Payload,
			CollectedAt: raw.CollectedAt,
		}
		if err := tx.Create(&rawModel).Error; err != nil {
			return err
		}

		models := make([]ListingSnapshotModel, 0, len(snapshots))
		for _, snapshot := range snapshots {
			models = append(models, ListingSnapshotModel{
				ID:               snapshot.ID,
				CollectionRunID:  snapshot.CollectionRunID,
				NeighborhoodID:   snapshot.NeighborhoodID,
				ListingPrice:     snapshot.ListingPrice,
				TransactionPrice: snapshot.TransactionPrice,
				PriceCut:         snapshot.PriceCut,
				DaysOnMarket:     snapshot.DaysOnMarket,
				Layout:           snapshot.Layout,
				CapturedAt:       snapshot.CapturedAt,
			})
		}
		if len(models) == 0 {
			return errors.New("listing snapshots are required")
		}
		return tx.Create(&models).Error
	})
}
