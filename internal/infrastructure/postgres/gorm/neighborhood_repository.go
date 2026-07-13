package gormrepo

import (
	"context"
	"errors"

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
}

func NewNeighborhoodRepository(db *gorm.DB) *NeighborhoodRepository {
	return &NeighborhoodRepository{db: db}
}

func NewNeighborhoodRepositoryWithMetricReader(db *gorm.DB, metricReader metricReader) *NeighborhoodRepository {
	return &NeighborhoodRepository{
		db:           db,
		metricReader: metricReader,
	}
}

func (r *NeighborhoodRepository) CreateNeighborhood(ctx context.Context, input appneighborhood.CreateNeighborhoodInput) (appneighborhood.Neighborhood, error) {
	var existing NeighborhoodModel
	err := r.db.WithContext(ctx).
		Where("name = ? AND area = ? AND target_layout = ?", input.Name, input.Area, input.TargetLayout).
		First(&existing).Error
	if err == nil {
		return neighborhoodFromModel(existing), nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return appneighborhood.Neighborhood{}, err
	}

	id := input.ID
	if id == "" {
		id = uuid.NewString()
	}
	model := NeighborhoodModel{
		ID:           id,
		Name:         input.Name,
		Area:         input.Area,
		TargetLayout: input.TargetLayout,
	}
	if err := r.db.WithContext(ctx).Create(&model).Error; err != nil {
		return appneighborhood.Neighborhood{}, err
	}
	return neighborhoodFromModel(model), nil
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
	return neighborhoodFromModel(model), nil
}

func (r *NeighborhoodRepository) AddWatchlistItem(ctx context.Context, userID string, neighborhoodID string) (appneighborhood.WatchlistItem, error) {
	if _, err := r.GetNeighborhood(ctx, neighborhoodID); err != nil {
		return appneighborhood.WatchlistItem{}, err
	}

	model := WatchlistItemModel{
		ID:             uuid.NewString(),
		UserID:         userID,
		NeighborhoodID: neighborhoodID,
	}
	err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "user_id"}, {Name: "neighborhood_id"}},
			DoNothing: true,
		}).
		Create(&model).Error
	if err != nil {
		return appneighborhood.WatchlistItem{}, err
	}

	var saved WatchlistItemModel
	if err := r.db.WithContext(ctx).
		First(&saved, "user_id = ? AND neighborhood_id = ?", userID, neighborhoodID).Error; err != nil {
		return appneighborhood.WatchlistItem{}, err
	}

	return watchlistItemFromModel(saved), nil
}

func (r *NeighborhoodRepository) ListWatchlist(ctx context.Context, userID string) ([]appneighborhood.WatchlistSummary, error) {
	var rows []struct {
		WatchlistItemModel
		Name         string `gorm:"column:name"`
		Area         string `gorm:"column:area"`
		TargetLayout string `gorm:"column:target_layout"`
	}

	err := r.db.WithContext(ctx).
		Table("watchlist_items").
		Select("watchlist_items.*, neighborhoods.name, neighborhoods.area, neighborhoods.target_layout").
		Joins("JOIN neighborhoods ON neighborhoods.id = watchlist_items.neighborhood_id").
		Where("watchlist_items.user_id = ?", userID).
		Order("watchlist_items.created_at ASC").
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

func neighborhoodFromModel(model NeighborhoodModel) appneighborhood.Neighborhood {
	return appneighborhood.Neighborhood{
		ID:           model.ID,
		Name:         model.Name,
		Area:         model.Area,
		TargetLayout: model.TargetLayout,
		CreatedAt:    model.CreatedAt,
	}
}

func watchlistItemFromModel(model WatchlistItemModel) appneighborhood.WatchlistItem {
	return appneighborhood.WatchlistItem{
		ID:             model.ID,
		UserID:         model.UserID,
		NeighborhoodID: model.NeighborhoodID,
		CreatedAt:      model.CreatedAt,
	}
}
