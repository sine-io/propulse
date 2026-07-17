package gormrepo

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	appasset "github.com/sine-io/propulse/internal/application/asset"
	domainasset "github.com/sine-io/propulse/internal/domain/asset"
	"gorm.io/gorm"
)

type AssetRepository struct {
	db *gorm.DB
}

var _ appasset.Repository = (*AssetRepository)(nil)

func NewAssetRepository(db *gorm.DB) *AssetRepository {
	return &AssetRepository{db: db}
}

func (r *AssetRepository) Create(ctx context.Context, asset domainasset.Asset) (domainasset.Asset, error) {
	model, err := propertyAssetModel(asset)
	if err != nil {
		return domainasset.Asset{}, err
	}
	if err := r.db.WithContext(ctx).Create(&model).Error; err != nil {
		return domainasset.Asset{}, err
	}
	return propertyAssetFromModel(model)
}

func (r *AssetRepository) Update(ctx context.Context, asset domainasset.Asset) (domainasset.Asset, error) {
	model, err := propertyAssetModel(asset)
	if err != nil {
		return domainasset.Asset{}, err
	}
	updates := map[string]any{
		"name": model.Name, "neighborhood_id": model.NeighborhoodID, "neighborhood_name": model.NeighborhoodName,
		"city": model.City, "district": model.District, "layout": model.Layout, "area_sqm": model.AreaSQM,
		"floor_band": model.FloorBand, "floor_description": model.FloorDescription, "orientation": model.Orientation,
		"current_listing_price_wan": model.CurrentListingPriceWan, "original_purchase_price_wan": model.OriginalPurchasePriceWan,
		"purchased_on": model.PurchasedOn, "current_loan_balance_wan": model.CurrentLoanBalanceWan,
		"source_kind": model.SourceKind, "source_snapshot": model.SourceSnapshot, "updated_at": model.UpdatedAt,
	}
	result := r.db.WithContext(ctx).Model(&UserPropertyAssetModel{}).
		Where("id = ? AND user_id = ? AND deleted_at IS NULL", model.ID, model.UserID).
		Updates(updates)
	if result.Error != nil {
		return domainasset.Asset{}, result.Error
	}
	if result.RowsAffected == 0 {
		return domainasset.Asset{}, appasset.ErrAssetNotFound
	}
	return r.Find(ctx, model.UserID, model.ID)
}

func (r *AssetRepository) Find(ctx context.Context, userID, id string) (domainasset.Asset, error) {
	var model UserPropertyAssetModel
	err := r.db.WithContext(ctx).
		Where("id = ? AND user_id = ? AND deleted_at IS NULL", id, userID).
		First(&model).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domainasset.Asset{}, appasset.ErrAssetNotFound
	}
	if err != nil {
		return domainasset.Asset{}, err
	}
	return propertyAssetFromModel(model)
}

func (r *AssetRepository) List(ctx context.Context, userID string, limit, offset int) ([]domainasset.Asset, int, error) {
	query := r.db.WithContext(ctx).Model(&UserPropertyAssetModel{}).
		Where("user_id = ? AND deleted_at IS NULL", userID)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var models []UserPropertyAssetModel
	if err := query.Order("updated_at DESC, id DESC").Limit(limit).Offset(offset).Find(&models).Error; err != nil {
		return nil, 0, err
	}
	items := make([]domainasset.Asset, 0, len(models))
	for _, model := range models {
		item, err := propertyAssetFromModel(model)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	return items, int(total), nil
}

func (r *AssetRepository) SoftDelete(ctx context.Context, userID, id string) error {
	deletedAt := time.Now().UTC()
	result := r.db.WithContext(ctx).Model(&UserPropertyAssetModel{}).
		Where("id = ? AND user_id = ? AND deleted_at IS NULL", id, userID).
		Update("deleted_at", deletedAt)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return appasset.ErrAssetNotFound
	}
	return nil
}

type assetListingSourcePersistence struct {
	SourceListingID string    `json:"sourceListingId"`
	DataSourceID    string    `json:"dataSourceId"`
	DataSourceName  string    `json:"dataSourceName"`
	DataSourceType  string    `json:"dataSourceType"`
	SourceRef       string    `json:"sourceRef"`
	CollectionRunID string    `json:"collectionRunId"`
	SnapshotID      string    `json:"snapshotId"`
	CollectedAt     time.Time `json:"collectedAt"`
	ListedAt        time.Time `json:"listedAt"`
	QualityStatus   string    `json:"qualityStatus"`
}

func propertyAssetModel(asset domainasset.Asset) (UserPropertyAssetModel, error) {
	sourceSnapshot := json.RawMessage(`{}`)
	if asset.ListingSource != nil {
		encoded, err := json.Marshal(assetListingSourcePersistence{
			SourceListingID: asset.ListingSource.SourceListingID, DataSourceID: asset.ListingSource.DataSourceID,
			DataSourceName: asset.ListingSource.DataSourceName, DataSourceType: asset.ListingSource.DataSourceType,
			SourceRef: asset.ListingSource.SourceRef, CollectionRunID: asset.ListingSource.CollectionRunID,
			SnapshotID: asset.ListingSource.SnapshotID, CollectedAt: asset.ListingSource.CollectedAt,
			ListedAt: asset.ListingSource.ListedAt, QualityStatus: asset.ListingSource.QualityStatus,
		})
		if err != nil {
			return UserPropertyAssetModel{}, err
		}
		sourceSnapshot = encoded
	}
	return UserPropertyAssetModel{
		ID: asset.ID, UserID: asset.UserID, Name: asset.Name,
		NeighborhoodID: asset.Property.NeighborhoodID, NeighborhoodName: asset.Property.NeighborhoodName,
		City: asset.Property.City, District: asset.Property.District, Layout: asset.Property.Layout,
		AreaSQM: asset.Property.AreaSQM, FloorBand: asset.Property.FloorBand,
		FloorDescription: asset.Property.FloorDescription, Orientation: asset.Property.Orientation,
		CurrentListingPriceWan:   cloneAssetFloat(asset.Property.CurrentListingPriceWan),
		OriginalPurchasePriceWan: asset.OriginalPurchasePriceWan, PurchasedOn: asset.PurchasedOn,
		CurrentLoanBalanceWan: asset.CurrentLoanBalanceWan, SourceKind: string(asset.SourceKind),
		SourceSnapshot: sourceSnapshot, CreatedAt: asset.CreatedAt, UpdatedAt: asset.UpdatedAt, DeletedAt: asset.DeletedAt,
	}, nil
}

func propertyAssetFromModel(model UserPropertyAssetModel) (domainasset.Asset, error) {
	var source *domainasset.ListingSourceSnapshot
	if model.SourceKind == string(domainasset.SourceMarketListing) {
		var persisted assetListingSourcePersistence
		if err := json.Unmarshal(model.SourceSnapshot, &persisted); err != nil {
			return domainasset.Asset{}, err
		}
		source = &domainasset.ListingSourceSnapshot{
			SourceListingID: persisted.SourceListingID, DataSourceID: persisted.DataSourceID,
			DataSourceName: persisted.DataSourceName, DataSourceType: persisted.DataSourceType,
			SourceRef: persisted.SourceRef, CollectionRunID: persisted.CollectionRunID, SnapshotID: persisted.SnapshotID,
			CollectedAt: persisted.CollectedAt, ListedAt: persisted.ListedAt, QualityStatus: persisted.QualityStatus,
		}
	}
	return domainasset.Asset{
		ID: model.ID, UserID: model.UserID, Name: model.Name,
		Property: domainasset.PropertySnapshot{
			NeighborhoodID: model.NeighborhoodID, NeighborhoodName: model.NeighborhoodName, City: model.City,
			District: model.District, Layout: model.Layout, AreaSQM: model.AreaSQM, FloorBand: model.FloorBand,
			FloorDescription: model.FloorDescription, Orientation: model.Orientation,
			CurrentListingPriceWan: cloneAssetFloat(model.CurrentListingPriceWan),
		},
		OriginalPurchasePriceWan: model.OriginalPurchasePriceWan, PurchasedOn: model.PurchasedOn,
		CurrentLoanBalanceWan: model.CurrentLoanBalanceWan, SourceKind: domainasset.SourceKind(model.SourceKind),
		ListingSource: source, CreatedAt: model.CreatedAt, UpdatedAt: model.UpdatedAt, DeletedAt: model.DeletedAt,
	}, nil
}

func cloneAssetFloat(value *float64) *float64 {
	if value == nil {
		return nil
	}
	clone := *value
	return &clone
}
