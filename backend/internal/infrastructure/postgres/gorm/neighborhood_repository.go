package gormrepo

import (
	"context"
	"errors"

	"github.com/google/uuid"
	appneighborhood "github.com/propulse/propulse/backend/internal/application/neighborhood"
	domainneighborhood "github.com/propulse/propulse/backend/internal/domain/neighborhood"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const demoUserID = "demo-user"

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

func (r *NeighborhoodRepository) ListWatchlistNeighborhoodIDs(ctx context.Context) ([]string, error) {
	var neighborhoodIDs []string
	err := r.db.WithContext(ctx).
		Model(&WatchlistItemModel{}).
		Distinct("neighborhood_id").
		Order("neighborhood_id ASC").
		Pluck("neighborhood_id", &neighborhoodIDs).Error
	if err != nil {
		return nil, err
	}
	return neighborhoodIDs, nil
}

func (r *NeighborhoodRepository) LatestMetric(ctx context.Context, neighborhoodID string) (appneighborhood.MetricSnapshot, error) {
	if r.metricReader != nil {
		return r.metricReader.LatestMetric(ctx, neighborhoodID)
	}

	return r.latestMetricFromGORM(ctx, neighborhoodID)
}

func (r *NeighborhoodRepository) latestMetricFromGORM(ctx context.Context, neighborhoodID string) (appneighborhood.MetricSnapshot, error) {
	var model NeighborhoodMetricModel
	err := r.db.WithContext(ctx).
		Where("neighborhood_id = ?", neighborhoodID).
		Order("calculated_at DESC").
		First(&model).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return appneighborhood.MetricSnapshot{}, appneighborhood.ErrMetricNotFound
		}
		return appneighborhood.MetricSnapshot{}, err
	}
	return metricFromModel(model), nil
}

func (r *NeighborhoodRepository) SeedDemoData(ctx context.Context) error {
	seeds := []struct {
		neighborhood appneighborhood.CreateNeighborhoodInput
		metric       NeighborhoodMetricModel
	}{
		{
			neighborhood: appneighborhood.CreateNeighborhoodInput{Name: "青枫花园", Area: "滨江核心", TargetLayout: "三房"},
			metric: NeighborhoodMetricModel{
				ListedHomes:         42,
				PriceCutHomes:       11,
				AvgDaysOnMarket:     78,
				ListingPriceMin:     520,
				ListingPriceMax:     620,
				TransactionPriceMin: 495,
				TransactionPriceMax: 545,
				TransactionMomentum: string(domainneighborhood.TransactionMomentumWeak),
				TargetLayoutSupply:  12,
			},
		},
		{
			neighborhood: appneighborhood.CreateNeighborhoodInput{Name: "云澜府", Area: "城东新区", TargetLayout: "四房"},
			metric: NeighborhoodMetricModel{
				ListedHomes:         14,
				PriceCutHomes:       1,
				AvgDaysOnMarket:     35,
				ListingPriceMin:     700,
				ListingPriceMax:     760,
				TransactionPriceMin: 690,
				TransactionPriceMax: 745,
				TransactionMomentum: string(domainneighborhood.TransactionMomentumStrong),
				TargetLayoutSupply:  3,
			},
		},
	}

	for _, seed := range seeds {
		neighborhood, err := r.CreateNeighborhood(ctx, seed.neighborhood)
		if err != nil {
			return err
		}
		if _, err := r.AddWatchlistItem(ctx, demoUserID, neighborhood.ID); err != nil {
			return err
		}

		var count int64
		if err := r.db.WithContext(ctx).
			Model(&NeighborhoodMetricModel{}).
			Where("neighborhood_id = ?", neighborhood.ID).
			Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			continue
		}
		metric := seed.metric
		metric.ID = uuid.NewString()
		metric.NeighborhoodID = neighborhood.ID
		if err := r.db.WithContext(ctx).Create(&metric).Error; err != nil {
			return err
		}
	}

	return nil
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

func metricFromModel(model NeighborhoodMetricModel) appneighborhood.MetricSnapshot {
	return appneighborhood.MetricSnapshot{
		ID:                  model.ID,
		NeighborhoodID:      model.NeighborhoodID,
		ListedHomes:         model.ListedHomes,
		PriceCutHomes:       model.PriceCutHomes,
		AvgDaysOnMarket:     model.AvgDaysOnMarket,
		ListingPriceMin:     model.ListingPriceMin,
		ListingPriceMax:     model.ListingPriceMax,
		TransactionPriceMin: model.TransactionPriceMin,
		TransactionPriceMax: model.TransactionPriceMax,
		TransactionMomentum: domainneighborhood.TransactionMomentum(model.TransactionMomentum),
		TargetLayoutSupply:  model.TargetLayoutSupply,
		CalculatedAt:        model.CalculatedAt,
	}
}
