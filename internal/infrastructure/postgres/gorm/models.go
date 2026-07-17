package gormrepo

import (
	"time"

	"encoding/json"
)

type CapacityCalculationModel struct {
	ID               string          `gorm:"column:id;type:uuid;primaryKey"`
	UserID           string          `gorm:"column:user_id"`
	Input            json.RawMessage `gorm:"column:input;type:jsonb;not null"`
	Result           json.RawMessage `gorm:"column:result;type:jsonb;not null"`
	SelectionContext json.RawMessage `gorm:"column:selection_context;type:jsonb"`
	CreatedAt        time.Time       `gorm:"column:created_at;autoCreateTime"`
}

func (CapacityCalculationModel) TableName() string {
	return "capacity_calculations"
}

type UserPropertyAssetModel struct {
	ID                       string          `gorm:"column:id;type:uuid;primaryKey"`
	UserID                   string          `gorm:"column:user_id"`
	Name                     string          `gorm:"column:name"`
	NeighborhoodID           string          `gorm:"column:neighborhood_id;type:uuid"`
	NeighborhoodName         string          `gorm:"column:neighborhood_name"`
	City                     string          `gorm:"column:city"`
	District                 string          `gorm:"column:district"`
	Layout                   string          `gorm:"column:layout"`
	AreaSQM                  float64         `gorm:"column:area_sqm"`
	FloorBand                string          `gorm:"column:floor_band"`
	FloorDescription         string          `gorm:"column:floor_description"`
	Orientation              string          `gorm:"column:orientation"`
	CurrentListingPriceWan   *float64        `gorm:"column:current_listing_price_wan"`
	OriginalPurchasePriceWan float64         `gorm:"column:original_purchase_price_wan"`
	PurchasedOn              time.Time       `gorm:"column:purchased_on;type:date"`
	CurrentLoanBalanceWan    float64         `gorm:"column:current_loan_balance_wan"`
	SourceKind               string          `gorm:"column:source_kind"`
	SourceSnapshot           json.RawMessage `gorm:"column:source_snapshot;type:jsonb;not null"`
	CreatedAt                time.Time       `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt                time.Time       `gorm:"column:updated_at;autoUpdateTime"`
	DeletedAt                *time.Time      `gorm:"column:deleted_at"`
}

func (UserPropertyAssetModel) TableName() string {
	return "user_property_assets"
}

type CapacityPolicyVersionModel struct {
	ID            string          `gorm:"column:id;type:uuid;primaryKey"`
	City          string          `gorm:"column:city"`
	Version       string          `gorm:"column:version"`
	Name          string          `gorm:"column:name"`
	EffectiveFrom time.Time       `gorm:"column:effective_from;type:date"`
	EffectiveTo   *time.Time      `gorm:"column:effective_to;type:date"`
	Enabled       bool            `gorm:"column:enabled"`
	Rules         json.RawMessage `gorm:"column:rules;type:jsonb;not null"`
	Sources       json.RawMessage `gorm:"column:sources;type:jsonb;not null"`
	CreatedAt     time.Time       `gorm:"column:created_at;autoCreateTime"`
}

func (CapacityPolicyVersionModel) TableName() string {
	return "capacity_policy_versions"
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
	ID                 string          `gorm:"column:id;type:uuid;primaryKey"`
	CollectionRunID    string          `gorm:"column:collection_run_id;type:uuid"`
	NeighborhoodID     string          `gorm:"column:neighborhood_id;type:uuid"`
	SourceRecordID     string          `gorm:"column:source_record_id"`
	SourceRow          int             `gorm:"column:source_row"`
	Layout             string          `gorm:"column:layout"`
	AreaSQM            float64         `gorm:"column:area_sqm"`
	TransactionPrice   float64         `gorm:"column:transaction_price"`
	TransactionDate    time.Time       `gorm:"column:transaction_date"`
	OriginalListingRef *string         `gorm:"column:original_listing_ref"`
	CapturedAt         time.Time       `gorm:"column:captured_at"`
	Attributes         json.RawMessage `gorm:"column:attributes;type:jsonb"`
}

func (TransactionObservationModel) TableName() string {
	return "transaction_observations"
}

type CommunityMarketSnapshotModel struct {
	ID                                string          `gorm:"column:id;type:uuid;primaryKey"`
	DataSourceID                      string          `gorm:"column:data_source_id;type:uuid"`
	NeighborhoodID                    string          `gorm:"column:neighborhood_id;type:uuid"`
	SourceRef                         string          `gorm:"column:source_ref"`
	CollectedAt                       time.Time       `gorm:"column:collected_at"`
	ContentChecksum                   string          `gorm:"column:content_checksum"`
	RawPayload                        []byte          `gorm:"column:raw_payload;type:bytea"`
	RawContentType                    string          `gorm:"column:raw_content_type"`
	CollectionRunID                   *string         `gorm:"column:collection_run_id;type:uuid"`
	SourceCommunityID                 string          `gorm:"column:source_community_id"`
	CommunityName                     string          `gorm:"column:community_name"`
	FormerName                        string          `gorm:"column:former_name"`
	ProvinceCode                      *string         `gorm:"column:province_code"`
	ProvinceName                      *string         `gorm:"column:province_name"`
	CityCode                          string          `gorm:"column:city_code"`
	CityName                          string          `gorm:"column:city_name"`
	DistrictCode                      string          `gorm:"column:district_code"`
	DistrictName                      string          `gorm:"column:district_name"`
	BlockCode                         string          `gorm:"column:block_code"`
	BlockName                         string          `gorm:"column:block_name"`
	PropertyType                      *string         `gorm:"column:property_type"`
	PropertyTags                      json.RawMessage `gorm:"column:property_tags;type:jsonb"`
	BuildingCount                     *int            `gorm:"column:building_count"`
	BuildingType                      *string         `gorm:"column:building_type"`
	BuildingYear                      *int            `gorm:"column:building_year"`
	Developer                         *string         `gorm:"column:developer"`
	HouseholdCount                    *int            `gorm:"column:household_count"`
	ClosedManagement                  *string         `gorm:"column:closed_management"`
	PlotRatio                         *float64        `gorm:"column:plot_ratio"`
	GreenAreaSQM                      *float64        `gorm:"column:green_area_sqm"`
	GreeningRatePercent               *float64        `gorm:"column:greening_rate_percent"`
	PropertyManagementCompany         *string         `gorm:"column:property_management_company"`
	PropertyFee                       *string         `gorm:"column:property_fee"`
	FixedParkingSpaces                *int            `gorm:"column:fixed_parking_spaces"`
	ParkingRatio                      *string         `gorm:"column:parking_ratio"`
	ParkingFee                        *string         `gorm:"column:parking_fee"`
	HeatingType                       *string         `gorm:"column:heating_type"`
	WaterType                         *string         `gorm:"column:water_type"`
	ElectricityType                   *string         `gorm:"column:electricity_type"`
	GasCost                           *string         `gorm:"column:gas_cost"`
	ManCarSeparation                  *string         `gorm:"column:man_car_separation"`
	Latitude                          float64         `gorm:"column:latitude"`
	Longitude                         float64         `gorm:"column:longitude"`
	LatestListingDate                 *time.Time      `gorm:"column:latest_listing_date;type:date"`
	ListingAvgUnitPrice               *float64        `gorm:"column:listing_avg_unit_price"`
	ListingCount                      *int            `gorm:"column:listing_count"`
	ListingAreaSQM                    *float64        `gorm:"column:listing_area_sqm"`
	ListingAvgTotalPriceWan           *float64        `gorm:"column:listing_avg_total_price_wan"`
	ListingAvgUnitPrice6Months        *float64        `gorm:"column:listing_avg_unit_price_6m"`
	NewListingCount3Months            *int            `gorm:"column:new_listing_count_3m"`
	NewListingAvgTotalPrice3MonthsWan *float64        `gorm:"column:new_listing_avg_total_price_3m_wan"`
	NewListingUnitPrice3Months        *float64        `gorm:"column:new_listing_unit_price_3m"`
	LatestTradeDate                   *time.Time      `gorm:"column:latest_trade_date;type:date"`
	LatestTradeAvgUnitPrice           *float64        `gorm:"column:latest_trade_avg_unit_price"`
	TradeCount3Months                 *int            `gorm:"column:trade_count_3m"`
	TradeArea3MonthsSQM               *float64        `gorm:"column:trade_area_3m_sqm"`
	TradeAvgTotalPrice3MonthsWan      *float64        `gorm:"column:trade_avg_total_price_3m_wan"`
	TradeUnitPrice3Months             *float64        `gorm:"column:trade_unit_price_3m"`
	TradeAvgUnitPrice6Months          *float64        `gorm:"column:trade_avg_unit_price_6m"`
	TradeCountPerMonth6Months         *float64        `gorm:"column:trade_count_per_month_6m"`
	TakeLookCount                     *int            `gorm:"column:take_look_count"`
	TakeLookConversionRate            *float64        `gorm:"column:take_look_conversion_rate"`
	OnSaleAreaRange                   string          `gorm:"column:on_sale_area_range"`
	OnSalePriceRange                  string          `gorm:"column:on_sale_price_range"`
	OnSaleRoomTypes                   json.RawMessage `gorm:"column:on_sale_room_types;type:jsonb"`
	Analysis                          json.RawMessage `gorm:"column:analysis;type:jsonb"`
	Surroundings                      json.RawMessage `gorm:"column:surroundings;type:jsonb"`
	CityContext                       json.RawMessage `gorm:"column:city_context;type:jsonb"`
	QualityStatus                     string          `gorm:"column:quality_status"`
	CreatedAt                         time.Time       `gorm:"column:created_at;autoCreateTime"`
}

func (CommunityMarketSnapshotModel) TableName() string {
	return "community_market_snapshots"
}

type ListingAdjustmentModel struct {
	ID              string    `gorm:"column:id;type:uuid;primaryKey"`
	CollectionRunID string    `gorm:"column:collection_run_id;type:uuid"`
	NeighborhoodID  string    `gorm:"column:neighborhood_id;type:uuid"`
	RoomID          string    `gorm:"column:room_id"`
	AdjustedAt      time.Time `gorm:"column:adjusted_at;type:date"`
	PriceBeforeWan  float64   `gorm:"column:price_before_wan"`
	PriceAfterWan   float64   `gorm:"column:price_after_wan"`
	AmountWan       float64   `gorm:"column:amount_wan"`
	CreatedAt       time.Time `gorm:"column:created_at;autoCreateTime"`
}

func (ListingAdjustmentModel) TableName() string {
	return "listing_adjustments"
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
