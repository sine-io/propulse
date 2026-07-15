package gormrepo

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	appneighborhood "github.com/sine-io/propulse/internal/application/neighborhood"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type NeighborhoodRepository struct {
	db           *gorm.DB
	metricReader metricReader
}

type metricReader interface {
	LatestMetric(ctx context.Context, neighborhoodID string) (appneighborhood.MetricSnapshot, error)
	ListMetricHistory(ctx context.Context, query appneighborhood.MetricHistoryRepositoryQuery) ([]appneighborhood.MetricHistoryRecord, error)
}

func NewNeighborhoodRepository(db *gorm.DB) *NeighborhoodRepository {
	return &NeighborhoodRepository{db: db}
}

func NewNeighborhoodRepositoryWithMetricReader(db *gorm.DB, metricReader metricReader) *NeighborhoodRepository {
	return &NeighborhoodRepository{db: db, metricReader: metricReader}
}

func (r *NeighborhoodRepository) CreateNeighborhood(ctx context.Context, input appneighborhood.CreateNeighborhoodInput) (appneighborhood.Neighborhood, error) {
	var result appneighborhood.Neighborhood
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var model NeighborhoodModel
		err := tx.Where("name = ? AND city = ? AND area = ?", input.Name, input.City, input.Area).First(&model).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			id := input.ID
			if id == "" {
				id = uuid.NewString()
			}
			city := input.City
			model = NeighborhoodModel{ID: id, Name: input.Name, City: &city, Area: input.Area}
			if err := tx.Create(&model).Error; err != nil {
				return err
			}
		} else if err != nil {
			return err
		}

		layouts := make([]NeighborhoodLayoutModel, 0, len(input.AvailableLayouts))
		for _, layout := range input.AvailableLayouts {
			layouts = append(layouts, NeighborhoodLayoutModel{NeighborhoodID: model.ID, Layout: layout})
		}
		if len(layouts) > 0 {
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&layouts).Error; err != nil {
				return err
			}
		}

		loaded, err := neighborhoodFromModel(ctx, tx, model)
		if err != nil {
			return err
		}
		result = loaded
		return nil
	})
	return result, err
}

func (r *NeighborhoodRepository) GetNeighborhood(ctx context.Context, id string) (appneighborhood.Neighborhood, error) {
	var model NeighborhoodModel
	err := r.db.WithContext(ctx).First(&model, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return appneighborhood.Neighborhood{}, appneighborhood.ErrNeighborhoodNotFound
		}
		return appneighborhood.Neighborhood{}, err
	}
	return neighborhoodFromModel(ctx, r.db, model)
}

func (r *NeighborhoodRepository) SearchNeighborhoods(ctx context.Context, input appneighborhood.SearchNeighborhoodsInput) (appneighborhood.SearchNeighborhoodsResult, error) {
	query := r.db.WithContext(ctx).
		Model(&NeighborhoodModel{}).
		Where("city IS NOT NULL").
		Where("EXISTS (SELECT 1 FROM neighborhood_layouts nl WHERE nl.neighborhood_id = neighborhoods.id)")

	if value := strings.TrimSpace(input.Query); value != "" {
		query = query.Where("name ILIKE ?", "%"+value+"%")
	}
	if city := strings.TrimSpace(input.City); city != "" {
		query = query.Where("city = ?", city)
	}
	if area := strings.TrimSpace(input.Area); area != "" {
		query = query.Where("area = ?", area)
	}
	if layout := strings.TrimSpace(input.TargetLayout); layout != "" {
		query = query.Where("EXISTS (SELECT 1 FROM neighborhood_layouts nl WHERE nl.neighborhood_id = neighborhoods.id AND nl.layout = ?)", layout)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return appneighborhood.SearchNeighborhoodsResult{}, err
	}

	var models []NeighborhoodModel
	if err := query.
		Order("city ASC, area ASC, name ASC, id ASC").
		Limit(input.Limit).
		Offset(input.Offset).
		Find(&models).Error; err != nil {
		return appneighborhood.SearchNeighborhoodsResult{}, err
	}
	layouts, err := layoutsByNeighborhood(ctx, r.db, neighborhoodModelIDs(models))
	if err != nil {
		return appneighborhood.SearchNeighborhoodsResult{}, err
	}
	items := make([]appneighborhood.Neighborhood, 0, len(models))
	for _, model := range models {
		items = append(items, neighborhoodFromLoadedModel(model, layouts[model.ID]))
	}

	filters, err := neighborhoodSearchFilters(ctx, r.db)
	if err != nil {
		return appneighborhood.SearchNeighborhoodsResult{}, err
	}
	return appneighborhood.SearchNeighborhoodsResult{Items: items, Total: int(total), Filters: filters}, nil
}

func (r *NeighborhoodRepository) AddWatchlistItem(ctx context.Context, userID string, neighborhoodID string, targetLayout string) (appneighborhood.WatchlistItem, error) {
	neighborhood, err := r.GetNeighborhood(ctx, neighborhoodID)
	if err != nil {
		return appneighborhood.WatchlistItem{}, err
	}
	validLayout := false
	for _, layout := range neighborhood.AvailableLayouts {
		if layout == targetLayout {
			validLayout = true
			break
		}
	}
	if !validLayout {
		return appneighborhood.WatchlistItem{}, appneighborhood.ErrInvalidTargetLayout
	}

	model := WatchlistItemModel{
		ID:             uuid.NewString(),
		UserID:         userID,
		NeighborhoodID: neighborhoodID,
		TargetLayout:   targetLayout,
	}
	created := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "user_id"}, {Name: "neighborhood_id"}},
			DoNothing: true,
		}).
		Create(&model)
	if created.Error != nil {
		return appneighborhood.WatchlistItem{}, created.Error
	}
	if created.RowsAffected == 0 {
		return appneighborhood.WatchlistItem{}, appneighborhood.ErrWatchlistItemExists
	}
	return watchlistItemFromModel(model), nil
}

func (r *NeighborhoodRepository) ListWatchlist(ctx context.Context, userID string) ([]appneighborhood.WatchlistSummary, error) {
	var rows []struct {
		WatchlistItemModel
		Name string  `gorm:"column:name"`
		City *string `gorm:"column:city"`
		Area string  `gorm:"column:area"`
	}

	err := r.db.WithContext(ctx).
		Table("watchlist_items").
		Select("watchlist_items.*, neighborhoods.name, neighborhoods.city, neighborhoods.area").
		Joins("JOIN neighborhoods ON neighborhoods.id = watchlist_items.neighborhood_id").
		Where("watchlist_items.user_id = ?", userID).
		Order("watchlist_items.created_at ASC, watchlist_items.id ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	items := make([]appneighborhood.WatchlistSummary, 0, len(rows))
	for _, row := range rows {
		metric, err := r.LatestMetric(ctx, row.NeighborhoodID)
		hasMetric := true
		if err != nil && !errors.Is(err, appneighborhood.ErrMetricNotFound) {
			return nil, err
		}
		if errors.Is(err, appneighborhood.ErrMetricNotFound) {
			hasMetric = false
		}
		items = append(items, appneighborhood.WatchlistSummary{
			ID:             row.ID,
			NeighborhoodID: row.NeighborhoodID,
			Name:           row.Name,
			City:           row.City,
			Area:           row.Area,
			TargetLayout:   row.TargetLayout,
			HasMetric:      hasMetric,
			Metric:         metric,
		})
	}
	return items, nil
}

func (r *NeighborhoodRepository) LatestMetric(ctx context.Context, neighborhoodID string) (appneighborhood.MetricSnapshot, error) {
	if r.metricReader == nil {
		return appneighborhood.MetricSnapshot{}, appneighborhood.ErrMetricNotFound
	}
	return r.metricReader.LatestMetric(ctx, neighborhoodID)
}

func (r *NeighborhoodRepository) ListMetricHistory(ctx context.Context, query appneighborhood.MetricHistoryRepositoryQuery) ([]appneighborhood.MetricHistoryRecord, error) {
	if r.metricReader == nil {
		return []appneighborhood.MetricHistoryRecord{}, nil
	}
	return r.metricReader.ListMetricHistory(ctx, query)
}

func neighborhoodFromModel(ctx context.Context, db *gorm.DB, model NeighborhoodModel) (appneighborhood.Neighborhood, error) {
	layouts, err := layoutsByNeighborhood(ctx, db, []string{model.ID})
	if err != nil {
		return appneighborhood.Neighborhood{}, err
	}
	return neighborhoodFromLoadedModel(model, layouts[model.ID]), nil
}

func neighborhoodFromLoadedModel(model NeighborhoodModel, layouts []string) appneighborhood.Neighborhood {
	return appneighborhood.Neighborhood{
		ID:               model.ID,
		Name:             model.Name,
		City:             model.City,
		Area:             model.Area,
		AvailableLayouts: append([]string(nil), layouts...),
		CreatedAt:        model.CreatedAt,
	}
}

func layoutsByNeighborhood(ctx context.Context, db *gorm.DB, neighborhoodIDs []string) (map[string][]string, error) {
	result := make(map[string][]string, len(neighborhoodIDs))
	if len(neighborhoodIDs) == 0 {
		return result, nil
	}
	var models []NeighborhoodLayoutModel
	if err := db.WithContext(ctx).
		Where("neighborhood_id IN ?", neighborhoodIDs).
		Order("neighborhood_id ASC, layout ASC").
		Find(&models).Error; err != nil {
		return nil, err
	}
	for _, model := range models {
		result[model.NeighborhoodID] = append(result[model.NeighborhoodID], model.Layout)
	}
	return result, nil
}

func neighborhoodModelIDs(models []NeighborhoodModel) []string {
	ids := make([]string, 0, len(models))
	for _, model := range models {
		ids = append(ids, model.ID)
	}
	return ids
}

func neighborhoodSearchFilters(ctx context.Context, db *gorm.DB) (appneighborhood.NeighborhoodSearchFilters, error) {
	var rows []struct {
		City string `gorm:"column:city"`
		Area string `gorm:"column:area"`
	}
	if err := db.WithContext(ctx).
		Table("neighborhoods AS n").
		Distinct("n.city", "n.area").
		Where("n.city IS NOT NULL").
		Where("EXISTS (SELECT 1 FROM neighborhood_layouts nl WHERE nl.neighborhood_id = n.id)").
		Order("n.city ASC, n.area ASC").
		Scan(&rows).Error; err != nil {
		return appneighborhood.NeighborhoodSearchFilters{}, err
	}
	filters := appneighborhood.NeighborhoodSearchFilters{
		Cities: []string{},
		Areas:  make([]appneighborhood.NeighborhoodAreaFilter, 0, len(rows)),
	}
	lastCity := ""
	for _, row := range rows {
		if row.City != lastCity {
			filters.Cities = append(filters.Cities, row.City)
			lastCity = row.City
		}
		filters.Areas = append(filters.Areas, appneighborhood.NeighborhoodAreaFilter{City: row.City, Area: row.Area})
	}
	return filters, nil
}

func watchlistItemFromModel(model WatchlistItemModel) appneighborhood.WatchlistItem {
	return appneighborhood.WatchlistItem{
		ID:             model.ID,
		UserID:         model.UserID,
		NeighborhoodID: model.NeighborhoodID,
		TargetLayout:   model.TargetLayout,
		CreatedAt:      model.CreatedAt,
	}
}
