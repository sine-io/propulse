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
	ID        string    `gorm:"column:id;type:uuid;primaryKey"`
	Name      string    `gorm:"column:name"`
	City      *string   `gorm:"column:city"`
	Area      string    `gorm:"column:area"`
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime"`
}

type NeighborhoodLayoutModel struct {
	NeighborhoodID string    `gorm:"column:neighborhood_id;type:uuid;primaryKey"`
	Layout         string    `gorm:"column:layout;primaryKey"`
	CreatedAt      time.Time `gorm:"column:created_at;autoCreateTime"`
}

func (NeighborhoodLayoutModel) TableName() string {
	return "neighborhood_layouts"
}

func (NeighborhoodModel) TableName() string {
	return "neighborhoods"
}

type WatchlistItemModel struct {
	ID             string    `gorm:"column:id;type:uuid;primaryKey"`
	NeighborhoodID string    `gorm:"column:neighborhood_id;type:uuid"`
	UserID         string    `gorm:"column:user_id"`
	TargetLayout   string    `gorm:"column:target_layout"`
	CreatedAt      time.Time `gorm:"column:created_at;autoCreateTime"`
}

func (WatchlistItemModel) TableName() string {
	return "watchlist_items"
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

type ReviewNoteModel struct {
	ID             string     `gorm:"column:id;type:uuid;primaryKey"`
	UserID         string     `gorm:"column:user_id"`
	NeighborhoodID *string    `gorm:"column:neighborhood_id;type:uuid"`
	Kind           string     `gorm:"column:kind"`
	WeekStartDate  *time.Time `gorm:"column:week_start_date;type:date"`
	Content        string     `gorm:"column:content"`
	CreatedAt      time.Time  `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt      time.Time  `gorm:"column:updated_at;autoUpdateTime"`
}

func (ReviewNoteModel) TableName() string {
	return "review_notes"
}
