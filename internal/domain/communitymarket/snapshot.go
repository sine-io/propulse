package communitymarket

import (
	"encoding/json"
	"math"
	"strings"
	"time"
	"unicode/utf8"
)

type SnapshotData struct {
	SourceCommunityID                 string          `json:"sourceCommunityId"`
	CommunityName                     string          `json:"communityName"`
	FormerName                        string          `json:"formerName,omitempty"`
	ProvinceCode                      string          `json:"provinceCode,omitempty"`
	ProvinceName                      string          `json:"provinceName,omitempty"`
	CityCode                          string          `json:"cityCode"`
	CityName                          string          `json:"cityName"`
	DistrictCode                      string          `json:"districtCode"`
	DistrictName                      string          `json:"districtName"`
	BlockCode                         string          `json:"blockCode"`
	BlockName                         string          `json:"blockName"`
	PropertyType                      string          `json:"propertyType,omitempty"`
	PropertyTags                      []string        `json:"propertyTags,omitempty"`
	BuildingCount                     *int            `json:"buildingCount,omitempty"`
	BuildingType                      string          `json:"buildingType,omitempty"`
	BuildingYear                      *int            `json:"buildingYear,omitempty"`
	Developer                         string          `json:"developer,omitempty"`
	HouseholdCount                    *int            `json:"householdCount,omitempty"`
	ClosedManagement                  string          `json:"closedManagement,omitempty"`
	PlotRatio                         *float64        `json:"plotRatio,omitempty"`
	GreenAreaSQM                      *float64        `json:"greenAreaSqm,omitempty"`
	GreeningRatePercent               *float64        `json:"greeningRatePercent,omitempty"`
	PropertyManagementCompany         string          `json:"propertyManagementCompany,omitempty"`
	PropertyFee                       string          `json:"propertyFee,omitempty"`
	FixedParkingSpaces                *int            `json:"fixedParkingSpaces,omitempty"`
	ParkingRatio                      string          `json:"parkingRatio,omitempty"`
	ParkingFee                        string          `json:"parkingFee,omitempty"`
	HeatingType                       string          `json:"heatingType,omitempty"`
	WaterType                         string          `json:"waterType,omitempty"`
	ElectricityType                   string          `json:"electricityType,omitempty"`
	GasCost                           string          `json:"gasCost,omitempty"`
	ManCarSeparation                  string          `json:"manCarSeparation,omitempty"`
	Latitude                          float64         `json:"latitude"`
	Longitude                         float64         `json:"longitude"`
	LatestListingDate                 *time.Time      `json:"latestListingDate,omitempty"`
	ListingAvgUnitPrice               *float64        `json:"listingAvgUnitPrice,omitempty"`
	ListingCount                      *int            `json:"listingCount,omitempty"`
	ListingAreaSQM                    *float64        `json:"listingAreaSqm,omitempty"`
	ListingAvgTotalPriceWan           *float64        `json:"listingAvgTotalPriceWan,omitempty"`
	ListingAvgUnitPrice6Months        *float64        `json:"listingAvgUnitPrice6Months,omitempty"`
	NewListingCount3Months            *int            `json:"newListingCount3Months,omitempty"`
	NewListingAvgTotalPrice3MonthsWan *float64        `json:"newListingAvgTotalPrice3MonthsWan,omitempty"`
	NewListingUnitPrice3Months        *float64        `json:"newListingUnitPrice3Months,omitempty"`
	LatestTradeDate                   *time.Time      `json:"latestTradeDate,omitempty"`
	LatestTradeAvgUnitPrice           *float64        `json:"latestTradeAvgUnitPrice,omitempty"`
	TradeCount3Months                 *int            `json:"tradeCount3Months,omitempty"`
	TradeArea3MonthsSQM               *float64        `json:"tradeArea3MonthsSqm,omitempty"`
	TradeAvgTotalPrice3MonthsWan      *float64        `json:"tradeAvgTotalPrice3MonthsWan,omitempty"`
	TradeUnitPrice3Months             *float64        `json:"tradeUnitPrice3Months,omitempty"`
	TradeAvgUnitPrice6Months          *float64        `json:"tradeAvgUnitPrice6Months,omitempty"`
	TradeCountPerMonth6Months         *float64        `json:"tradeCountPerMonth6Months,omitempty"`
	TakeLookCount                     *int            `json:"takeLookCount,omitempty"`
	TakeLookConversionRate            *float64        `json:"takeLookConversionRatePercent,omitempty"`
	OnSaleAreaRange                   string          `json:"onSaleAreaRangeSqm,omitempty"`
	OnSalePriceRange                  string          `json:"onSalePriceRangeWan,omitempty"`
	OnSaleRoomTypes                   []string        `json:"onSaleRoomTypes,omitempty"`
	Analysis                          json.RawMessage `json:"analysis"`
	Surroundings                      json.RawMessage `json:"surroundings"`
	CityContext                       json.RawMessage `json:"cityContext"`
}

type Violation struct {
	Field   string
	Code    string
	Message string
}

func (data SnapshotData) Normalize() SnapshotData {
	data.SourceCommunityID = strings.TrimSpace(data.SourceCommunityID)
	data.CommunityName = strings.TrimSpace(data.CommunityName)
	data.FormerName = strings.TrimSpace(data.FormerName)
	data.ProvinceCode = strings.TrimSpace(data.ProvinceCode)
	data.ProvinceName = strings.TrimSpace(data.ProvinceName)
	data.CityCode = strings.TrimSpace(data.CityCode)
	data.CityName = strings.TrimSpace(data.CityName)
	data.DistrictCode = strings.TrimSpace(data.DistrictCode)
	data.DistrictName = strings.TrimSpace(data.DistrictName)
	data.BlockCode = strings.TrimSpace(data.BlockCode)
	data.BlockName = strings.TrimSpace(data.BlockName)
	data.PropertyType = strings.TrimSpace(data.PropertyType)
	data.BuildingType = strings.TrimSpace(data.BuildingType)
	data.Developer = strings.TrimSpace(data.Developer)
	data.ClosedManagement = strings.TrimSpace(data.ClosedManagement)
	data.PropertyManagementCompany = strings.TrimSpace(data.PropertyManagementCompany)
	data.PropertyFee = strings.TrimSpace(data.PropertyFee)
	data.ParkingRatio = strings.TrimSpace(data.ParkingRatio)
	data.ParkingFee = strings.TrimSpace(data.ParkingFee)
	data.HeatingType = strings.TrimSpace(data.HeatingType)
	data.WaterType = strings.TrimSpace(data.WaterType)
	data.ElectricityType = strings.TrimSpace(data.ElectricityType)
	data.GasCost = strings.TrimSpace(data.GasCost)
	data.ManCarSeparation = strings.TrimSpace(data.ManCarSeparation)
	data.OnSaleAreaRange = strings.TrimSpace(data.OnSaleAreaRange)
	data.OnSalePriceRange = strings.TrimSpace(data.OnSalePriceRange)

	data.PropertyTags = normalizeDistinctStrings(data.PropertyTags)
	if len(data.PropertyTags) == 0 {
		data.PropertyTags = nil
	}
	data.OnSaleRoomTypes = normalizeDistinctStrings(data.OnSaleRoomTypes)
	data.Analysis = normalizeJSONObject(data.Analysis)
	data.Surroundings = normalizeJSONObject(data.Surroundings)
	data.CityContext = normalizeJSONObject(data.CityContext)
	return data
}

func (data SnapshotData) Validate(collectedAt time.Time) []Violation {
	violations := make([]Violation, 0)
	for _, field := range []struct {
		name  string
		value string
		max   int
	}{
		{name: "sourceCommunityId", value: data.SourceCommunityID, max: 128},
		{name: "communityName", value: data.CommunityName, max: 256},
		{name: "cityCode", value: data.CityCode, max: 32},
		{name: "cityName", value: data.CityName, max: 128},
		{name: "districtCode", value: data.DistrictCode, max: 32},
		{name: "districtName", value: data.DistrictName, max: 128},
		{name: "blockCode", value: data.BlockCode, max: 64},
		{name: "blockName", value: data.BlockName, max: 128},
	} {
		length := utf8.RuneCountInString(field.value)
		if length == 0 {
			violations = append(violations, Violation{Field: field.name, Code: "required", Message: field.name + " is required"})
		} else if length > field.max {
			violations = append(violations, Violation{Field: field.name, Code: "too_long", Message: field.name + " is too long"})
		}
	}
	for _, field := range []struct {
		name  string
		value string
		max   int
	}{
		{name: "formerName", value: data.FormerName, max: 256},
		{name: "provinceCode", value: data.ProvinceCode, max: 32},
		{name: "provinceName", value: data.ProvinceName, max: 128},
		{name: "propertyType", value: data.PropertyType, max: 128},
		{name: "buildingType", value: data.BuildingType, max: 128},
		{name: "developer", value: data.Developer, max: 256},
		{name: "closedManagement", value: data.ClosedManagement, max: 16},
		{name: "propertyManagementCompany", value: data.PropertyManagementCompany, max: 256},
		{name: "propertyFee", value: data.PropertyFee, max: 128},
		{name: "parkingRatio", value: data.ParkingRatio, max: 64},
		{name: "parkingFee", value: data.ParkingFee, max: 128},
		{name: "heatingType", value: data.HeatingType, max: 128},
		{name: "waterType", value: data.WaterType, max: 128},
		{name: "electricityType", value: data.ElectricityType, max: 128},
		{name: "gasCost", value: data.GasCost, max: 128},
		{name: "manCarSeparation", value: data.ManCarSeparation, max: 16},
		{name: "onSaleAreaSegSqm", value: data.OnSaleAreaRange, max: 128},
		{name: "onSalePriceSegWan", value: data.OnSalePriceRange, max: 128},
	} {
		if utf8.RuneCountInString(field.value) > field.max {
			violations = append(violations, Violation{Field: field.name, Code: "too_long", Message: field.name + " is too long"})
		}
	}
	if !finiteInRange(data.Latitude, -90, 90) {
		violations = append(violations, Violation{Field: "latitude", Code: "out_of_range", Message: "latitude must be between -90 and 90"})
	}
	if !finiteInRange(data.Longitude, -180, 180) {
		violations = append(violations, Violation{Field: "longitude", Code: "out_of_range", Message: "longitude must be between -180 and 180"})
	}
	for _, field := range []struct {
		name  string
		value *int
	}{
		{name: "buildingCount", value: data.BuildingCount},
		{name: "householdCount", value: data.HouseholdCount},
		{name: "fixedParkingSpaces", value: data.FixedParkingSpaces},
		{name: "listingNum", value: data.ListingCount},
		{name: "newListingNum3month", value: data.NewListingCount3Months},
		{name: "tradeNum3month", value: data.TradeCount3Months},
		{name: "takeLook", value: data.TakeLookCount},
	} {
		if field.value != nil && (*field.value < 0 || int64(*field.value) > math.MaxInt32) {
			violations = append(violations, Violation{Field: field.name, Code: "out_of_range", Message: field.name + " must be a non-negative 32-bit integer"})
		}
	}
	if data.BuildingYear != nil && (*data.BuildingYear < 1800 || (!collectedAt.IsZero() && *data.BuildingYear > collectedAt.UTC().Year())) {
		violations = append(violations, Violation{Field: "buildingYear", Code: "out_of_range", Message: "buildingYear must be between 1800 and the collection year"})
	}
	for _, field := range []struct {
		name  string
		value *float64
	}{
		{name: "latestListingAvgPriceYuanPerSqm", value: data.ListingAvgUnitPrice},
		{name: "listingAreaSqm", value: data.ListingAreaSQM},
		{name: "listingAvgTotalPriceWan", value: data.ListingAvgTotalPriceWan},
		{name: "listingAvgPrice6monthYuanPerSqm", value: data.ListingAvgUnitPrice6Months},
		{name: "newListingAvgTotalPrice3monthWan", value: data.NewListingAvgTotalPrice3MonthsWan},
		{name: "newListingUnitPrice3monthYuanPerSqm", value: data.NewListingUnitPrice3Months},
		{name: "latestTradeAvgPriceYuanPerSqm", value: data.LatestTradeAvgUnitPrice},
		{name: "tradeArea3monthSqm", value: data.TradeArea3MonthsSQM},
		{name: "tradeAvgTotalPrice3monthWan", value: data.TradeAvgTotalPrice3MonthsWan},
		{name: "tradeUnitPrice3monthYuanPerSqm", value: data.TradeUnitPrice3Months},
		{name: "tradeAvgPrice6monthYuanPerSqm", value: data.TradeAvgUnitPrice6Months},
		{name: "tradeNumPerMonth6month", value: data.TradeCountPerMonth6Months},
	} {
		if field.value != nil && (!isFinite(*field.value) || *field.value < 0) {
			violations = append(violations, Violation{Field: field.name, Code: "out_of_range", Message: field.name + " must be a non-negative finite number"})
		}
	}
	if data.TakeLookConversionRate != nil && (!finiteInRange(*data.TakeLookConversionRate, 0, 100)) {
		violations = append(violations, Violation{Field: "takeLookTransRatePercent", Code: "out_of_range", Message: "takeLookTransRatePercent must be between 0 and 100"})
	}
	for _, field := range []struct {
		name  string
		value *float64
		max   float64
	}{
		{name: "plotRatio", value: data.PlotRatio, max: 100},
		{name: "greenAreaSqm", value: data.GreenAreaSQM, max: 999999999999},
		{name: "greeningRatePercent", value: data.GreeningRatePercent, max: 100},
	} {
		if field.value != nil && !finiteInRange(*field.value, 0, field.max) {
			violations = append(violations, Violation{Field: field.name, Code: "out_of_range", Message: field.name + " is outside its supported range"})
		}
	}
	for _, field := range []struct {
		name  string
		value string
	}{
		{name: "closedManagement", value: data.ClosedManagement},
		{name: "manCarSeparation", value: data.ManCarSeparation},
	} {
		if field.value != "" && field.value != "是" && field.value != "否" {
			violations = append(violations, Violation{Field: field.name, Code: "invalid_enum", Message: field.name + " must be 是 or 否"})
		}
	}
	for _, field := range []struct {
		name  string
		value *time.Time
	}{
		{name: "latestListingDate", value: data.LatestListingDate},
		{name: "latestTradeDate", value: data.LatestTradeDate},
	} {
		if field.value != nil && field.value.After(endOfUTCDate(collectedAt)) {
			violations = append(violations, Violation{Field: field.name, Code: "future", Message: field.name + " must not be after the collection date"})
		}
	}
	if len(data.OnSaleRoomTypes) > 20 {
		violations = append(violations, Violation{Field: "onSaleRoomType", Code: "too_many", Message: "onSaleRoomType must contain at most 20 values"})
	}
	for _, roomType := range data.OnSaleRoomTypes {
		if utf8.RuneCountInString(roomType) > 64 {
			violations = append(violations, Violation{Field: "onSaleRoomType", Code: "too_long", Message: "each onSaleRoomType value must be at most 64 characters"})
		}
	}
	if len(data.PropertyTags) > 20 {
		violations = append(violations, Violation{Field: "propertyTags", Code: "too_many", Message: "propertyTags must contain at most 20 values"})
	}
	for _, tag := range data.PropertyTags {
		if utf8.RuneCountInString(tag) > 64 {
			violations = append(violations, Violation{Field: "propertyTags", Code: "too_long", Message: "each propertyTags value must be at most 64 characters"})
		}
	}
	return violations
}

func normalizeDistinctStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		normalized = append(normalized, value)
	}
	return normalized
}

func normalizeJSONObject(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage(`{}`)
	}
	return append(json.RawMessage(nil), raw...)
}

func finiteInRange(value, min, max float64) bool {
	return isFinite(value) && value >= min && value <= max
}

func isFinite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

func endOfUTCDate(value time.Time) time.Time {
	utc := value.UTC()
	return time.Date(utc.Year(), utc.Month(), utc.Day(), 23, 59, 59, int(time.Second-time.Nanosecond), time.UTC)
}
