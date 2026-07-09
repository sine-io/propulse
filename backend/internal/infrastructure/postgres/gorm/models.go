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

type RawCollectionRecordModel struct {
	ID          string          `gorm:"column:id;type:uuid;primaryKey"`
	SourceType  string          `gorm:"column:source_type"`
	SourceRef   string          `gorm:"column:source_ref"`
	Payload     json.RawMessage `gorm:"column:payload;type:jsonb;not null"`
	CollectedAt time.Time       `gorm:"column:collected_at;autoCreateTime"`
}

func (RawCollectionRecordModel) TableName() string {
	return "raw_collection_records"
}

type ListingSnapshotModel struct {
	ID               string    `gorm:"column:id;type:uuid;primaryKey"`
	NeighborhoodID   string    `gorm:"column:neighborhood_id;type:uuid"`
	ListingPrice     float64   `gorm:"column:listing_price"`
	TransactionPrice float64   `gorm:"column:transaction_price"`
	PriceCut         bool      `gorm:"column:price_cut"`
	DaysOnMarket     int       `gorm:"column:days_on_market"`
	Layout           string    `gorm:"column:layout"`
	CapturedAt       time.Time `gorm:"column:captured_at;autoCreateTime"`
}

func (ListingSnapshotModel) TableName() string {
	return "listing_snapshots"
}
