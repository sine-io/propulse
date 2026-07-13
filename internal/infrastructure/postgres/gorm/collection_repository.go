package gormrepo

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	appcollection "github.com/sine-io/propulse/internal/application/collection"
	domainneighborhood "github.com/sine-io/propulse/internal/domain/neighborhood"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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

func (r *CollectionRepository) CreateDataSource(ctx context.Context, source appcollection.DataSource) (appcollection.DataSource, error) {
	model := dataSourceModel(source)
	if model.ID == "" {
		model.ID = uuid.NewString()
	}
	err := r.db.WithContext(ctx).
		Clauses(
			clause.OnConflict{
				Columns:   []clause.Column{{Name: "name"}, {Name: "city"}},
				DoNothing: true,
			},
			clause.Returning{},
		).
		Create(&model).Error
	if err != nil {
		return appcollection.DataSource{}, err
	}

	var saved DataSourceModel
	if err := r.db.WithContext(ctx).Where("name = ? AND city = ?", model.Name, model.City).First(&saved).Error; err != nil {
		return appcollection.DataSource{}, err
	}
	return dataSourceFromModel(saved), nil
}

func (r *CollectionRepository) ListDataSources(ctx context.Context) ([]appcollection.DataSource, error) {
	var models []DataSourceModel
	if err := r.db.WithContext(ctx).Order("city ASC, name ASC, id ASC").Find(&models).Error; err != nil {
		return nil, err
	}
	sources := make([]appcollection.DataSource, 0, len(models))
	for _, model := range models {
		sources = append(sources, dataSourceFromModel(model))
	}
	return sources, nil
}

func (r *CollectionRepository) DataSourceExists(ctx context.Context, id string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&DataSourceModel{}).
		Where("id = ?", id).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *CollectionRepository) SaveCollectionRun(ctx context.Context, batch appcollection.ImportBatch) (appcollection.SaveCollectionRunResult, error) {
	var result appcollection.SaveCollectionRunResult
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		run := collectionRunModel(batch.Run)
		create := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&run)
		if create.Error != nil {
			return create.Error
		}
		if create.RowsAffected == 0 {
			var existing CollectionRunModel
			if err := tx.Where("data_source_id = ? AND source_ref = ? AND content_checksum = ?", run.DataSourceID, run.SourceRef, run.ContentChecksum).First(&existing).Error; err != nil {
				return err
			}
			result = appcollection.SaveCollectionRunResult{Run: collectionRunFromModel(existing), Created: false}
			return nil
		}

		listings, err := listingObservationModels(batch.Run, batch.Listings)
		if err != nil {
			return err
		}
		transactions := transactionObservationModels(batch.Run, batch.Transactions)
		if len(listings) > 0 {
			if err := tx.Create(&listings).Error; err != nil {
				return err
			}
		}
		if len(transactions) > 0 {
			if err := tx.Create(&transactions).Error; err != nil {
				return err
			}
		}
		result = appcollection.SaveCollectionRunResult{Run: collectionRunFromModel(run), Created: true}
		return nil
	})
	return result, err
}

func (r *CollectionRepository) GetCollectionRun(ctx context.Context, id string) (appcollection.CollectionRunDetail, error) {
	var run CollectionRunModel
	if err := r.db.WithContext(ctx).First(&run, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return appcollection.CollectionRunDetail{}, appcollection.ErrCollectionRunNotFound
		}
		return appcollection.CollectionRunDetail{}, err
	}

	var source DataSourceModel
	if err := r.db.WithContext(ctx).First(&source, "id = ?", run.DataSourceID).Error; err != nil {
		return appcollection.CollectionRunDetail{}, err
	}

	var listings []ListingObservationModel
	if err := r.db.WithContext(ctx).Where("collection_run_id = ?", run.ID).Order("source_row ASC, id ASC").Find(&listings).Error; err != nil {
		return appcollection.CollectionRunDetail{}, err
	}
	var transactions []TransactionObservationModel
	if err := r.db.WithContext(ctx).Where("collection_run_id = ?", run.ID).Order("source_row ASC, id ASC").Find(&transactions).Error; err != nil {
		return appcollection.CollectionRunDetail{}, err
	}

	return appcollection.CollectionRunDetail{
		Run:          collectionRunFromModel(run),
		Source:       dataSourceFromModel(source),
		Listings:     listingObservationsFromModels(listings),
		Transactions: transactionObservationsFromModels(transactions),
	}, nil
}

func (r *CollectionRepository) ListMetricRefreshCandidates(ctx context.Context, updatedBefore time.Time, limit int) ([]appcollection.MetricRefreshCandidate, error) {
	var runs []CollectionRunModel
	if err := r.db.WithContext(ctx).
		Select("id", "neighborhood_id").
		Where("metric_status IN ?", []string{string(appcollection.MetricStatusPending), string(appcollection.MetricStatusFailed)}).
		Where("updated_at <= ?", updatedBefore).
		Order("updated_at ASC, id ASC").
		Limit(limit).
		Find(&runs).Error; err != nil {
		return nil, err
	}

	candidates := make([]appcollection.MetricRefreshCandidate, 0, len(runs))
	for _, run := range runs {
		candidates = append(candidates, appcollection.MetricRefreshCandidate{
			CollectionRunID: run.ID,
			NeighborhoodID:  run.NeighborhoodID,
		})
	}
	return candidates, nil
}

func (r *CollectionRepository) UpdateMetricStatus(ctx context.Context, id string, status appcollection.MetricStatus) error {
	updated := r.db.WithContext(ctx).
		Model(&CollectionRunModel{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"metric_status": string(status),
			"updated_at":    time.Now().UTC(),
		})
	if updated.Error != nil {
		return updated.Error
	}
	if updated.RowsAffected == 0 {
		return appcollection.ErrCollectionRunNotFound
	}
	return nil
}

func dataSourceModel(source appcollection.DataSource) DataSourceModel {
	return DataSourceModel{
		ID:         source.ID,
		Name:       source.Name,
		SourceType: source.SourceType,
		City:       source.City,
		Notes:      source.Notes,
		CreatedAt:  source.CreatedAt,
		UpdatedAt:  source.UpdatedAt,
	}
}

func dataSourceFromModel(model DataSourceModel) appcollection.DataSource {
	return appcollection.DataSource{
		ID:         model.ID,
		Name:       model.Name,
		SourceType: model.SourceType,
		City:       model.City,
		Notes:      model.Notes,
		CreatedAt:  model.CreatedAt,
		UpdatedAt:  model.UpdatedAt,
	}
}

func collectionRunModel(run appcollection.CollectionRun) CollectionRunModel {
	validationSummary := run.ValidationSummary
	if validationSummary.Issues == nil {
		validationSummary.Issues = []appcollection.ValidationIssue{}
	}
	validationBytes, _ := json.Marshal(validationSummary)
	status := run.Status
	if status == "" {
		status = appcollection.CollectionRunStatusCompleted
	}
	metricStatus := run.MetricStatus
	if metricStatus == "" {
		metricStatus = appcollection.MetricStatusPending
	}
	return CollectionRunModel{
		ID:                run.ID,
		DataSourceID:      run.DataSourceID,
		NeighborhoodID:    run.NeighborhoodID,
		SourceRef:         run.SourceRef,
		CollectedAt:       run.CollectedAt,
		Coverage:          string(run.Coverage),
		ImportFormat:      string(run.Format),
		ContentChecksum:   run.ContentChecksum,
		RawPayload:        append([]byte(nil), run.RawPayload...),
		RawContentType:    run.RawContentType,
		ValidationSummary: validationBytes,
		Status:            string(status),
		MetricStatus:      string(metricStatus),
		CreatedAt:         run.CreatedAt,
		UpdatedAt:         run.UpdatedAt,
	}
}

func collectionRunFromModel(model CollectionRunModel) appcollection.CollectionRun {
	var validationSummary appcollection.ValidationSummary
	_ = json.Unmarshal(model.ValidationSummary, &validationSummary)
	if validationSummary.Issues == nil {
		validationSummary.Issues = []appcollection.ValidationIssue{}
	}
	return appcollection.CollectionRun{
		ID:                model.ID,
		DataSourceID:      model.DataSourceID,
		NeighborhoodID:    model.NeighborhoodID,
		SourceRef:         model.SourceRef,
		CollectedAt:       model.CollectedAt,
		Coverage:          domainneighborhood.Coverage(model.Coverage),
		Format:            appcollection.ImportFormat(model.ImportFormat),
		ContentChecksum:   model.ContentChecksum,
		RawPayload:        append([]byte(nil), model.RawPayload...),
		RawContentType:    model.RawContentType,
		ValidationSummary: validationSummary,
		Status:            appcollection.CollectionRunStatus(model.Status),
		MetricStatus:      appcollection.MetricStatus(model.MetricStatus),
		CreatedAt:         model.CreatedAt,
		UpdatedAt:         model.UpdatedAt,
	}
}

func marshalAttributes(attributes map[string]string) ([]byte, error) {
	if attributes == nil {
		attributes = map[string]string{}
	}
	return json.Marshal(attributes)
}

func attributesFromJSON(raw []byte, target *map[string]string) error {
	if len(raw) == 0 {
		*target = map[string]string{}
		return nil
	}
	return json.Unmarshal(raw, target)
}

func listingObservationModels(run appcollection.CollectionRun, observations []appcollection.ListingObservation) ([]ListingObservationModel, error) {
	models := make([]ListingObservationModel, 0, len(observations))
	for _, observation := range observations {
		attributes, err := marshalAttributes(observation.Attributes)
		if err != nil {
			return nil, err
		}
		models = append(models, ListingObservationModel{
			ID:              observation.ID,
			CollectionRunID: run.ID,
			NeighborhoodID:  run.NeighborhoodID,
			SourceListingID: observation.SourceListingID,
			SourceRow:       observation.SourceRow,
			Layout:          observation.Layout,
			AreaSQM:         observation.AreaSQM,
			ListingPrice:    observation.ListingPrice,
			DaysOnMarket:    observation.DaysOnMarket,
			Status:          string(observation.Status),
			CapturedAt:      observation.CapturedAt,
			Attributes:      attributes,
		})
	}
	return models, nil
}

func transactionObservationModels(run appcollection.CollectionRun, observations []appcollection.TransactionObservation) []TransactionObservationModel {
	models := make([]TransactionObservationModel, 0, len(observations))
	for _, observation := range observations {
		models = append(models, TransactionObservationModel{
			ID:                 observation.ID,
			CollectionRunID:    run.ID,
			NeighborhoodID:     run.NeighborhoodID,
			SourceRecordID:     observation.SourceRecordID,
			SourceRow:          observation.SourceRow,
			Layout:             observation.Layout,
			AreaSQM:            observation.AreaSQM,
			TransactionPrice:   observation.TransactionPrice,
			TransactionDate:    observation.TransactionDate,
			OriginalListingRef: observation.OriginalListingRef,
			CapturedAt:         observation.CapturedAt,
		})
	}
	return models
}

func listingObservationsFromModels(models []ListingObservationModel) []appcollection.ListingObservation {
	observations := make([]appcollection.ListingObservation, 0, len(models))
	for _, model := range models {
		attributes := map[string]string{}
		_ = attributesFromJSON(model.Attributes, &attributes)
		observations = append(observations, appcollection.ListingObservation{
			ID:              model.ID,
			CollectionRunID: model.CollectionRunID,
			NeighborhoodID:  model.NeighborhoodID,
			SourceListingID: model.SourceListingID,
			SourceRow:       model.SourceRow,
			Layout:          model.Layout,
			AreaSQM:         model.AreaSQM,
			ListingPrice:    model.ListingPrice,
			DaysOnMarket:    model.DaysOnMarket,
			Status:          appcollection.ListingStatus(model.Status),
			CapturedAt:      model.CapturedAt,
			Attributes:      attributes,
		})
	}
	return observations
}

func transactionObservationsFromModels(models []TransactionObservationModel) []appcollection.TransactionObservation {
	observations := make([]appcollection.TransactionObservation, 0, len(models))
	for _, model := range models {
		observations = append(observations, appcollection.TransactionObservation{
			ID:                 model.ID,
			CollectionRunID:    model.CollectionRunID,
			NeighborhoodID:     model.NeighborhoodID,
			SourceRecordID:     model.SourceRecordID,
			SourceRow:          model.SourceRow,
			Layout:             model.Layout,
			AreaSQM:            model.AreaSQM,
			TransactionPrice:   model.TransactionPrice,
			TransactionDate:    model.TransactionDate,
			OriginalListingRef: model.OriginalListingRef,
			CapturedAt:         model.CapturedAt,
		})
	}
	return observations
}
