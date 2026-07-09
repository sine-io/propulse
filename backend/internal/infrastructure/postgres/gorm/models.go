package gormrepo

import (
	"time"

	"encoding/json"
)

type CapacityCalculationModel struct {
	ID        string          `gorm:"column:id;type:uuid;primaryKey"`
	Input     json.RawMessage `gorm:"column:input;type:jsonb;not null"`
	Result    json.RawMessage `gorm:"column:result;type:jsonb;not null"`
	CreatedAt time.Time       `gorm:"column:created_at;autoCreateTime"`
}

func (CapacityCalculationModel) TableName() string {
	return "capacity_calculations"
}

type NeighborhoodModel struct {
	ID           string    `gorm:"column:id;type:uuid;primaryKey"`
	Name         string    `gorm:"column:name"`
	Area         string    `gorm:"column:area"`
	TargetLayout string    `gorm:"column:target_layout"`
	CreatedAt    time.Time `gorm:"column:created_at;autoCreateTime"`
}

func (NeighborhoodModel) TableName() string {
	return "neighborhoods"
}

type WatchlistItemModel struct {
	ID             string    `gorm:"column:id;type:uuid;primaryKey"`
	NeighborhoodID string    `gorm:"column:neighborhood_id;type:uuid"`
	UserID         string    `gorm:"column:user_id"`
	CreatedAt      time.Time `gorm:"column:created_at;autoCreateTime"`
}

func (WatchlistItemModel) TableName() string {
	return "watchlist_items"
}

type NeighborhoodMetricModel struct {
	ID                  string    `gorm:"column:id;type:uuid;primaryKey"`
	NeighborhoodID      string    `gorm:"column:neighborhood_id;type:uuid"`
	ListedHomes         int       `gorm:"column:listed_homes"`
	PriceCutHomes       int       `gorm:"column:price_cut_homes"`
	AvgDaysOnMarket     float64   `gorm:"column:avg_days_on_market"`
	ListingPriceMin     float64   `gorm:"column:listing_price_min"`
	ListingPriceMax     float64   `gorm:"column:listing_price_max"`
	TransactionPriceMin float64   `gorm:"column:transaction_price_min"`
	TransactionPriceMax float64   `gorm:"column:transaction_price_max"`
	TransactionMomentum string    `gorm:"column:transaction_momentum"`
	TargetLayoutSupply  int       `gorm:"column:target_layout_supply"`
	CalculatedAt        time.Time `gorm:"column:calculated_at;autoCreateTime"`
}

func (NeighborhoodMetricModel) TableName() string {
	return "neighborhood_metrics"
}
