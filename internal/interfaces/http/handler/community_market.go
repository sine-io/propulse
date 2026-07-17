package handler

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	appcommunitymarket "github.com/sine-io/propulse/internal/application/communitymarket"
	domaincommunitymarket "github.com/sine-io/propulse/internal/domain/communitymarket"
)

var communityMarketCSVHeaders = []string{
	"collectedAt",
	"aurCommunityId",
	"communityName",
	"formerName",
	"cityCode",
	"cityName",
	"districtCode",
	"districtName",
	"blockCode",
	"blockName",
	"latitude",
	"longitude",
	"latestListingDate",
	"latestListingAvgPriceYuanPerSqm",
	"listingNum",
	"listingAreaSqm",
	"listingAvgTotalPriceWan",
	"listingAvgPrice6monthYuanPerSqm",
	"newListingNum3month",
	"newListingAvgTotalPrice3monthWan",
	"newListingUnitPrice3monthYuanPerSqm",
	"latestTradeDate",
	"latestTradeAvgPriceYuanPerSqm",
	"tradeNum3month",
	"tradeArea3monthSqm",
	"tradeAvgTotalPrice3monthWan",
	"tradeUnitPrice3monthYuanPerSqm",
	"tradeAvgPrice6monthYuanPerSqm",
	"tradeNumPerMonth6month",
	"takeLook",
	"takeLookTransRatePercent",
	"onSaleAreaSegSqm",
	"onSalePriceSegWan",
	"onSaleRoomType",
}

var communityMarketCSVV2Headers = append(append([]string(nil), communityMarketCSVHeaders...),
	"provinceCode",
	"provinceName",
	"propertyType",
	"propertyTags",
	"buildingCount",
	"buildingType",
	"buildingYear",
	"developer",
	"householdCount",
	"closedManagement",
	"plotRatio",
	"greenAreaSqm",
	"greeningRatePercent",
	"propertyManagementCompany",
	"propertyFee",
	"fixedParkingSpaces",
	"parkingRatio",
	"parkingFee",
	"heatingType",
	"waterType",
	"electricityType",
	"gasCost",
	"manCarSeparation",
)

type CommunityMarketApplication interface {
	ImportSnapshot(context.Context, appcommunitymarket.ImportSnapshotCommand) (appcommunitymarket.ImportSnapshotResult, error)
	ImportFangjian(context.Context, appcommunitymarket.ImportFangjianCommand) (appcommunitymarket.ImportFangjianResult, error)
	LatestSnapshot(context.Context, appcommunitymarket.LatestSnapshotQuery) (appcommunitymarket.Snapshot, error)
	ListListings(context.Context, appcommunitymarket.MarketListQuery) (appcommunitymarket.Page[appcommunitymarket.MarketListing], error)
	GetListing(context.Context, appcommunitymarket.GetListingQuery) (appcommunitymarket.MarketListingDetail, error)
	ListTransactions(context.Context, appcommunitymarket.MarketListQuery) (appcommunitymarket.Page[appcommunitymarket.MarketTransaction], error)
	ListingAdjustments(context.Context, appcommunitymarket.ListingAdjustmentsQuery) ([]appcommunitymarket.ListingAdjustment, error)
	Compare(context.Context, appcommunitymarket.ComparisonQuery) (appcommunitymarket.Comparison, error)
}

type CommunityMarket struct {
	app CommunityMarketApplication
}

func NewCommunityMarket(app CommunityMarketApplication) CommunityMarket {
	return CommunityMarket{app: app}
}

type communityMarketSnapshotResponse struct {
	ID                                string          `json:"id"`
	DataSourceID                      string          `json:"dataSourceId"`
	NeighborhoodID                    string          `json:"neighborhoodId"`
	SourceRef                         string          `json:"sourceRef"`
	CollectedAt                       string          `json:"collectedAt"`
	ContentChecksum                   string          `json:"contentChecksum"`
	CollectionRunID                   *string         `json:"collectionRunId"`
	QualityStatus                     string          `json:"qualityStatus"`
	SourceCommunityID                 string          `json:"sourceCommunityId"`
	CommunityName                     string          `json:"communityName"`
	FormerName                        string          `json:"formerName"`
	ProvinceCode                      *string         `json:"provinceCode"`
	ProvinceName                      *string         `json:"provinceName"`
	CityCode                          string          `json:"cityCode"`
	CityName                          string          `json:"cityName"`
	DistrictCode                      string          `json:"districtCode"`
	DistrictName                      string          `json:"districtName"`
	BlockCode                         string          `json:"blockCode"`
	BlockName                         string          `json:"blockName"`
	PropertyType                      *string         `json:"propertyType"`
	PropertyTags                      *[]string       `json:"propertyTags"`
	BuildingCount                     *int            `json:"buildingCount"`
	BuildingType                      *string         `json:"buildingType"`
	BuildingYear                      *int            `json:"buildingYear"`
	Developer                         *string         `json:"developer"`
	HouseholdCount                    *int            `json:"householdCount"`
	ClosedManagement                  *string         `json:"closedManagement"`
	PlotRatio                         *float64        `json:"plotRatio"`
	GreenAreaSQM                      *float64        `json:"greenAreaSqm"`
	GreeningRatePercent               *float64        `json:"greeningRatePercent"`
	PropertyManagementCompany         *string         `json:"propertyManagementCompany"`
	PropertyFee                       *string         `json:"propertyFee"`
	FixedParkingSpaces                *int            `json:"fixedParkingSpaces"`
	ParkingRatio                      *string         `json:"parkingRatio"`
	ParkingFee                        *string         `json:"parkingFee"`
	HeatingType                       *string         `json:"heatingType"`
	WaterType                         *string         `json:"waterType"`
	ElectricityType                   *string         `json:"electricityType"`
	GasCost                           *string         `json:"gasCost"`
	ManCarSeparation                  *string         `json:"manCarSeparation"`
	Latitude                          float64         `json:"latitude"`
	Longitude                         float64         `json:"longitude"`
	LatestListingDate                 *string         `json:"latestListingDate"`
	ListingAvgUnitPrice               *float64        `json:"listingAvgUnitPrice"`
	ListingCount                      *int            `json:"listingCount"`
	ListingAreaSQM                    *float64        `json:"listingAreaSqm"`
	ListingAvgTotalPriceWan           *float64        `json:"listingAvgTotalPriceWan"`
	ListingAvgUnitPrice6Months        *float64        `json:"listingAvgUnitPrice6Months"`
	NewListingCount3Months            *int            `json:"newListingCount3Months"`
	NewListingAvgTotalPrice3MonthsWan *float64        `json:"newListingAvgTotalPrice3MonthsWan"`
	NewListingUnitPrice3Months        *float64        `json:"newListingUnitPrice3Months"`
	LatestTradeDate                   *string         `json:"latestTradeDate"`
	LatestTradeAvgUnitPrice           *float64        `json:"latestTradeAvgUnitPrice"`
	TradeCount3Months                 *int            `json:"tradeCount3Months"`
	TradeArea3MonthsSQM               *float64        `json:"tradeArea3MonthsSqm"`
	TradeAvgTotalPrice3MonthsWan      *float64        `json:"tradeAvgTotalPrice3MonthsWan"`
	TradeUnitPrice3Months             *float64        `json:"tradeUnitPrice3Months"`
	TradeAvgUnitPrice6Months          *float64        `json:"tradeAvgUnitPrice6Months"`
	TradeCountPerMonth6Months         *float64        `json:"tradeCountPerMonth6Months"`
	TakeLookCount                     *int            `json:"takeLookCount"`
	TakeLookConversionRate            *float64        `json:"takeLookConversionRatePercent"`
	OnSaleAreaRange                   string          `json:"onSaleAreaRangeSqm"`
	OnSalePriceRange                  string          `json:"onSalePriceRangeWan"`
	OnSaleRoomTypes                   []string        `json:"onSaleRoomTypes"`
	Analysis                          json.RawMessage `json:"analysis"`
	Surroundings                      json.RawMessage `json:"surroundings"`
	CityContext                       json.RawMessage `json:"cityContext"`
	CreatedAt                         string          `json:"createdAt"`
}

type importCommunityMarketResponse struct {
	Snapshot         communityMarketSnapshotResponse `json:"snapshot"`
	IdempotentReplay bool                            `json:"idempotentReplay"`
}

type importFangjianRequest struct {
	DataSourceID   string                            `json:"dataSourceId"`
	NeighborhoodID string                            `json:"neighborhoodId"`
	SourceRef      string                            `json:"sourceRef"`
	Bundle         appcommunitymarket.FangjianBundle `json:"bundle"`
}

type importFangjianResponse struct {
	Snapshot         communityMarketSnapshotResponse `json:"snapshot"`
	CollectionRunID  string                          `json:"collectionRunId"`
	ListingCount     int                             `json:"listingCount"`
	TransactionCount int                             `json:"transactionCount"`
	AdjustmentCount  int                             `json:"adjustmentCount"`
	IdempotentReplay bool                            `json:"idempotentReplay"`
}

type communityMarketComparisonResponse struct {
	Primary           communityMarketSnapshotResponse     `json:"primary"`
	Peer              communityMarketSnapshotResponse     `json:"peer"`
	ListingUnitPrice  appcommunitymarket.ComparisonMetric `json:"listingUnitPrice"`
	Supply            appcommunitymarket.ComparisonMetric `json:"supply"`
	RecentTrades      appcommunitymarket.ComparisonMetric `json:"recentTrades"`
	ListingTradeGap   appcommunitymarket.ComparisonMetric `json:"listingTradeGap"`
	AverageTradeCycle appcommunitymarket.ComparisonMetric `json:"averageTradeCycle"`
}

func (h CommunityMarket) ImportCSV(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxMultipartImportBytes)
	if err := c.Request.ParseMultipartForm(64 << 10); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			writeError(c, http.StatusRequestEntityTooLarge, "payload_too_large", "multipart request exceeds 2 MiB plus metadata allowance")
			return
		}
		writeError(c, http.StatusBadRequest, "invalid_request", "multipart request is invalid")
		return
	}
	defer func() {
		if err := c.Request.MultipartForm.RemoveAll(); err != nil {
			c.Set("multipart_cleanup_error", err)
		}
	}()

	files := c.Request.MultipartForm.File["file"]
	if len(files) != 1 {
		writeError(c, http.StatusBadRequest, "invalid_request", "exactly one community market CSV file is required")
		return
	}
	if files[0].Size > maxImportBytes {
		writeError(c, http.StatusRequestEntityTooLarge, "payload_too_large", "CSV file exceeds 2 MiB")
		return
	}
	file, err := files[0].Open()
	if err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", "CSV file cannot be read")
		return
	}
	defer func() { _ = file.Close() }()
	raw, err := io.ReadAll(io.LimitReader(file, maxImportBytes+1))
	if err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", "CSV file cannot be read")
		return
	}
	if len(raw) > maxImportBytes {
		writeError(c, http.StatusRequestEntityTooLarge, "payload_too_large", "CSV file exceeds 2 MiB")
		return
	}

	collectedAt, data, issues := parseCommunityMarketCSV(raw)
	if len(issues) > 0 {
		writeCommunityMarketValidationError(c, issues)
		return
	}
	result, err := h.app.ImportSnapshot(c.Request.Context(), appcommunitymarket.ImportSnapshotCommand{
		DataSourceID:   c.PostForm("dataSourceId"),
		NeighborhoodID: c.PostForm("neighborhoodId"),
		SourceRef:      c.PostForm("sourceRef"),
		CollectedAt:    collectedAt,
		RawPayload:     append([]byte(nil), raw...),
		RawContentType: "text/csv",
		Data:           data,
	})
	if err != nil {
		var validationErr *appcommunitymarket.ValidationError
		switch {
		case errors.As(err, &validationErr):
			writeCommunityMarketValidationError(c, validationErr.Issues)
		case errors.Is(err, appcommunitymarket.ErrDataSourceNotFound), errors.Is(err, appcommunitymarket.ErrNeighborhoodNotFound):
			writeError(c, http.StatusNotFound, "not_found", "selected data source or neighborhood was not found")
		default:
			writeError(c, http.StatusInternalServerError, "import_failed", "community market import failed")
		}
		return
	}
	status := http.StatusCreated
	if result.IdempotentReplay {
		status = http.StatusOK
	}
	c.JSON(status, importCommunityMarketResponse{
		Snapshot:         newCommunityMarketSnapshotResponse(result.Snapshot),
		IdempotentReplay: result.IdempotentReplay,
	})
}

func (h CommunityMarket) ImportFangjian(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, appcommunitymarket.MaxFangjianBundleBytes+(64<<10))
	raw, err := io.ReadAll(c.Request.Body)
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			writeError(c, http.StatusRequestEntityTooLarge, "payload_too_large", "Fangjian import exceeds 16 MiB")
			return
		}
		writeError(c, http.StatusBadRequest, "invalid_request", "request body could not be read")
		return
	}
	var request importFangjianRequest
	if !validFangjianImportShape(raw) {
		writeError(c, http.StatusBadRequest, "invalid_request", "request is missing required Fangjian bundle fields")
		return
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", "request must be a valid Fangjian import document")
		return
	}
	if err := ensureJSONEOF(decoder); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", "request must contain one JSON document")
		return
	}
	bundlePayload, err := json.Marshal(request.Bundle)
	if err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", "bundle could not be normalized")
		return
	}
	result, err := h.app.ImportFangjian(c.Request.Context(), appcommunitymarket.ImportFangjianCommand{
		DataSourceID: request.DataSourceID, NeighborhoodID: request.NeighborhoodID,
		SourceRef: request.SourceRef, RawPayload: bundlePayload, Bundle: request.Bundle,
	})
	if err != nil {
		writeCommunityMarketApplicationError(c, err, "Fangjian import failed")
		return
	}
	status := http.StatusCreated
	if result.IdempotentReplay {
		status = http.StatusOK
	}
	c.JSON(status, importFangjianResponse{
		Snapshot: newCommunityMarketSnapshotResponse(result.Snapshot), CollectionRunID: result.CollectionRunID,
		ListingCount: result.ListingCount, TransactionCount: result.TransactionCount,
		AdjustmentCount: result.AdjustmentCount, IdempotentReplay: result.IdempotentReplay,
	})
}

func (h CommunityMarket) GetLatest(c *gin.Context) {
	snapshot, err := h.app.LatestSnapshot(c.Request.Context(), appcommunitymarket.LatestSnapshotQuery{
		NeighborhoodID: c.Param("id"),
	})
	if err != nil {
		if errors.Is(err, appcommunitymarket.ErrSnapshotNotFound) || errors.Is(err, appcommunitymarket.ErrNeighborhoodNotFound) {
			writeError(c, http.StatusNotFound, "not_found", "community market snapshot not found")
			return
		}
		writeError(c, http.StatusInternalServerError, "internal_error", "internal server error")
		return
	}
	c.JSON(http.StatusOK, newCommunityMarketSnapshotResponse(snapshot))
}

func (h CommunityMarket) ListListings(c *gin.Context) {
	query, err := marketListQuery(c)
	if err != nil {
		writeError(c, http.StatusBadRequest, "invalid_query", "market listing filters are invalid")
		return
	}
	result, err := h.app.ListListings(c.Request.Context(), query)
	if err != nil {
		writeCommunityMarketQueryError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h CommunityMarket) GetListing(c *gin.Context) {
	result, err := h.app.GetListing(c.Request.Context(), appcommunitymarket.GetListingQuery{
		NeighborhoodID: c.Param("id"), RoomID: c.Param("roomId"),
	})
	if err != nil {
		writeCommunityMarketQueryError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h CommunityMarket) ListTransactions(c *gin.Context) {
	query, err := marketListQuery(c)
	if err != nil {
		writeError(c, http.StatusBadRequest, "invalid_query", "market transaction filters are invalid")
		return
	}
	result, err := h.app.ListTransactions(c.Request.Context(), query)
	if err != nil {
		writeCommunityMarketQueryError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h CommunityMarket) ListAdjustments(c *gin.Context) {
	items, err := h.app.ListingAdjustments(c.Request.Context(), appcommunitymarket.ListingAdjustmentsQuery{
		NeighborhoodID: c.Param("id"), RoomID: c.Param("roomId"),
	})
	if err != nil {
		writeCommunityMarketQueryError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h CommunityMarket) Compare(c *gin.Context) {
	result, err := h.app.Compare(c.Request.Context(), appcommunitymarket.ComparisonQuery{
		NeighborhoodID: c.Query("neighborhoodId"), PeerNeighborhoodID: c.Query("peerNeighborhoodId"),
	})
	if err != nil {
		writeCommunityMarketQueryError(c, err)
		return
	}
	c.JSON(http.StatusOK, communityMarketComparisonResponse{
		Primary: newCommunityMarketSnapshotResponse(result.Primary), Peer: newCommunityMarketSnapshotResponse(result.Peer),
		ListingUnitPrice: result.ListingUnitPrice, Supply: result.Supply, RecentTrades: result.RecentTrades,
		ListingTradeGap: result.ListingTradeGap, AverageTradeCycle: result.AverageTradeCycle,
	})
}

func marketListQuery(c *gin.Context) (appcommunitymarket.MarketListQuery, error) {
	query := appcommunitymarket.MarketListQuery{
		NeighborhoodID: c.Param("id"), Layout: c.Query("layout"), Floor: c.Query("floor"),
		SortBy: c.Query("sortBy"), SortOrder: c.Query("sortOrder"),
	}
	var err error
	query.MinPriceWan, err = optionalQueryFloat(c.Query("minPriceWan"))
	if err != nil {
		return appcommunitymarket.MarketListQuery{}, err
	}
	query.MaxPriceWan, err = optionalQueryFloat(c.Query("maxPriceWan"))
	if err != nil {
		return appcommunitymarket.MarketListQuery{}, err
	}
	if value := c.Query("page"); value != "" {
		query.Page, err = strconv.Atoi(value)
		if err != nil {
			return appcommunitymarket.MarketListQuery{}, err
		}
	}
	if value := c.Query("pageSize"); value != "" {
		query.PageSize, err = strconv.Atoi(value)
		if err != nil {
			return appcommunitymarket.MarketListQuery{}, err
		}
	}
	return query, nil
}

func optionalQueryFloat(value string) (*float64, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil || math.IsNaN(parsed) || math.IsInf(parsed, 0) {
		return nil, errors.New("invalid number")
	}
	return &parsed, nil
}

func writeCommunityMarketApplicationError(c *gin.Context, err error, fallback string) {
	var validationErr *appcommunitymarket.ValidationError
	switch {
	case errors.As(err, &validationErr):
		writeCommunityMarketValidationError(c, validationErr.Issues)
	case errors.Is(err, appcommunitymarket.ErrDataSourceNotFound), errors.Is(err, appcommunitymarket.ErrNeighborhoodNotFound):
		writeError(c, http.StatusNotFound, "not_found", "selected data source or neighborhood was not found")
	default:
		writeError(c, http.StatusInternalServerError, "import_failed", fallback)
	}
}

func writeCommunityMarketQueryError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, appcommunitymarket.ErrInvalidQuery):
		writeError(c, http.StatusBadRequest, "invalid_query", "community market query is invalid")
	case errors.Is(err, appcommunitymarket.ErrListingUnavailable):
		writeError(c, http.StatusNotFound, "listing_unavailable", "the listing is no longer in the latest active inventory")
	case errors.Is(err, appcommunitymarket.ErrListingNotFound):
		writeError(c, http.StatusNotFound, "not_found", "market listing was not found")
	case errors.Is(err, appcommunitymarket.ErrSnapshotNotFound), errors.Is(err, appcommunitymarket.ErrNeighborhoodNotFound):
		writeError(c, http.StatusNotFound, "not_found", "complete community market data was not found")
	default:
		writeError(c, http.StatusInternalServerError, "internal_error", "community market query failed")
	}
}

func ensureJSONEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return errors.New("extra JSON document")
	}
	return nil
}

func validFangjianImportShape(raw []byte) bool {
	var root map[string]json.RawMessage
	if json.Unmarshal(raw, &root) != nil {
		return false
	}
	for _, key := range []string{"dataSourceId", "neighborhoodId", "sourceRef", "bundle"} {
		if len(root[key]) == 0 || string(root[key]) == "null" {
			return false
		}
	}
	var bundle map[string]json.RawMessage
	if json.Unmarshal(root["bundle"], &bundle) != nil {
		return false
	}
	for _, key := range []string{"schemaVersion", "collectedAt", "community", "listings", "transactions", "adjustments", "quality"} {
		if len(bundle[key]) == 0 || string(bundle[key]) == "null" {
			return false
		}
	}
	return true
}

func parseCommunityMarketCSV(raw []byte) (time.Time, domaincommunitymarket.SnapshotData, []appcommunitymarket.ValidationIssue) {
	if !utf8.Valid(raw) {
		return time.Time{}, domaincommunitymarket.SnapshotData{}, []appcommunitymarket.ValidationIssue{{
			Field: "file", Code: "invalid_encoding", Message: "CSV file must be valid UTF-8",
		}}
	}
	reader := csv.NewReader(bytes.NewReader(bytes.TrimPrefix(raw, []byte{0xef, 0xbb, 0xbf})))
	reader.FieldsPerRecord = -1
	header, err := reader.Read()
	if errors.Is(err, io.EOF) {
		return time.Time{}, domaincommunitymarket.SnapshotData{}, []appcommunitymarket.ValidationIssue{{
			Field: "header", Code: "required", Message: "CSV header is required",
		}}
	}
	if err != nil {
		return time.Time{}, domaincommunitymarket.SnapshotData{}, []appcommunitymarket.ValidationIssue{communityMarketCSVIssue(1, "invalid_csv", err.Error())}
	}
	if issue := validateCommunityMarketHeader(header); issue != nil {
		return time.Time{}, domaincommunitymarket.SnapshotData{}, []appcommunitymarket.ValidationIssue{*issue}
	}
	reader.FieldsPerRecord = len(header)
	row, err := reader.Read()
	if errors.Is(err, io.EOF) {
		return time.Time{}, domaincommunitymarket.SnapshotData{}, []appcommunitymarket.ValidationIssue{communityMarketCSVIssue(2, "required", "exactly one data row is required")}
	}
	if err != nil {
		return time.Time{}, domaincommunitymarket.SnapshotData{}, []appcommunitymarket.ValidationIssue{communityMarketCSVIssue(2, "invalid_csv", err.Error())}
	}
	if _, err := reader.Read(); !errors.Is(err, io.EOF) {
		return time.Time{}, domaincommunitymarket.SnapshotData{}, []appcommunitymarket.ValidationIssue{communityMarketCSVIssue(3, "too_many_rows", "community market CSV must contain exactly one data row")}
	}

	values := make(map[string]string, len(header))
	for index, name := range header {
		values[name] = strings.TrimSpace(row[index])
	}
	issues := make([]appcommunitymarket.ValidationIssue, 0)
	collectedAt, err := time.Parse(time.RFC3339, values["collectedAt"])
	if err != nil {
		issues = append(issues, communityMarketCSVIssue(2, "invalid_datetime", "collectedAt must use RFC3339"))
		issues[len(issues)-1].Field = "collectedAt"
	}
	latitude, issue := requiredCommunityMarketFloat(values["latitude"], "latitude")
	if issue != nil {
		issues = append(issues, *issue)
	}
	longitude, issue := requiredCommunityMarketFloat(values["longitude"], "longitude")
	if issue != nil {
		issues = append(issues, *issue)
	}
	latestListingDate, issue := optionalCommunityMarketDate(values["latestListingDate"], "latestListingDate")
	if issue != nil {
		issues = append(issues, *issue)
	}
	latestTradeDate, issue := optionalCommunityMarketDate(values["latestTradeDate"], "latestTradeDate")
	if issue != nil {
		issues = append(issues, *issue)
	}

	data := domaincommunitymarket.SnapshotData{
		SourceCommunityID:         values["aurCommunityId"],
		CommunityName:             values["communityName"],
		FormerName:                values["formerName"],
		ProvinceCode:              values["provinceCode"],
		ProvinceName:              values["provinceName"],
		CityCode:                  values["cityCode"],
		CityName:                  values["cityName"],
		DistrictCode:              values["districtCode"],
		DistrictName:              values["districtName"],
		BlockCode:                 values["blockCode"],
		BlockName:                 values["blockName"],
		PropertyType:              values["propertyType"],
		PropertyTags:              splitCommunityMarketList(values["propertyTags"]),
		BuildingType:              values["buildingType"],
		Developer:                 values["developer"],
		ClosedManagement:          values["closedManagement"],
		PropertyManagementCompany: values["propertyManagementCompany"],
		PropertyFee:               values["propertyFee"],
		ParkingRatio:              values["parkingRatio"],
		ParkingFee:                values["parkingFee"],
		HeatingType:               values["heatingType"],
		WaterType:                 values["waterType"],
		ElectricityType:           values["electricityType"],
		GasCost:                   values["gasCost"],
		ManCarSeparation:          values["manCarSeparation"],
		Latitude:                  latitude,
		Longitude:                 longitude,
		LatestListingDate:         latestListingDate,
		LatestTradeDate:           latestTradeDate,
		OnSaleAreaRange:           values["onSaleAreaSegSqm"],
		OnSalePriceRange:          values["onSalePriceSegWan"],
		OnSaleRoomTypes:           strings.Split(values["onSaleRoomType"], ","),
	}

	floatFields := []struct {
		name   string
		target **float64
	}{
		{name: "latestListingAvgPriceYuanPerSqm", target: &data.ListingAvgUnitPrice},
		{name: "listingAreaSqm", target: &data.ListingAreaSQM},
		{name: "listingAvgTotalPriceWan", target: &data.ListingAvgTotalPriceWan},
		{name: "listingAvgPrice6monthYuanPerSqm", target: &data.ListingAvgUnitPrice6Months},
		{name: "newListingAvgTotalPrice3monthWan", target: &data.NewListingAvgTotalPrice3MonthsWan},
		{name: "newListingUnitPrice3monthYuanPerSqm", target: &data.NewListingUnitPrice3Months},
		{name: "latestTradeAvgPriceYuanPerSqm", target: &data.LatestTradeAvgUnitPrice},
		{name: "tradeArea3monthSqm", target: &data.TradeArea3MonthsSQM},
		{name: "tradeAvgTotalPrice3monthWan", target: &data.TradeAvgTotalPrice3MonthsWan},
		{name: "tradeUnitPrice3monthYuanPerSqm", target: &data.TradeUnitPrice3Months},
		{name: "tradeAvgPrice6monthYuanPerSqm", target: &data.TradeAvgUnitPrice6Months},
		{name: "tradeNumPerMonth6month", target: &data.TradeCountPerMonth6Months},
		{name: "takeLookTransRatePercent", target: &data.TakeLookConversionRate},
		{name: "plotRatio", target: &data.PlotRatio},
		{name: "greenAreaSqm", target: &data.GreenAreaSQM},
		{name: "greeningRatePercent", target: &data.GreeningRatePercent},
	}
	for _, field := range floatFields {
		value, issue := optionalCommunityMarketFloat(values[field.name], field.name)
		if issue != nil {
			issues = append(issues, *issue)
		}
		*field.target = value
	}
	intFields := []struct {
		name   string
		target **int
	}{
		{name: "listingNum", target: &data.ListingCount},
		{name: "newListingNum3month", target: &data.NewListingCount3Months},
		{name: "tradeNum3month", target: &data.TradeCount3Months},
		{name: "takeLook", target: &data.TakeLookCount},
		{name: "buildingCount", target: &data.BuildingCount},
		{name: "buildingYear", target: &data.BuildingYear},
		{name: "householdCount", target: &data.HouseholdCount},
		{name: "fixedParkingSpaces", target: &data.FixedParkingSpaces},
	}
	for _, field := range intFields {
		value, issue := optionalCommunityMarketInt(values[field.name], field.name)
		if issue != nil {
			issues = append(issues, *issue)
		}
		*field.target = value
	}
	return collectedAt, data, issues
}

func validateCommunityMarketHeader(header []string) *appcommunitymarket.ValidationIssue {
	if sameCommunityMarketHeader(header, communityMarketCSVHeaders) || sameCommunityMarketHeader(header, communityMarketCSVV2Headers) {
		return nil
	}
	issue := communityMarketCSVIssue(1, "invalid_header", fmt.Sprintf(
		"CSV header must exactly match the supported v1 (%d columns) or v2 (%d columns) schema",
		len(communityMarketCSVHeaders), len(communityMarketCSVV2Headers),
	))
	issue.Field = "header"
	return &issue
}

func sameCommunityMarketHeader(actual, expected []string) bool {
	if len(actual) != len(expected) {
		return false
	}
	for index := range expected {
		if actual[index] != expected[index] {
			return false
		}
	}
	return true
}

func splitCommunityMarketList(raw string) []string {
	if raw == "" {
		return nil
	}
	return strings.Split(raw, ",")
}

func requiredCommunityMarketFloat(raw, field string) (float64, *appcommunitymarket.ValidationIssue) {
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		issue := communityMarketCSVIssue(2, "invalid_number", field+" must be a number")
		issue.Field = field
		return 0, &issue
	}
	return value, nil
}

func optionalCommunityMarketFloat(raw, field string) (*float64, *appcommunitymarket.ValidationIssue) {
	if raw == "" {
		return nil, nil
	}
	value, issue := requiredCommunityMarketFloat(raw, field)
	if issue != nil {
		return nil, issue
	}
	return &value, nil
}

func optionalCommunityMarketInt(raw, field string) (*int, *appcommunitymarket.ValidationIssue) {
	if raw == "" {
		return nil, nil
	}
	value, err := strconv.ParseInt(raw, 10, 32)
	if err == nil {
		converted := int(value)
		return &converted, nil
	}
	decimal, decimalErr := strconv.ParseFloat(raw, 64)
	if decimalErr != nil || math.IsNaN(decimal) || math.IsInf(decimal, 0) || math.Trunc(decimal) != decimal || decimal < math.MinInt32 || decimal > math.MaxInt32 {
		issue := communityMarketCSVIssue(2, "invalid_integer", field+" must be an integer")
		issue.Field = field
		return nil, &issue
	}
	converted := int(decimal)
	return &converted, nil
}

func optionalCommunityMarketDate(raw, field string) (*time.Time, *appcommunitymarket.ValidationIssue) {
	if raw == "" {
		return nil, nil
	}
	normalized := strings.ReplaceAll(raw, ".", "-")
	value, err := time.Parse(time.DateOnly, normalized)
	if err != nil {
		issue := communityMarketCSVIssue(2, "invalid_date", field+" must use YYYY-MM-DD")
		issue.Field = field
		return nil, &issue
	}
	return &value, nil
}

func communityMarketCSVIssue(row int, code, message string) appcommunitymarket.ValidationIssue {
	return appcommunitymarket.ValidationIssue{Row: &row, Field: "file", Code: code, Message: message}
}

func writeCommunityMarketValidationError(c *gin.Context, issues []appcommunitymarket.ValidationIssue) {
	c.JSON(http.StatusUnprocessableEntity, gin.H{"error": gin.H{
		"code": "validation_failed", "message": "one or more community market fields are invalid", "details": issues,
	}})
}

func newCommunityMarketSnapshotResponse(snapshot appcommunitymarket.Snapshot) communityMarketSnapshotResponse {
	return communityMarketSnapshotResponse{
		ID:                                snapshot.ID,
		DataSourceID:                      snapshot.DataSourceID,
		NeighborhoodID:                    snapshot.NeighborhoodID,
		SourceRef:                         snapshot.SourceRef,
		CollectedAt:                       snapshot.CollectedAt.UTC().Format(time.RFC3339),
		ContentChecksum:                   snapshot.ContentChecksum,
		CollectionRunID:                   snapshot.CollectionRunID,
		QualityStatus:                     communityMarketQualityStatus(snapshot.QualityStatus),
		SourceCommunityID:                 snapshot.Data.SourceCommunityID,
		CommunityName:                     snapshot.Data.CommunityName,
		FormerName:                        snapshot.Data.FormerName,
		ProvinceCode:                      optionalCommunityMarketResponseString(snapshot.Data.ProvinceCode),
		ProvinceName:                      optionalCommunityMarketResponseString(snapshot.Data.ProvinceName),
		CityCode:                          snapshot.Data.CityCode,
		CityName:                          snapshot.Data.CityName,
		DistrictCode:                      snapshot.Data.DistrictCode,
		DistrictName:                      snapshot.Data.DistrictName,
		BlockCode:                         snapshot.Data.BlockCode,
		BlockName:                         snapshot.Data.BlockName,
		PropertyType:                      optionalCommunityMarketResponseString(snapshot.Data.PropertyType),
		PropertyTags:                      optionalCommunityMarketResponseStrings(snapshot.Data.PropertyTags),
		BuildingCount:                     snapshot.Data.BuildingCount,
		BuildingType:                      optionalCommunityMarketResponseString(snapshot.Data.BuildingType),
		BuildingYear:                      snapshot.Data.BuildingYear,
		Developer:                         optionalCommunityMarketResponseString(snapshot.Data.Developer),
		HouseholdCount:                    snapshot.Data.HouseholdCount,
		ClosedManagement:                  optionalCommunityMarketResponseString(snapshot.Data.ClosedManagement),
		PlotRatio:                         snapshot.Data.PlotRatio,
		GreenAreaSQM:                      snapshot.Data.GreenAreaSQM,
		GreeningRatePercent:               snapshot.Data.GreeningRatePercent,
		PropertyManagementCompany:         optionalCommunityMarketResponseString(snapshot.Data.PropertyManagementCompany),
		PropertyFee:                       optionalCommunityMarketResponseString(snapshot.Data.PropertyFee),
		FixedParkingSpaces:                snapshot.Data.FixedParkingSpaces,
		ParkingRatio:                      optionalCommunityMarketResponseString(snapshot.Data.ParkingRatio),
		ParkingFee:                        optionalCommunityMarketResponseString(snapshot.Data.ParkingFee),
		HeatingType:                       optionalCommunityMarketResponseString(snapshot.Data.HeatingType),
		WaterType:                         optionalCommunityMarketResponseString(snapshot.Data.WaterType),
		ElectricityType:                   optionalCommunityMarketResponseString(snapshot.Data.ElectricityType),
		GasCost:                           optionalCommunityMarketResponseString(snapshot.Data.GasCost),
		ManCarSeparation:                  optionalCommunityMarketResponseString(snapshot.Data.ManCarSeparation),
		Latitude:                          snapshot.Data.Latitude,
		Longitude:                         snapshot.Data.Longitude,
		LatestListingDate:                 formatOptionalDate(snapshot.Data.LatestListingDate),
		ListingAvgUnitPrice:               snapshot.Data.ListingAvgUnitPrice,
		ListingCount:                      snapshot.Data.ListingCount,
		ListingAreaSQM:                    snapshot.Data.ListingAreaSQM,
		ListingAvgTotalPriceWan:           snapshot.Data.ListingAvgTotalPriceWan,
		ListingAvgUnitPrice6Months:        snapshot.Data.ListingAvgUnitPrice6Months,
		NewListingCount3Months:            snapshot.Data.NewListingCount3Months,
		NewListingAvgTotalPrice3MonthsWan: snapshot.Data.NewListingAvgTotalPrice3MonthsWan,
		NewListingUnitPrice3Months:        snapshot.Data.NewListingUnitPrice3Months,
		LatestTradeDate:                   formatOptionalDate(snapshot.Data.LatestTradeDate),
		LatestTradeAvgUnitPrice:           snapshot.Data.LatestTradeAvgUnitPrice,
		TradeCount3Months:                 snapshot.Data.TradeCount3Months,
		TradeArea3MonthsSQM:               snapshot.Data.TradeArea3MonthsSQM,
		TradeAvgTotalPrice3MonthsWan:      snapshot.Data.TradeAvgTotalPrice3MonthsWan,
		TradeUnitPrice3Months:             snapshot.Data.TradeUnitPrice3Months,
		TradeAvgUnitPrice6Months:          snapshot.Data.TradeAvgUnitPrice6Months,
		TradeCountPerMonth6Months:         snapshot.Data.TradeCountPerMonth6Months,
		TakeLookCount:                     snapshot.Data.TakeLookCount,
		TakeLookConversionRate:            snapshot.Data.TakeLookConversionRate,
		OnSaleAreaRange:                   snapshot.Data.OnSaleAreaRange,
		OnSalePriceRange:                  snapshot.Data.OnSalePriceRange,
		OnSaleRoomTypes:                   append(make([]string, 0, len(snapshot.Data.OnSaleRoomTypes)), snapshot.Data.OnSaleRoomTypes...),
		Analysis:                          communityMarketJSONObject(snapshot.Data.Analysis),
		Surroundings:                      communityMarketJSONObject(snapshot.Data.Surroundings),
		CityContext:                       communityMarketJSONObject(snapshot.Data.CityContext),
		CreatedAt:                         snapshot.CreatedAt.UTC().Format(time.RFC3339),
	}
}

func formatOptionalDate(value *time.Time) *string {
	if value == nil {
		return nil
	}
	formatted := value.UTC().Format(time.DateOnly)
	return &formatted
}

func optionalCommunityMarketResponseString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func optionalCommunityMarketResponseStrings(values []string) *[]string {
	if len(values) == 0 {
		return nil
	}
	copyOfValues := append([]string(nil), values...)
	return &copyOfValues
}

func communityMarketJSONObject(value json.RawMessage) json.RawMessage {
	if len(value) == 0 {
		return json.RawMessage(`{}`)
	}
	return append(json.RawMessage(nil), value...)
}

func communityMarketQualityStatus(value string) string {
	if value == "" {
		return "aggregate_only"
	}
	return value
}
