package gormrepo

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"github.com/google/uuid"
	appcommunitymarket "github.com/sine-io/propulse/internal/application/communitymarket"
	domaincommunitymarket "github.com/sine-io/propulse/internal/domain/communitymarket"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type CommunityMarketRepository struct {
	db *gorm.DB
}

var _ appcommunitymarket.Repository = (*CommunityMarketRepository)(nil)

func NewCommunityMarketRepository(db *gorm.DB) *CommunityMarketRepository {
	return &CommunityMarketRepository{db: db}
}

func (r *CommunityMarketRepository) DataSourceExists(ctx context.Context, id string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&DataSourceModel{}).Where("id = ?", id).Count(&count).Error
	return count > 0, err
}

func (r *CommunityMarketRepository) NeighborhoodExists(ctx context.Context, id string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&NeighborhoodModel{}).Where("id = ?", id).Count(&count).Error
	return count > 0, err
}

func (r *CommunityMarketRepository) SaveSnapshot(ctx context.Context, snapshot appcommunitymarket.Snapshot) (appcommunitymarket.SaveSnapshotResult, error) {
	model, err := communityMarketSnapshotModel(snapshot)
	if err != nil {
		return appcommunitymarket.SaveSnapshotResult{}, err
	}
	create := r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&model)
	if create.Error != nil {
		return appcommunitymarket.SaveSnapshotResult{}, create.Error
	}
	if create.RowsAffected > 0 {
		return appcommunitymarket.SaveSnapshotResult{Snapshot: communityMarketSnapshotFromModel(model), Created: true}, nil
	}

	var existing CommunityMarketSnapshotModel
	err = r.db.WithContext(ctx).
		Where("data_source_id = ? AND source_ref = ? AND content_checksum = ?", model.DataSourceID, model.SourceRef, model.ContentChecksum).
		First(&existing).Error
	if err != nil {
		return appcommunitymarket.SaveSnapshotResult{}, err
	}
	return appcommunitymarket.SaveSnapshotResult{Snapshot: communityMarketSnapshotFromModel(existing), Created: false}, nil
}

func (r *CommunityMarketRepository) SaveFangjian(ctx context.Context, batch appcommunitymarket.FangjianImportBatch) (appcommunitymarket.SaveFangjianResult, error) {
	var result appcommunitymarket.SaveFangjianResult
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		validationSummary, _ := json.Marshal(map[string]any{
			"recordCount":  len(batch.Listings) + len(batch.Transactions),
			"listingCount": len(batch.Listings), "transactionCount": len(batch.Transactions),
			"issues": []any{},
		})
		run := CollectionRunModel{
			ID: batch.CollectionRunID, DataSourceID: batch.Snapshot.DataSourceID,
			NeighborhoodID: batch.Snapshot.NeighborhoodID, SourceRef: batch.Snapshot.SourceRef,
			CollectedAt: batch.Snapshot.CollectedAt, Coverage: "full", ImportFormat: "json",
			ContentChecksum: batch.Snapshot.ContentChecksum,
			RawPayload:      append([]byte(nil), batch.Snapshot.RawPayload...), RawContentType: "application/json",
			ValidationSummary: validationSummary, Status: "completed", MetricStatus: "pending",
			CreatedAt: batch.Snapshot.CreatedAt, UpdatedAt: batch.Snapshot.CreatedAt,
		}
		created := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&run)
		if created.Error != nil {
			return created.Error
		}
		if created.RowsAffected == 0 {
			var existingRun CollectionRunModel
			if err := tx.Where("data_source_id = ? AND source_ref = ? AND content_checksum = ?", run.DataSourceID, run.SourceRef, run.ContentChecksum).First(&existingRun).Error; err != nil {
				return err
			}
			var existingSnapshot CommunityMarketSnapshotModel
			if err := tx.Where("collection_run_id = ?", existingRun.ID).First(&existingSnapshot).Error; err != nil {
				return err
			}
			result = appcommunitymarket.SaveFangjianResult{Snapshot: communityMarketSnapshotFromModel(existingSnapshot), Created: false}
			return nil
		}

		snapshotModel, err := communityMarketSnapshotModel(batch.Snapshot)
		if err != nil {
			return err
		}
		if err := tx.Create(&snapshotModel).Error; err != nil {
			return err
		}
		listingModels := make([]ListingObservationModel, 0, len(batch.Listings))
		layoutModels := make([]NeighborhoodLayoutModel, 0, len(batch.Listings)+len(batch.Transactions))
		for index, listing := range batch.Listings {
			attributes, err := json.Marshal(map[string]string{
				"listingUnitPrice": formatMarketFloat(listing.ListingUnitPrice),
				"listedAt":         listing.ListedAt.UTC().Format(time.RFC3339), "floorBand": listing.FloorBand,
				"floorDescription": listing.FloorDescription, "orientation": listing.Orientation,
				"adjustmentCount": strconv.Itoa(listing.AdjustmentCount), "followCount": strconv.Itoa(listing.FollowCount),
				"lookCount30Days": strconv.Itoa(listing.LookCount30Days),
			})
			if err != nil {
				return err
			}
			listingModels = append(listingModels, ListingObservationModel{
				ID: uuid.NewString(), CollectionRunID: run.ID, NeighborhoodID: run.NeighborhoodID,
				SourceListingID: listing.RoomID, SourceRow: index + 1, Layout: listing.Layout,
				AreaSQM: listing.AreaSQM, ListingPrice: listing.ListingTotalPriceWan,
				DaysOnMarket: listing.DaysOnMarket, Status: "active", CapturedAt: run.CollectedAt,
				Attributes: attributes,
			})
			layoutModels = append(layoutModels, NeighborhoodLayoutModel{NeighborhoodID: run.NeighborhoodID, Layout: listing.Layout})
		}
		transactionModels := make([]TransactionObservationModel, 0, len(batch.Transactions))
		for index, transaction := range batch.Transactions {
			attributes, err := json.Marshal(map[string]string{
				"listingTotalPriceWan": formatMarketFloat(transaction.ListingTotalPriceWan),
				"tradeUnitPrice":       formatMarketFloat(transaction.TradeUnitPrice),
				"negotiationWan":       formatMarketFloat(transaction.NegotiationWan),
				"negotiationPercent":   formatMarketFloat(transaction.NegotiationPercent),
				"floorBand":            transaction.FloorBand, "floorDescription": transaction.FloorDescription,
				"orientation": transaction.Orientation, "adjustmentCount": strconv.Itoa(transaction.AdjustmentCount),
			})
			if err != nil {
				return err
			}
			transactionModels = append(transactionModels, TransactionObservationModel{
				ID: uuid.NewString(), CollectionRunID: run.ID, NeighborhoodID: run.NeighborhoodID,
				SourceRecordID: transaction.RoomID, SourceRow: index + 1, Layout: transaction.Layout,
				AreaSQM: transaction.AreaSQM, TransactionPrice: transaction.TradeTotalPriceWan,
				TransactionDate: transaction.TradeDate, CapturedAt: run.CollectedAt, Attributes: attributes,
			})
			layoutModels = append(layoutModels, NeighborhoodLayoutModel{NeighborhoodID: run.NeighborhoodID, Layout: transaction.Layout})
		}
		adjustmentModels := make([]ListingAdjustmentModel, 0, len(batch.Adjustments))
		for _, adjustment := range batch.Adjustments {
			adjustmentModels = append(adjustmentModels, ListingAdjustmentModel{
				ID: adjustment.ID, CollectionRunID: run.ID, NeighborhoodID: run.NeighborhoodID,
				RoomID: adjustment.RoomID, AdjustedAt: adjustment.AdjustedAt,
				PriceBeforeWan: adjustment.PriceBeforeWan, PriceAfterWan: adjustment.PriceAfterWan,
				AmountWan: adjustment.AmountWan, CreatedAt: batch.Snapshot.CreatedAt,
			})
		}
		if len(listingModels) > 0 {
			if err := tx.Create(&listingModels).Error; err != nil {
				return err
			}
		}
		if len(transactionModels) > 0 {
			if err := tx.Create(&transactionModels).Error; err != nil {
				return err
			}
		}
		if len(adjustmentModels) > 0 {
			if err := tx.Create(&adjustmentModels).Error; err != nil {
				return err
			}
		}
		if len(layoutModels) > 0 {
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&layoutModels).Error; err != nil {
				return err
			}
		}
		result = appcommunitymarket.SaveFangjianResult{Snapshot: communityMarketSnapshotFromModel(snapshotModel), Created: true}
		return nil
	})
	return result, err
}

func (r *CommunityMarketRepository) LatestSnapshot(ctx context.Context, neighborhoodID string) (appcommunitymarket.Snapshot, error) {
	var model CommunityMarketSnapshotModel
	err := r.db.WithContext(ctx).
		Where("neighborhood_id = ?", neighborhoodID).
		Order("collected_at DESC, created_at DESC, id DESC").
		First(&model).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return appcommunitymarket.Snapshot{}, appcommunitymarket.ErrSnapshotNotFound
	}
	if err != nil {
		return appcommunitymarket.Snapshot{}, err
	}
	return communityMarketSnapshotFromModel(model), nil
}

func (r *CommunityMarketRepository) LatestListings(ctx context.Context, neighborhoodID string) ([]appcommunitymarket.MarketListing, error) {
	runID, err := r.latestFangjianRunID(ctx, neighborhoodID)
	if err != nil {
		return nil, err
	}
	var models []ListingObservationModel
	if err := r.db.WithContext(ctx).Where("collection_run_id = ?", runID).Find(&models).Error; err != nil {
		return nil, err
	}
	items := make([]appcommunitymarket.MarketListing, 0, len(models))
	for _, model := range models {
		attributes := map[string]string{}
		_ = json.Unmarshal(model.Attributes, &attributes)
		items = append(items, appcommunitymarket.MarketListing{
			RoomID: model.SourceListingID, Layout: model.Layout, AreaSQM: model.AreaSQM,
			ListingTotalPriceWan: model.ListingPrice, ListingUnitPrice: marketFloat(attributes["listingUnitPrice"]),
			ListedAt: marketTime(attributes["listedAt"]), DaysOnMarket: model.DaysOnMarket,
			FloorBand: attributes["floorBand"], FloorDescription: attributes["floorDescription"],
			Orientation: attributes["orientation"], AdjustmentCount: marketInt(attributes["adjustmentCount"]),
			FollowCount: marketInt(attributes["followCount"]), LookCount30Days: marketInt(attributes["lookCount30Days"]),
		})
	}
	return items, nil
}

func (r *CommunityMarketRepository) LatestListing(ctx context.Context, neighborhoodID, roomID string) (appcommunitymarket.MarketListingDetail, error) {
	snapshot, err := r.latestFangjianSnapshot(ctx, neighborhoodID)
	if err != nil {
		return appcommunitymarket.MarketListingDetail{}, err
	}

	var model ListingObservationModel
	err = r.db.WithContext(ctx).
		Where("collection_run_id = ? AND source_listing_id = ?", *snapshot.CollectionRunID, roomID).
		First(&model).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		var historicalCount int64
		if countErr := r.db.WithContext(ctx).Model(&ListingObservationModel{}).
			Where("neighborhood_id = ? AND source_listing_id = ?", neighborhoodID, roomID).
			Count(&historicalCount).Error; countErr != nil {
			return appcommunitymarket.MarketListingDetail{}, countErr
		}
		if historicalCount > 0 {
			return appcommunitymarket.MarketListingDetail{}, appcommunitymarket.ErrListingUnavailable
		}
		return appcommunitymarket.MarketListingDetail{}, appcommunitymarket.ErrListingNotFound
	}
	if err != nil {
		return appcommunitymarket.MarketListingDetail{}, err
	}
	if model.Status != "active" {
		return appcommunitymarket.MarketListingDetail{}, appcommunitymarket.ErrListingUnavailable
	}

	var source DataSourceModel
	if err := r.db.WithContext(ctx).First(&source, "id = ?", snapshot.DataSourceID).Error; err != nil {
		return appcommunitymarket.MarketListingDetail{}, err
	}
	var neighborhood NeighborhoodModel
	if err := r.db.WithContext(ctx).First(&neighborhood, "id = ?", neighborhoodID).Error; err != nil {
		return appcommunitymarket.MarketListingDetail{}, err
	}
	city := ""
	if neighborhood.City != nil {
		city = *neighborhood.City
	}
	return appcommunitymarket.MarketListingDetail{
		MarketListing:    marketListingFromModel(model),
		NeighborhoodID:   neighborhood.ID,
		NeighborhoodName: neighborhood.Name,
		City:             city,
		District:         neighborhood.Area,
		Status:           model.Status,
		SnapshotID:       snapshot.ID,
		CollectionRunID:  *snapshot.CollectionRunID,
		CollectedAt:      snapshot.CollectedAt,
		Source: appcommunitymarket.MarketSource{
			DataSourceID: source.ID, DataSourceName: source.Name, DataSourceType: source.SourceType, SourceRef: snapshot.SourceRef,
		},
		QualityStatus: snapshot.QualityStatus,
	}, nil
}

func (r *CommunityMarketRepository) LatestTransactions(ctx context.Context, neighborhoodID string) ([]appcommunitymarket.MarketTransaction, error) {
	runID, err := r.latestFangjianRunID(ctx, neighborhoodID)
	if err != nil {
		return nil, err
	}
	var models []TransactionObservationModel
	if err := r.db.WithContext(ctx).Where("collection_run_id = ?", runID).Find(&models).Error; err != nil {
		return nil, err
	}
	items := make([]appcommunitymarket.MarketTransaction, 0, len(models))
	for _, model := range models {
		attributes := map[string]string{}
		_ = json.Unmarshal(model.Attributes, &attributes)
		items = append(items, appcommunitymarket.MarketTransaction{
			RoomID: model.SourceRecordID, Layout: model.Layout, AreaSQM: model.AreaSQM,
			ListingTotalPriceWan: marketFloat(attributes["listingTotalPriceWan"]), TradeTotalPriceWan: model.TransactionPrice,
			TradeUnitPrice: marketFloat(attributes["tradeUnitPrice"]), TradeDate: model.TransactionDate,
			NegotiationWan: marketFloat(attributes["negotiationWan"]), NegotiationPercent: marketFloat(attributes["negotiationPercent"]),
			FloorBand: attributes["floorBand"], FloorDescription: attributes["floorDescription"], Orientation: attributes["orientation"],
			AdjustmentCount: marketInt(attributes["adjustmentCount"]),
		})
	}
	return items, nil
}

func (r *CommunityMarketRepository) LatestAdjustments(ctx context.Context, neighborhoodID, roomID string) ([]appcommunitymarket.ListingAdjustment, error) {
	runID, err := r.latestFangjianRunID(ctx, neighborhoodID)
	if err != nil {
		return nil, err
	}
	var models []ListingAdjustmentModel
	if err := r.db.WithContext(ctx).Where("collection_run_id = ? AND room_id = ?", runID, roomID).Order("adjusted_at DESC, id DESC").Find(&models).Error; err != nil {
		return nil, err
	}
	items := make([]appcommunitymarket.ListingAdjustment, 0, len(models))
	for _, model := range models {
		items = append(items, appcommunitymarket.ListingAdjustment{
			ID: model.ID, RoomID: model.RoomID, AdjustedAt: model.AdjustedAt,
			PriceBeforeWan: model.PriceBeforeWan, PriceAfterWan: model.PriceAfterWan, AmountWan: model.AmountWan,
		})
	}
	return items, nil
}

func (r *CommunityMarketRepository) latestFangjianRunID(ctx context.Context, neighborhoodID string) (string, error) {
	model, err := r.latestFangjianSnapshot(ctx, neighborhoodID)
	if err != nil {
		return "", err
	}
	return *model.CollectionRunID, nil
}

func (r *CommunityMarketRepository) latestFangjianSnapshot(ctx context.Context, neighborhoodID string) (CommunityMarketSnapshotModel, error) {
	var model CommunityMarketSnapshotModel
	err := r.db.WithContext(ctx).
		Where("neighborhood_id = ? AND collection_run_id IS NOT NULL", neighborhoodID).
		Order("collected_at DESC, created_at DESC, id DESC").First(&model).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return CommunityMarketSnapshotModel{}, appcommunitymarket.ErrSnapshotNotFound
	}
	if err != nil {
		return CommunityMarketSnapshotModel{}, err
	}
	if model.CollectionRunID == nil {
		return CommunityMarketSnapshotModel{}, appcommunitymarket.ErrSnapshotNotFound
	}
	return model, nil
}

func marketListingFromModel(model ListingObservationModel) appcommunitymarket.MarketListing {
	attributes := map[string]string{}
	_ = json.Unmarshal(model.Attributes, &attributes)
	return appcommunitymarket.MarketListing{
		RoomID: model.SourceListingID, Layout: model.Layout, AreaSQM: model.AreaSQM,
		ListingTotalPriceWan: model.ListingPrice, ListingUnitPrice: marketFloat(attributes["listingUnitPrice"]),
		ListedAt: marketTime(attributes["listedAt"]), DaysOnMarket: model.DaysOnMarket,
		FloorBand: attributes["floorBand"], FloorDescription: attributes["floorDescription"],
		Orientation: attributes["orientation"], AdjustmentCount: marketInt(attributes["adjustmentCount"]),
		FollowCount: marketInt(attributes["followCount"]), LookCount30Days: marketInt(attributes["lookCount30Days"]),
	}
}

func communityMarketSnapshotModel(snapshot appcommunitymarket.Snapshot) (CommunityMarketSnapshotModel, error) {
	roomTypes, err := json.Marshal(snapshot.Data.OnSaleRoomTypes)
	if err != nil {
		return CommunityMarketSnapshotModel{}, err
	}
	var propertyTags json.RawMessage
	if len(snapshot.Data.PropertyTags) > 0 {
		propertyTags, err = json.Marshal(snapshot.Data.PropertyTags)
		if err != nil {
			return CommunityMarketSnapshotModel{}, err
		}
	}
	return CommunityMarketSnapshotModel{
		ID:                                snapshot.ID,
		DataSourceID:                      snapshot.DataSourceID,
		NeighborhoodID:                    snapshot.NeighborhoodID,
		SourceRef:                         snapshot.SourceRef,
		CollectedAt:                       snapshot.CollectedAt,
		ContentChecksum:                   snapshot.ContentChecksum,
		RawPayload:                        append([]byte(nil), snapshot.RawPayload...),
		RawContentType:                    snapshot.RawContentType,
		CollectionRunID:                   snapshot.CollectionRunID,
		SourceCommunityID:                 snapshot.Data.SourceCommunityID,
		CommunityName:                     snapshot.Data.CommunityName,
		FormerName:                        snapshot.Data.FormerName,
		ProvinceCode:                      optionalCommunityMarketString(snapshot.Data.ProvinceCode),
		ProvinceName:                      optionalCommunityMarketString(snapshot.Data.ProvinceName),
		CityCode:                          snapshot.Data.CityCode,
		CityName:                          snapshot.Data.CityName,
		DistrictCode:                      snapshot.Data.DistrictCode,
		DistrictName:                      snapshot.Data.DistrictName,
		BlockCode:                         snapshot.Data.BlockCode,
		BlockName:                         snapshot.Data.BlockName,
		PropertyType:                      optionalCommunityMarketString(snapshot.Data.PropertyType),
		PropertyTags:                      propertyTags,
		BuildingCount:                     snapshot.Data.BuildingCount,
		BuildingType:                      optionalCommunityMarketString(snapshot.Data.BuildingType),
		BuildingYear:                      snapshot.Data.BuildingYear,
		Developer:                         optionalCommunityMarketString(snapshot.Data.Developer),
		HouseholdCount:                    snapshot.Data.HouseholdCount,
		ClosedManagement:                  optionalCommunityMarketString(snapshot.Data.ClosedManagement),
		PlotRatio:                         snapshot.Data.PlotRatio,
		GreenAreaSQM:                      snapshot.Data.GreenAreaSQM,
		GreeningRatePercent:               snapshot.Data.GreeningRatePercent,
		PropertyManagementCompany:         optionalCommunityMarketString(snapshot.Data.PropertyManagementCompany),
		PropertyFee:                       optionalCommunityMarketString(snapshot.Data.PropertyFee),
		FixedParkingSpaces:                snapshot.Data.FixedParkingSpaces,
		ParkingRatio:                      optionalCommunityMarketString(snapshot.Data.ParkingRatio),
		ParkingFee:                        optionalCommunityMarketString(snapshot.Data.ParkingFee),
		HeatingType:                       optionalCommunityMarketString(snapshot.Data.HeatingType),
		WaterType:                         optionalCommunityMarketString(snapshot.Data.WaterType),
		ElectricityType:                   optionalCommunityMarketString(snapshot.Data.ElectricityType),
		GasCost:                           optionalCommunityMarketString(snapshot.Data.GasCost),
		ManCarSeparation:                  optionalCommunityMarketString(snapshot.Data.ManCarSeparation),
		Latitude:                          snapshot.Data.Latitude,
		Longitude:                         snapshot.Data.Longitude,
		LatestListingDate:                 snapshot.Data.LatestListingDate,
		ListingAvgUnitPrice:               snapshot.Data.ListingAvgUnitPrice,
		ListingCount:                      snapshot.Data.ListingCount,
		ListingAreaSQM:                    snapshot.Data.ListingAreaSQM,
		ListingAvgTotalPriceWan:           snapshot.Data.ListingAvgTotalPriceWan,
		ListingAvgUnitPrice6Months:        snapshot.Data.ListingAvgUnitPrice6Months,
		NewListingCount3Months:            snapshot.Data.NewListingCount3Months,
		NewListingAvgTotalPrice3MonthsWan: snapshot.Data.NewListingAvgTotalPrice3MonthsWan,
		NewListingUnitPrice3Months:        snapshot.Data.NewListingUnitPrice3Months,
		LatestTradeDate:                   snapshot.Data.LatestTradeDate,
		LatestTradeAvgUnitPrice:           snapshot.Data.LatestTradeAvgUnitPrice,
		TradeCount3Months:                 snapshot.Data.TradeCount3Months,
		TradeArea3MonthsSQM:               snapshot.Data.TradeArea3MonthsSQM,
		TradeAvgTotalPrice3MonthsWan:      snapshot.Data.TradeAvgTotalPrice3MonthsWan,
		TradeUnitPrice3Months:             snapshot.Data.TradeUnitPrice3Months,
		TradeAvgUnitPrice6Months:          snapshot.Data.TradeAvgUnitPrice6Months,
		TradeCountPerMonth6Months:         snapshot.Data.TradeCountPerMonth6Months,
		TakeLookCount:                     snapshot.Data.TakeLookCount,
		TakeLookConversionRate:            snapshot.Data.TakeLookConversionRate,
		OnSaleAreaRange:                   snapshot.Data.OnSaleAreaRange,
		OnSalePriceRange:                  snapshot.Data.OnSalePriceRange,
		OnSaleRoomTypes:                   roomTypes,
		Analysis:                          jsonObjectOrEmpty(snapshot.Data.Analysis),
		Surroundings:                      jsonObjectOrEmpty(snapshot.Data.Surroundings),
		CityContext:                       jsonObjectOrEmpty(snapshot.Data.CityContext),
		QualityStatus:                     snapshotQualityStatus(snapshot),
		CreatedAt:                         snapshot.CreatedAt,
	}, nil
}

func communityMarketSnapshotFromModel(model CommunityMarketSnapshotModel) appcommunitymarket.Snapshot {
	roomTypes := make([]string, 0)
	_ = json.Unmarshal(model.OnSaleRoomTypes, &roomTypes)
	var propertyTags []string
	_ = json.Unmarshal(model.PropertyTags, &propertyTags)
	return appcommunitymarket.Snapshot{
		ID:              model.ID,
		DataSourceID:    model.DataSourceID,
		NeighborhoodID:  model.NeighborhoodID,
		SourceRef:       model.SourceRef,
		CollectedAt:     model.CollectedAt,
		ContentChecksum: model.ContentChecksum,
		RawPayload:      append([]byte(nil), model.RawPayload...),
		RawContentType:  model.RawContentType,
		CollectionRunID: model.CollectionRunID,
		QualityStatus:   model.QualityStatus,
		CreatedAt:       model.CreatedAt,
		Data: domaincommunitymarket.SnapshotData{
			SourceCommunityID:                 model.SourceCommunityID,
			CommunityName:                     model.CommunityName,
			FormerName:                        model.FormerName,
			ProvinceCode:                      communityMarketString(model.ProvinceCode),
			ProvinceName:                      communityMarketString(model.ProvinceName),
			CityCode:                          model.CityCode,
			CityName:                          model.CityName,
			DistrictCode:                      model.DistrictCode,
			DistrictName:                      model.DistrictName,
			BlockCode:                         model.BlockCode,
			BlockName:                         model.BlockName,
			PropertyType:                      communityMarketString(model.PropertyType),
			PropertyTags:                      propertyTags,
			BuildingCount:                     model.BuildingCount,
			BuildingType:                      communityMarketString(model.BuildingType),
			BuildingYear:                      model.BuildingYear,
			Developer:                         communityMarketString(model.Developer),
			HouseholdCount:                    model.HouseholdCount,
			ClosedManagement:                  communityMarketString(model.ClosedManagement),
			PlotRatio:                         model.PlotRatio,
			GreenAreaSQM:                      model.GreenAreaSQM,
			GreeningRatePercent:               model.GreeningRatePercent,
			PropertyManagementCompany:         communityMarketString(model.PropertyManagementCompany),
			PropertyFee:                       communityMarketString(model.PropertyFee),
			FixedParkingSpaces:                model.FixedParkingSpaces,
			ParkingRatio:                      communityMarketString(model.ParkingRatio),
			ParkingFee:                        communityMarketString(model.ParkingFee),
			HeatingType:                       communityMarketString(model.HeatingType),
			WaterType:                         communityMarketString(model.WaterType),
			ElectricityType:                   communityMarketString(model.ElectricityType),
			GasCost:                           communityMarketString(model.GasCost),
			ManCarSeparation:                  communityMarketString(model.ManCarSeparation),
			Latitude:                          model.Latitude,
			Longitude:                         model.Longitude,
			LatestListingDate:                 model.LatestListingDate,
			ListingAvgUnitPrice:               model.ListingAvgUnitPrice,
			ListingCount:                      model.ListingCount,
			ListingAreaSQM:                    model.ListingAreaSQM,
			ListingAvgTotalPriceWan:           model.ListingAvgTotalPriceWan,
			ListingAvgUnitPrice6Months:        model.ListingAvgUnitPrice6Months,
			NewListingCount3Months:            model.NewListingCount3Months,
			NewListingAvgTotalPrice3MonthsWan: model.NewListingAvgTotalPrice3MonthsWan,
			NewListingUnitPrice3Months:        model.NewListingUnitPrice3Months,
			LatestTradeDate:                   model.LatestTradeDate,
			LatestTradeAvgUnitPrice:           model.LatestTradeAvgUnitPrice,
			TradeCount3Months:                 model.TradeCount3Months,
			TradeArea3MonthsSQM:               model.TradeArea3MonthsSQM,
			TradeAvgTotalPrice3MonthsWan:      model.TradeAvgTotalPrice3MonthsWan,
			TradeUnitPrice3Months:             model.TradeUnitPrice3Months,
			TradeAvgUnitPrice6Months:          model.TradeAvgUnitPrice6Months,
			TradeCountPerMonth6Months:         model.TradeCountPerMonth6Months,
			TakeLookCount:                     model.TakeLookCount,
			TakeLookConversionRate:            model.TakeLookConversionRate,
			OnSaleAreaRange:                   model.OnSaleAreaRange,
			OnSalePriceRange:                  model.OnSalePriceRange,
			OnSaleRoomTypes:                   roomTypes,
			Analysis:                          jsonObjectOrEmpty(model.Analysis),
			Surroundings:                      jsonObjectOrEmpty(model.Surroundings),
			CityContext:                       jsonObjectOrEmpty(model.CityContext),
		},
	}
}

func optionalCommunityMarketString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func communityMarketString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func jsonObjectOrEmpty(value json.RawMessage) json.RawMessage {
	if len(value) == 0 {
		return json.RawMessage(`{}`)
	}
	return append(json.RawMessage(nil), value...)
}

func snapshotQualityStatus(snapshot appcommunitymarket.Snapshot) string {
	if snapshot.QualityStatus != "" {
		return snapshot.QualityStatus
	}
	return "aggregate_only"
}

func formatMarketFloat(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}

func marketFloat(value string) float64 {
	parsed, _ := strconv.ParseFloat(value, 64)
	return parsed
}

func marketInt(value string) int {
	parsed, _ := strconv.Atoi(value)
	return parsed
}

func marketTime(value string) time.Time {
	parsed, _ := time.Parse(time.RFC3339, value)
	return parsed
}
