package gormrepo

import (
	"time"

	"encoding/json"
)

type CapacityCalculationModel struct {
	ID        string          `gorm:"column:id;type:uuid;primaryKey"`
	UserID    string          `gorm:"column:user_id"`
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
	CollectionRunID  string    `gorm:"column:collection_run_id;type:uuid"`
	NeighborhoodID   string    `gorm:"column:neighborhood_id;type:uuid"`
	ListingPrice     float64   `gorm:"column:listing_price"`
	TransactionPrice *float64  `gorm:"column:transaction_price"`
	PriceCut         bool      `gorm:"column:price_cut"`
	DaysOnMarket     int       `gorm:"column:days_on_market"`
	Layout           string    `gorm:"column:layout"`
	CapturedAt       time.Time `gorm:"column:captured_at;autoCreateTime"`
}

func (ListingSnapshotModel) TableName() string {
	return "listing_snapshots"
}

type DataSourceModel struct {
	ID         string    `gorm:"column:id;type:uuid;primaryKey"`
	Name       string    `gorm:"column:name"`
	SourceType string    `gorm:"column:source_type"`
	City       string    `gorm:"column:city"`
	Notes      string    `gorm:"column:notes"`
	CreatedAt  time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt  time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (DataSourceModel) TableName() string {
	return "data_sources"
}

type CollectionRunModel struct {
	ID                string          `gorm:"column:id;type:uuid;primaryKey"`
	DataSourceID      string          `gorm:"column:data_source_id;type:uuid"`
	NeighborhoodID    string          `gorm:"column:neighborhood_id;type:uuid"`
	SourceRef         string          `gorm:"column:source_ref"`
	CollectedAt       time.Time       `gorm:"column:collected_at"`
	Coverage          string          `gorm:"column:coverage"`
	ImportFormat      string          `gorm:"column:import_format"`
	ContentChecksum   string          `gorm:"column:content_checksum"`
	RawPayload        []byte          `gorm:"column:raw_payload;type:bytea"`
	RawContentType    string          `gorm:"column:raw_content_type"`
	ValidationSummary json.RawMessage `gorm:"column:validation_summary;type:jsonb"`
	Status            string          `gorm:"column:status"`
	MetricStatus      string          `gorm:"column:metric_status"`
	CreatedAt         time.Time       `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt         time.Time       `gorm:"column:updated_at;autoUpdateTime"`
}

func (CollectionRunModel) TableName() string {
	return "collection_runs"
}

type ListingObservationModel struct {
	ID              string          `gorm:"column:id;type:uuid;primaryKey"`
	CollectionRunID string          `gorm:"column:collection_run_id;type:uuid"`
	NeighborhoodID  string          `gorm:"column:neighborhood_id;type:uuid"`
	SourceListingID string          `gorm:"column:source_listing_id"`
	SourceRow       int             `gorm:"column:source_row"`
	Layout          string          `gorm:"column:layout"`
	AreaSQM         float64         `gorm:"column:area_sqm"`
	ListingPrice    float64         `gorm:"column:listing_price"`
	DaysOnMarket    int             `gorm:"column:days_on_market"`
	Status          string          `gorm:"column:status"`
	CapturedAt      time.Time       `gorm:"column:captured_at"`
	Attributes      json.RawMessage `gorm:"column:attributes;type:jsonb"`
}

func (ListingObservationModel) TableName() string {
	return "listing_observations"
}

type TransactionObservationModel struct {
	ID                 string    `gorm:"column:id;type:uuid;primaryKey"`
	CollectionRunID    string    `gorm:"column:collection_run_id;type:uuid"`
	NeighborhoodID     string    `gorm:"column:neighborhood_id;type:uuid"`
	SourceRecordID     string    `gorm:"column:source_record_id"`
	SourceRow          int       `gorm:"column:source_row"`
	Layout             string    `gorm:"column:layout"`
	AreaSQM            float64   `gorm:"column:area_sqm"`
	TransactionPrice   float64   `gorm:"column:transaction_price"`
	TransactionDate    time.Time `gorm:"column:transaction_date"`
	OriginalListingRef *string   `gorm:"column:original_listing_ref"`
	CapturedAt         time.Time `gorm:"column:captured_at"`
}

func (TransactionObservationModel) TableName() string {
	return "transaction_observations"
}
