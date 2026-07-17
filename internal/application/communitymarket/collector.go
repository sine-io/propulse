package communitymarket

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	domaincommunitymarket "github.com/sine-io/propulse/internal/domain/communitymarket"
)

var (
	ErrFangjianResponse   = errors.New("fangjian_response_invalid")
	ErrFangjianIncomplete = errors.New("fangjian_collection_incomplete")
)

type FangjianCommunityConfig struct {
	Slug          string
	CommunityID   string
	CommunityName string
	CityCode      string
	DistrictCode  string
	BlockCode     string
	Longitude     string
	Latitude      string
}

var DefaultFangjianCommunities = []FangjianCommunityConfig{
	{
		Slug: "mingquan", CommunityID: "a2d56505411446cfe70fd3960beb19c7",
		CommunityName: "富力津门湖鸣泉花园", CityCode: "120100", DistrictCode: "120111",
		BlockCode: "BK2022112435579", Longitude: "117.203624", Latitude: "39.057089",
	},
	{
		Slug: "qinhe", CommunityID: "0a5b87b0d81dadbb50fb85df01489a13",
		CommunityName: "亲和美园", CityCode: "120100", DistrictCode: "120111",
		BlockCode: "BK2022112435657", Longitude: "117.265793001744", Latitude: "39.00686699034",
	},
}

type Collector struct {
	client  FangjianClient
	archive FangjianArchive
	now     func() time.Time
}

func NewCollector(client FangjianClient, archive FangjianArchive, now func() time.Time) *Collector {
	if now == nil {
		now = time.Now
	}
	return &Collector{client: client, archive: archive, now: now}
}

func (c *Collector) Collect(ctx context.Context, config FangjianCommunityConfig) (string, FangjianBundle, error) {
	collectedAt := c.now().UTC().Truncate(time.Second)
	raw := make(map[string]json.RawMessage)
	endpoints := make([]string, 0, 24)
	requestGet := func(key, path string) (json.RawMessage, error) {
		body, err := c.client.Get(ctx, path)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		data, err := fangjianData(body)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		raw[key] = append(json.RawMessage(nil), body...)
		endpoints = append(endpoints, path)
		return data, nil
	}
	requestPost := func(key, path string, input any) (json.RawMessage, error) {
		body, err := c.client.Post(ctx, path, input)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		data, err := fangjianData(body)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		raw[key] = append(json.RawMessage(nil), body...)
		endpoints = append(endpoints, path)
		return data, nil
	}

	basicPath := "/esf/ex/basicInfo/" + config.CommunityID
	basicData, err := requestGet("basic-info", basicPath)
	if err != nil {
		return "", FangjianBundle{}, err
	}
	var basicEnvelope map[string]json.RawMessage
	if err := json.Unmarshal(basicData, &basicEnvelope); err != nil {
		return "", FangjianBundle{}, fmt.Errorf("basic info: %w", ErrFangjianResponse)
	}
	basicRaw := basicEnvelope["basicInfo"]
	if len(basicRaw) == 0 || string(basicRaw) == "null" {
		return "", FangjianBundle{}, fmt.Errorf("basic info missing: %w", ErrFangjianResponse)
	}

	analysisKinds := []string{
		"listingPrice", "tradeTrends", "priceDiff", "roomType", "tradeCycle",
		"supplyTrend", "tradeSummary", "zf", "adjustCondition", "adjustConditionSummary",
		"adjustDetailSummary", "hotIndex", "hotIndexCompare", "confidenceIndex",
	}
	analysis := make(map[string]json.RawMessage, len(analysisKinds))
	for _, kind := range analysisKinds {
		path := fmt.Sprintf("/esf/ex/%s/%s/%s/%s/%s", kind, config.CommunityID, config.CityCode, config.DistrictCode, config.BlockCode)
		data, err := requestGet("analysis-"+kind, path)
		if err != nil {
			return "", FangjianBundle{}, err
		}
		analysis[kind] = data
	}

	citySummary, err := requestGet("city-summary", "/home/summary?cityCode="+config.CityCode)
	if err != nil {
		return "", FangjianBundle{}, err
	}
	mapInput := fangjianMapInput(config)
	mapData, err := requestPost("map-community", "/assetMap/fixSearch", mapInput)
	if err != nil {
		return "", FangjianBundle{}, err
	}
	mapCommunity, err := findMapCommunity(mapData, config.CommunityID)
	if err != nil {
		return "", FangjianBundle{}, err
	}

	_, listingResult, err := c.collectRecordRows(ctx, raw, &endpoints, "listing", config, collectedAt)
	if err != nil {
		return "", FangjianBundle{}, err
	}
	listings, ok := listingResult.([]MarketListing)
	if !ok {
		return "", FangjianBundle{}, ErrFangjianResponse
	}
	_, transactionResult, err := c.collectRecordRows(ctx, raw, &endpoints, "transaction", config, collectedAt)
	if err != nil {
		return "", FangjianBundle{}, err
	}
	transactions, ok := transactionResult.([]MarketTransaction)
	if !ok {
		return "", FangjianBundle{}, ErrFangjianResponse
	}

	adjustmentRoomIDs := map[string]struct{}{}
	for _, item := range listings {
		if item.AdjustmentCount > 0 {
			adjustmentRoomIDs[item.RoomID] = struct{}{}
		}
	}
	for _, item := range transactions {
		if item.AdjustmentCount > 0 {
			adjustmentRoomIDs[item.RoomID] = struct{}{}
		}
	}
	roomIDs := make([]string, 0, len(adjustmentRoomIDs))
	for roomID := range adjustmentRoomIDs {
		roomIDs = append(roomIDs, roomID)
	}
	sort.Strings(roomIDs)
	adjustments := make([]ListingAdjustment, 0)
	for _, roomID := range roomIDs {
		data, err := requestPost("adjustment-"+roomID, "/esf/adjustRecord", map[string]any{
			"aurCommunityId": config.CommunityID, "roomId": roomID,
		})
		if err != nil {
			return "", FangjianBundle{}, err
		}
		rows, err := decodeObjectRows(data)
		if err != nil {
			return "", FangjianBundle{}, fmt.Errorf("adjustment %s: %w", roomID, err)
		}
		for _, row := range rows {
			adjustedAt, err := sourceDate(row["adjustDate"])
			if err != nil {
				return "", FangjianBundle{}, err
			}
			adjustments = append(adjustments, ListingAdjustment{
				RoomID: roomID, AdjustedAt: adjustedAt,
				PriceBeforeWan: sourceFloat(row["listingPriceAdjustBefore"]),
				PriceAfterWan:  sourceFloat(row["listingPriceAdjustAfter"]),
				AmountWan:      sourceFloat(row["adjustAmount"]),
			})
		}
	}
	adjustments = deduplicateAdjustments(adjustments)

	surroundings := make(map[string]json.RawMessage, 3)
	surroundRequests := []struct {
		key  string
		path string
		body any
	}{
		{key: "competitiveSummary", path: "/assets/surround/queryCompetitiveSummary", body: competitiveInput(config)},
		{key: "competitiveProducts", path: "/assets/surround/competitiveProduct", body: competitiveInput(config)},
		{key: "poi", path: "/assets/surround/pagePoi", body: poiInput(config)},
	}
	for _, request := range surroundRequests {
		data, err := requestPost("surround-"+request.key, request.path, request.body)
		if err != nil {
			return "", FangjianBundle{}, err
		}
		surroundings[request.key] = data
	}

	analysisJSON, _ := json.Marshal(analysis)
	surroundingsJSON, _ := json.Marshal(surroundings)
	cityContextJSON, _ := json.Marshal(map[string]json.RawMessage{"summary": citySummary, "map": mapData})
	community, err := normalizeCommunitySnapshot(basicRaw, mapCommunity, analysisJSON, surroundingsJSON, cityContextJSON)
	if err != nil {
		return "", FangjianBundle{}, err
	}
	listingCount := len(listings)
	community.ListingCount = &listingCount
	bundle := FangjianBundle{
		SchemaVersion: FangjianBundleSchemaVersion, CollectedAt: collectedAt, Community: community,
		Listings: listings, Transactions: transactions, Adjustments: adjustments,
		Quality: BundleQuality{Status: "complete", Warnings: []string{}},
	}
	path, err := c.archive.Write(ctx, CollectedCommunity{
		Slug: config.Slug, Bundle: bundle, Raw: raw, Endpoints: uniqueStrings(endpoints),
	})
	if err != nil {
		return "", FangjianBundle{}, err
	}
	return path, bundle, nil
}

func (c *Collector) collectRecordRows(
	ctx context.Context,
	raw map[string]json.RawMessage,
	endpoints *[]string,
	kind string,
	config FangjianCommunityConfig,
	collectedAt time.Time,
) (json.RawMessage, any, error) {
	path := "/esf/listingRecord"
	if kind == "transaction" {
		path = "/esf/tradeRecord"
	}
	request := func(key, roomType, floor string) (json.RawMessage, []map[string]any, error) {
		input := map[string]any{
			"aurCommunityId": config.CommunityID, "sorting": nil,
			"roomTypeFilter": nullableString(roomType), "currentFloor": nullableString(floor),
		}
		if kind == "listing" {
			input["latestListingDate"] = firstDayOfPreviousMonth(collectedAt).Format(time.DateOnly)
			input["listingPriceStart"] = nil
			input["listingPriceEnd"] = nil
		} else {
			input["tradePriceStart"] = nil
			input["tradePriceEnd"] = nil
		}
		body, err := c.client.Post(ctx, path, input)
		if err != nil {
			return nil, nil, fmt.Errorf("%s [%s]: %w", path, key, err)
		}
		data, err := fangjianData(body)
		if err != nil {
			return nil, nil, fmt.Errorf("%s [%s]: %w", path, key, err)
		}
		raw[key] = append(json.RawMessage(nil), body...)
		*endpoints = append(*endpoints, path)
		rows, err := decodeObjectRows(data)
		return data, rows, err
	}

	allRaw, allRows, err := request(kind+"-all", "", "")
	if err != nil {
		return nil, nil, err
	}
	merged := mergeRows(nil, allRows)
	if len(allRows) == 100 {
		roomTypes := sourceRoomTypes(allRows)
		for _, roomType := range roomTypes {
			_, rows, err := request(kind+"-"+safeArchiveKey(roomType), roomType, "")
			if err != nil {
				return nil, nil, err
			}
			if len(rows) < 100 {
				merged = mergeRows(merged, rows)
				continue
			}
			for _, floor := range []string{"高楼层", "中楼层", "低楼层"} {
				_, leafRows, err := request(kind+"-"+safeArchiveKey(roomType)+"-"+safeArchiveKey(floor), roomType, floor)
				if err != nil {
					return nil, nil, err
				}
				if len(leafRows) == 100 {
					return nil, nil, fmt.Errorf("%s %s %s still returned 100 rows: %w", kind, roomType, floor, ErrFangjianIncomplete)
				}
				merged = mergeRows(merged, leafRows)
			}
		}
	}
	if kind == "listing" {
		items, err := normalizeListingRows(merged, collectedAt)
		return allRaw, items, err
	}
	items, err := normalizeTransactionRows(merged)
	return allRaw, items, err
}

func firstDayOfPreviousMonth(value time.Time) time.Time {
	firstDay := time.Date(value.Year(), value.Month(), 1, 0, 0, 0, 0, value.Location())
	return firstDay.AddDate(0, -1, 0)
}

func fangjianData(body json.RawMessage) (json.RawMessage, error) {
	var response struct {
		Code        int             `json:"code"`
		Description string          `json:"description"`
		Data        json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("%w: invalid JSON", ErrFangjianResponse)
	}
	if response.Code != 200 {
		description := strings.Join(strings.Fields(response.Description), " ")
		if len(description) > 200 {
			description = description[:200]
		}
		if description != "" {
			return nil, fmt.Errorf("%w: business code %d: %s", ErrFangjianResponse, response.Code, description)
		}
		return nil, fmt.Errorf("%w: business code %d", ErrFangjianResponse, response.Code)
	}
	if len(response.Data) == 0 || string(response.Data) == "null" {
		return nil, fmt.Errorf("%w: missing data", ErrFangjianResponse)
	}
	return response.Data, nil
}

func normalizeCommunitySnapshot(basicRaw json.RawMessage, mapRow map[string]any, analysis, surroundings, cityContext json.RawMessage) (domaincommunitymarket.SnapshotData, error) {
	var basic map[string]any
	if err := json.Unmarshal(basicRaw, &basic); err != nil {
		return domaincommunitymarket.SnapshotData{}, err
	}
	data := domaincommunitymarket.SnapshotData{
		SourceCommunityID: sourceString(basic["aurCommunityId"]), CommunityName: sourceString(basic["communityName"]),
		FormerName: sourceString(basic["formerName"]), ProvinceCode: sourceString(mapRow["provinceCode"]), ProvinceName: sourceString(mapRow["provinceName"]),
		CityCode: firstString(basic["aurCityCode"], mapRow["cityCode"]), CityName: firstString(basic["aurCityName"], basic["cityName"], mapRow["cityName"]),
		DistrictCode: firstString(basic["aurDistrictCode"], mapRow["districtCode"]), DistrictName: firstString(basic["aurDistrictName"], basic["districtName"], mapRow["districtName"]),
		BlockCode: firstString(basic["aurBlockCode"], mapRow["blockCode"]), BlockName: firstString(basic["aurBlockName"], basic["blockName"], mapRow["blockName"]),
		PropertyType: sourceString(mapRow["aurPropertyType"]), PropertyTags: sourceStringSlice(mapRow["aurAttributeTag"]),
		BuildingCount: sourceIntPointer(basic["buildingNum"]), BuildingType: sourceString(basic["buildingType"]), BuildingYear: sourceIntPointer(basic["buildingYear2"]),
		Developer: sourceString(basic["projectCompany"]), HouseholdCount: sourceIntPointer(basic["households"]), ClosedManagement: sourceString(basic["isClosed"]),
		PlotRatio: sourceFloatPointer(basic["plotRatio"]), GreenAreaSQM: sourceFloatPointer(basic["greenArea"]), GreeningRatePercent: sourceFloatPointer(basic["greeningRate"]),
		PropertyManagementCompany: sourceString(basic["propertyManageCompany"]), PropertyFee: sourceString(basic["propertyFee"]),
		FixedParkingSpaces: sourceIntPointer(basic["fixedParkingSpace"]), ParkingRatio: sourceString(basic["parkingRate"]), ParkingFee: sourceString(basic["parkingFee"]),
		HeatingType: sourceString(basic["heatingType"]), WaterType: sourceString(basic["waterType"]), ElectricityType: sourceString(basic["electricityType"]),
		GasCost: sourceString(basic["gasCost"]), ManCarSeparation: sourceString(basic["manCar"]), Latitude: sourceFloat(basic["gaodeLat"]), Longitude: sourceFloat(basic["gaodeLng"]),
		ListingAvgUnitPrice: sourceFloatPointer(mapRow["latestListingAvgPrice"]), ListingCount: sourceIntPointer(mapRow["listingNum"]),
		ListingAreaSQM: sourceFloatPointer(mapRow["listingArea"]), ListingAvgTotalPriceWan: sourceFloatPointer(mapRow["listingAvgNumPrice"]),
		ListingAvgUnitPrice6Months: sourceFloatPointer(mapRow["listingAvgPrice6month"]), NewListingCount3Months: sourceIntPointer(mapRow["newListingNum3month"]),
		NewListingAvgTotalPrice3MonthsWan: sourceFloatPointer(mapRow["newListingAvgNumPrice3month"]), NewListingUnitPrice3Months: sourceFloatPointer(mapRow["newListingUnitPrice3month"]),
		LatestTradeAvgUnitPrice: sourceFloatPointer(mapRow["latestTradeAvgPrice"]), TradeCount3Months: sourceIntPointer(mapRow["tradeNum3month"]),
		TradeArea3MonthsSQM: sourceFloatPointer(mapRow["tradeArea3month"]), TradeAvgTotalPrice3MonthsWan: sourceFloatPointer(mapRow["tradeAvgNumPrice3month"]),
		TradeUnitPrice3Months: sourceFloatPointer(mapRow["tradeUnitPrice3month"]), TradeAvgUnitPrice6Months: sourceFloatPointer(mapRow["tradeAvgPrice6month"]),
		TradeCountPerMonth6Months: sourceFloatPointer(mapRow["tradeNumPerMonth6month"]), TakeLookCount: sourceIntPointer(mapRow["takeLook"]),
		TakeLookConversionRate: sourceFloatPointer(mapRow["takeLookTransRate"]), OnSaleAreaRange: sourceString(mapRow["onSaleAreaSeg"]),
		OnSalePriceRange: sourceString(mapRow["onSalePriceSeg"]), OnSaleRoomTypes: sourceStringSlice(mapRow["onSaleRoomType"]),
		Analysis: analysis, Surroundings: surroundings, CityContext: cityContext,
	}
	data.LatestListingDate = optionalSourceDate(mapRow["latestListingDateOrigin"])
	data.LatestTradeDate = optionalSourceDate(mapRow["latestTradeDateOrigin"])
	return data.Normalize(), nil
}

func normalizeListingRows(rows []map[string]any, collectedAt time.Time) ([]MarketListing, error) {
	items := make([]MarketListing, 0, len(rows))
	for _, row := range rows {
		listedAt, err := sourceDate(row["autualListingDate"])
		if err != nil {
			return nil, err
		}
		days := int(collectedAt.Sub(listedAt).Hours() / 24)
		if days < 0 {
			days = 0
		}
		items = append(items, MarketListing{
			RoomID: sourceString(row["roomId"]), Layout: normalizeLayout(sourceString(row["roomType"])),
			AreaSQM: sourceFloat(row["listingArea"]), ListingTotalPriceWan: sourceFloat(row["listingTotalPrice"]),
			ListingUnitPrice: sourceFloat(row["listingUnitPrice"]), ListedAt: listedAt, DaysOnMarket: days,
			FloorBand: sourceString(row["currentFloor"]), FloorDescription: sourceString(row["onFloor"]), Orientation: sourceString(row["orientation"]),
			AdjustmentCount: sourceInt(row["adjustNum"]), FollowCount: sourceInt(row["followAll"]), LookCount30Days: sourceInt(row["takeLookThirty"]),
		})
	}
	return items, nil
}

func normalizeTransactionRows(rows []map[string]any) ([]MarketTransaction, error) {
	items := make([]MarketTransaction, 0, len(rows))
	for _, row := range rows {
		tradeDate, err := sourceDate(row["tradeDate"])
		if err != nil {
			return nil, err
		}
		items = append(items, MarketTransaction{
			RoomID: sourceString(row["roomId"]), Layout: normalizeLayout(sourceString(row["roomType"])), AreaSQM: sourceFloat(row["tradeArea"]),
			ListingTotalPriceWan: sourceFloat(row["listingPrice"]), TradeTotalPriceWan: sourceFloat(row["tradePrice"]), TradeUnitPrice: sourceFloat(row["tradeAvgPrice"]),
			TradeDate: tradeDate, NegotiationWan: sourceFloat(row["premium"]), NegotiationPercent: sourceFloat(row["premiumSpace"]),
			FloorBand: sourceString(row["currentFloor"]), FloorDescription: sourceString(row["onFloor"]), Orientation: sourceString(row["orientation"]),
			AdjustmentCount: sourceInt(row["adjustNum"]),
		})
	}
	return items, nil
}

func decodeObjectRows(raw json.RawMessage) ([]map[string]any, error) {
	var rows []map[string]any
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.UseNumber()
	if err := decoder.Decode(&rows); err != nil {
		return nil, fmt.Errorf("%w: expected array data", ErrFangjianResponse)
	}
	return rows, nil
}

func findMapCommunity(raw json.RawMessage, communityID string) (map[string]any, error) {
	var root map[string]any
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.UseNumber()
	if err := decoder.Decode(&root); err != nil {
		return nil, err
	}
	esfs, _ := root["esfs"].(map[string]any)
	rows, _ := esfs["rows"].([]any)
	for _, item := range rows {
		row, _ := item.(map[string]any)
		if sourceString(row["aurCommunityId"]) == communityID {
			return row, nil
		}
	}
	return nil, fmt.Errorf("community %s absent from map response: %w", communityID, ErrFangjianResponse)
}

func mergeRows(existing, incoming []map[string]any) []map[string]any {
	byID := make(map[string]map[string]any, len(existing)+len(incoming))
	for _, row := range append(append([]map[string]any(nil), existing...), incoming...) {
		if id := sourceString(row["roomId"]); id != "" {
			byID[id] = row
		}
	}
	ids := make([]string, 0, len(byID))
	for id := range byID {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	rows := make([]map[string]any, 0, len(ids))
	for _, id := range ids {
		rows = append(rows, byID[id])
	}
	return rows
}

func deduplicateAdjustments(items []ListingAdjustment) []ListingAdjustment {
	seen := make(map[string]struct{}, len(items))
	result := make([]ListingAdjustment, 0, len(items))
	for _, item := range items {
		key := fmt.Sprintf("%s|%s|%.4f|%.4f", item.RoomID, item.AdjustedAt.Format(time.DateOnly), item.PriceBeforeWan, item.PriceAfterWan)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, item)
	}
	return normalizeAdjustments(result)
}

func sourceRoomTypes(rows []map[string]any) []string {
	set := map[string]struct{}{}
	for _, roomType := range []string{"一室", "二室", "三室", "四室", "五室", "六室", "七室", "八室", "九室"} {
		set[roomType] = struct{}{}
	}
	for _, row := range rows {
		if value := sourceString(row["roomTypeFilter"]); value != "" {
			set[value] = struct{}{}
		}
	}
	values := make([]string, 0, len(set))
	for value := range set {
		values = append(values, value)
	}
	sort.Strings(values)
	return values
}

func sourceString(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case json.Number:
		return typed.String()
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case nil:
		return ""
	default:
		return strings.TrimSpace(fmt.Sprint(typed))
	}
}

func firstString(values ...any) string {
	for _, value := range values {
		if text := sourceString(value); text != "" {
			return text
		}
	}
	return ""
}

func sourceFloat(value any) float64 {
	parsed, _ := strconv.ParseFloat(strings.TrimSpace(sourceString(value)), 64)
	return parsed
}

func sourceInt(value any) int {
	parsed, _ := strconv.ParseFloat(strings.TrimSpace(sourceString(value)), 64)
	return int(parsed)
}

func sourceFloatPointer(value any) *float64 {
	if sourceString(value) == "" {
		return nil
	}
	parsed := sourceFloat(value)
	return &parsed
}

func sourceIntPointer(value any) *int {
	if sourceString(value) == "" {
		return nil
	}
	parsed := sourceInt(value)
	return &parsed
}

func sourceStringSlice(value any) []string {
	switch typed := value.(type) {
	case []any:
		result := make([]string, 0, len(typed))
		for _, item := range typed {
			if text := sourceString(item); text != "" {
				result = append(result, text)
			}
		}
		return result
	case string:
		fields := strings.FieldsFunc(typed, func(r rune) bool { return r == ',' || r == '、' || r == ';' || r == '|' })
		result := make([]string, 0, len(fields))
		for _, item := range fields {
			if item = strings.TrimSpace(item); item != "" {
				result = append(result, item)
			}
		}
		return result
	default:
		return nil
	}
}

func sourceDate(value any) (time.Time, error) {
	text := strings.TrimSpace(sourceString(value))
	for _, layout := range []string{"2006.01.02", "2006-01-02", time.RFC3339} {
		if parsed, err := time.Parse(layout, text); err == nil {
			return parsed.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid Fangjian date %q: %w", text, ErrFangjianResponse)
}

func optionalSourceDate(value any) *time.Time {
	if sourceString(value) == "" {
		return nil
	}
	parsed, err := sourceDate(value)
	if err != nil {
		return nil
	}
	return &parsed
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func uniqueStrings(values []string) []string {
	set := map[string]struct{}{}
	for _, value := range values {
		set[value] = struct{}{}
	}
	result := make([]string, 0, len(set))
	for value := range set {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func safeArchiveKey(value string) string {
	replacer := strings.NewReplacer("/", "-", " ", "-", "\\", "-", "..", "-")
	return replacer.Replace(value)
}

func fangjianMapInput(config FangjianCommunityConfig) map[string]any {
	lng, lat := sourceFloat(config.Longitude), sourceFloat(config.Latitude)
	geo := fmt.Sprintf("%.6f,%.6f;%.6f,%.6f;%.6f,%.6f;%.6f,%.6f;%.6f,%.6f;", lng-.008, lat-.008, lng+.008, lat-.008, lng+.008, lat+.008, lng-.008, lat+.008, lng-.008, lat-.008)
	return map[string]any{
		"assetMapLandReq":    map[string]any{"pageIndex": 1, "pageSize": 100},
		"assetMapProjectReq": map[string]any{"pageIndex": 1, "pageSize": 100},
		"searchMapReq":       map[string]any{"type": "1", "centerLng": "", "centerLat": "", "distance": "", "geoStr": geo},
		"blocks":             []any{}, "cityCode": config.CityCode, "districts": []any{}, "keyword": "", "types": []int{1},
	}
}

func competitiveInput(config FangjianCommunityConfig) map[string]any {
	return map[string]any{
		"code":         config.CommunityID,
		"landParam":    map[string]any{"timeRange": []any{}, "tradeStatus": []string{"正在交易", "未上市", "已成交"}, "landUsages": []string{"综合用地(含住宅)", "住宅用地"}, "minTradeFloorPrice": "", "maxTradeFloorPrice": "", "getLandCompanys": []any{}, "cityCode": config.CityCode, "pageIndex": 1, "pageSize": 100, "sortType": "", "sortRule": "asc"},
		"projectParam": map[string]any{"saleStatus": []string{"未售", "在售", "售罄"}, "timeRange": []any{}, "minTradeAvgPriceThreeMonth": "", "maxTradeAvgPriceThreeMonth": "", "groupDeveloper": []any{}, "cityCode": config.CityCode, "pageIndex": 1, "pageSize": 100, "sortType": "", "sortRule": "asc"},
		"esfParam":     map[string]any{"buildTypes": []string{"板楼", "塔楼", "塔板结合"}, "buildYearRange": "", "pageIndex": 1, "pageSize": 100, "sortType": "", "sortRule": "asc"},
		"searchMap":    map[string]any{"type": "0", "centerLng": config.Longitude, "centerLat": config.Latitude, "geoStr": "", "distance": 3000},
	}
}

func poiInput(config FangjianCommunityConfig) map[string]any {
	categories := map[string][]string{
		"交通": {"地铁站", "公交站", "火车站", "飞机场"}, "商业": {"购物中心", "大型超市"},
		"教育": {"小学", "中学", "大学"}, "医疗": {"综合医院", "专科医院"}, "景观": {"景观", "公园"},
		"劣势": {"丧葬设施", "传染病医院", "寺庙教堂"},
	}
	order := []string{"交通", "商业", "教育", "医疗", "景观", "劣势"}
	requests := make([]map[string]any, 0, len(order))
	for _, category := range order {
		children := make([]map[string]any, 0, len(categories[category]))
		for _, child := range categories[category] {
			children = append(children, map[string]any{"type": child})
		}
		requests = append(requests, map[string]any{"bizType": category, "pageIndex": 1, "pageSize": 10, "sortType": 0, "sortRule": "asc", "poiType": []any{map[string]any{"type": category, "childType": children}}})
	}
	return map[string]any{
		"searchMap":      map[string]any{"type": "0", "centerLng": config.Longitude, "centerLat": config.Latitude, "geoStr": "", "distance": "3000"},
		"landmarkStatus": "", "itemSearchReq": requests, "showLoading": false,
	}
}
