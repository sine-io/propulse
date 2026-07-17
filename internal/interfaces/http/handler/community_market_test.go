package handler

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	appcommunitymarket "github.com/sine-io/propulse/internal/application/communitymarket"
	domaincommunitymarket "github.com/sine-io/propulse/internal/domain/communitymarket"
)

func TestParseCommunityMarketCSVPreservesAggregateUnits(t *testing.T) {
	header := append([]string(nil), communityMarketCSVHeaders...)
	row := communityMarketV1Row()
	raw := communityMarketCSV(t, header, row)

	collectedAt, data, issues := parseCommunityMarketCSV(raw)
	if len(issues) != 0 {
		t.Fatalf("issues = %#v", issues)
	}
	if collectedAt.Format("2006-01-02T15:04:05Z") != "2026-07-16T12:58:57Z" {
		t.Fatalf("collectedAt = %s", collectedAt)
	}
	if data.ListingAvgUnitPrice == nil || *data.ListingAvgUnitPrice != 22741 || data.ListingCount == nil || *data.ListingCount != 46 {
		t.Fatalf("listing aggregate = %#v", data)
	}
	if data.LatestTradeDate == nil || data.LatestTradeDate.Format("2006-01-02") != "2026-06-12" {
		t.Fatalf("latest trade date = %#v", data.LatestTradeDate)
	}
	if data.TradeCount3Months == nil || *data.TradeCount3Months != 5 || data.TradeUnitPrice3Months == nil || *data.TradeUnitPrice3Months != 16780 {
		t.Fatalf("trade aggregate = %#v", data)
	}
	if data.BuildingCount != nil || data.PropertyType != "" || data.PropertyTags != nil {
		t.Fatalf("v1 profile fields = %#v, want absent", data)
	}
}

func TestParseCommunityMarketCSVV2PreservesCompleteProfileAndSourceText(t *testing.T) {
	row := append(communityMarketV1Row(),
		"120000", "天津市", "普通住宅", "商品房,私产", "11", "板楼", "2012",
		"天津耀华投资发展有限公司", "1089", "是", "1.80", "14271.00", "40.00",
		"天津碧桂园物业有限公司", "2.3-2.9", "1550.00", "1:0.7", "500", "集中供暖",
		"民水", "民电", "2.5-2.61", "否",
	)
	_, data, issues := parseCommunityMarketCSV(communityMarketCSV(t, communityMarketCSVV2Headers, row))
	if len(issues) != 0 {
		t.Fatalf("issues = %#v", issues)
	}
	if data.PropertyType != "普通住宅" || len(data.PropertyTags) != 2 || data.PropertyTags[1] != "私产" {
		t.Fatalf("property identity = %#v", data)
	}
	if data.BuildingCount == nil || *data.BuildingCount != 11 || data.BuildingYear == nil || *data.BuildingYear != 2012 || data.HouseholdCount == nil || *data.HouseholdCount != 1089 {
		t.Fatalf("building profile = %#v", data)
	}
	if data.PlotRatio == nil || *data.PlotRatio != 1.8 || data.GreenAreaSQM == nil || *data.GreenAreaSQM != 14271 || data.GreeningRatePercent == nil || *data.GreeningRatePercent != 40 {
		t.Fatalf("site profile = %#v", data)
	}
	if data.PropertyFee != "2.3-2.9" || data.ParkingFee != "500" || data.GasCost != "2.5-2.61" {
		t.Fatalf("source text profile = %#v", data)
	}
	if data.FixedParkingSpaces == nil || *data.FixedParkingSpaces != 1550 || data.ClosedManagement != "是" || data.ManCarSeparation != "否" {
		t.Fatalf("parking and management profile = %#v", data)
	}
}

func TestParseCommunityMarketCSVRejectsSchemaDriftAndMultipleRows(t *testing.T) {
	header := append([]string(nil), communityMarketCSVHeaders...)
	header[0] = "capturedAt"
	_, _, issues := parseCommunityMarketCSV(communityMarketCSV(t, header, make([]string, len(header))))
	if len(issues) != 1 || issues[0].Code != "invalid_header" {
		t.Fatalf("header issues = %#v", issues)
	}

	validHeader := append([]string(nil), communityMarketCSVHeaders...)
	row := make([]string, len(validHeader))
	var body bytes.Buffer
	writer := csv.NewWriter(&body)
	_ = writer.Write(validHeader)
	_ = writer.Write(row)
	_ = writer.Write(row)
	writer.Flush()
	_, _, issues = parseCommunityMarketCSV(body.Bytes())
	if len(issues) != 1 || issues[0].Code != "too_many_rows" {
		t.Fatalf("row issues = %#v", issues)
	}
}

func TestParseCommunityMarketCSVRejectsInvalidUTF8AndDates(t *testing.T) {
	_, _, issues := parseCommunityMarketCSV([]byte{0xff, 0xfe})
	if len(issues) != 1 || issues[0].Code != "invalid_encoding" {
		t.Fatalf("encoding issues = %#v", issues)
	}

	row := communityMarketV1Row()
	row[12] = "2026/99/01"
	_, _, issues = parseCommunityMarketCSV(communityMarketCSV(t, communityMarketCSVHeaders, row))
	if len(issues) != 1 || issues[0].Code != "invalid_date" || issues[0].Field != "latestListingDate" {
		t.Fatalf("date issues = %#v", issues)
	}
}

func TestCommunityMarketImportCSVReturnsCreatedAndIdempotentReplay(t *testing.T) {
	for _, test := range []struct {
		name       string
		replay     bool
		wantStatus int
	}{
		{name: "created", wantStatus: http.StatusCreated},
		{name: "replay", replay: true, wantStatus: http.StatusOK},
	} {
		t.Run(test.name, func(t *testing.T) {
			app := &stubCommunityMarketApplication{result: communityMarketImportResult(test.replay)}
			engine := gin.New()
			engine.POST("/imports", NewCommunityMarket(app).ImportCSV)
			recorder := httptest.NewRecorder()
			engine.ServeHTTP(recorder, newCommunityMarketImportRequest(t, communityMarketCSV(t, communityMarketCSVV2Headers, append(
				communityMarketV1Row(),
				"120000", "天津市", "普通住宅", "商品房,私产", "11", "板楼", "2012",
				"开发商", "1089", "是", "1.8", "14271", "40", "物业公司", "2.3-2.9",
				"1550", "1:0.7", "500", "集中供暖", "民水", "民电", "2.5-2.61", "否",
			))))

			if recorder.Code != test.wantStatus || app.importCalls != 1 {
				t.Fatalf("status/calls = %d/%d; body=%s", recorder.Code, app.importCalls, recorder.Body.String())
			}
			if !strings.Contains(recorder.Body.String(), `"idempotentReplay":`+strconv.FormatBool(test.replay)) {
				t.Fatalf("body = %s", recorder.Body.String())
			}
			if app.command.Data.BuildingCount == nil || *app.command.Data.BuildingCount != 11 || app.command.Data.PropertyFee != "2.3-2.9" {
				t.Fatalf("command profile = %#v", app.command.Data)
			}
		})
	}
}

func TestCommunityMarketImportCSVMapsValidationNotFoundAndSizeFailures(t *testing.T) {
	t.Run("validation", func(t *testing.T) {
		app := &stubCommunityMarketApplication{}
		engine := gin.New()
		engine.POST("/imports", NewCommunityMarket(app).ImportCSV)
		recorder := httptest.NewRecorder()
		engine.ServeHTTP(recorder, newCommunityMarketImportRequest(t, []byte("wrong\nvalue\n")))
		if recorder.Code != http.StatusUnprocessableEntity || app.importCalls != 0 {
			t.Fatalf("status/calls = %d/%d; body=%s", recorder.Code, app.importCalls, recorder.Body.String())
		}
	})

	t.Run("not found", func(t *testing.T) {
		app := &stubCommunityMarketApplication{importErr: appcommunitymarket.ErrNeighborhoodNotFound}
		engine := gin.New()
		engine.POST("/imports", NewCommunityMarket(app).ImportCSV)
		recorder := httptest.NewRecorder()
		engine.ServeHTTP(recorder, newCommunityMarketImportRequest(t, communityMarketCSV(t, communityMarketCSVHeaders, communityMarketV1Row())))
		if recorder.Code != http.StatusNotFound || app.importCalls != 1 {
			t.Fatalf("status/calls = %d/%d; body=%s", recorder.Code, app.importCalls, recorder.Body.String())
		}
	})

	t.Run("too large", func(t *testing.T) {
		app := &stubCommunityMarketApplication{}
		engine := gin.New()
		engine.POST("/imports", NewCommunityMarket(app).ImportCSV)
		recorder := httptest.NewRecorder()
		engine.ServeHTTP(recorder, newCommunityMarketImportRequest(t, bytes.Repeat([]byte("x"), maxImportBytes+1)))
		if recorder.Code != http.StatusRequestEntityTooLarge || app.importCalls != 0 {
			t.Fatalf("status/calls = %d/%d; body=%s", recorder.Code, app.importCalls, recorder.Body.String())
		}
	})
}

func TestCommunityMarketSnapshotResponseMapsNullableProfile(t *testing.T) {
	snapshot := communityMarketImportResult(false).Snapshot
	buildingCount := 11
	snapshot.Data.ProvinceName = "天津市"
	snapshot.Data.PropertyTags = []string{"商品房", "私产"}
	snapshot.Data.BuildingCount = &buildingCount
	response := newCommunityMarketSnapshotResponse(snapshot)
	if response.ProvinceName == nil || *response.ProvinceName != "天津市" || response.PropertyTags == nil || len(*response.PropertyTags) != 2 {
		t.Fatalf("profile response = %#v", response)
	}
	if response.BuildingCount == nil || *response.BuildingCount != 11 {
		t.Fatalf("building count response = %#v", response.BuildingCount)
	}

	empty := newCommunityMarketSnapshotResponse(appcommunitymarket.Snapshot{})
	if empty.ProvinceName != nil || empty.PropertyTags != nil || empty.BuildingCount != nil {
		t.Fatalf("empty profile response = %#v, want null profile fields", empty)
	}
	if empty.OnSaleRoomTypes == nil || string(empty.Analysis) != "{}" || empty.QualityStatus != "aggregate_only" {
		t.Fatalf("empty collection response = %#v, want stable empty arrays/objects and quality", empty)
	}
}

func TestCommunityMarketImportFangjianAcceptsCompleteBundle(t *testing.T) {
	gin.SetMode(gin.TestMode)
	app := &stubCommunityMarketApplication{fangjianResult: appcommunitymarket.ImportFangjianResult{
		Snapshot:        appcommunitymarket.Snapshot{ID: "88888888-8888-4888-8888-888888888888"},
		CollectionRunID: "77777777-7777-4777-8777-777777777777", ListingCount: 1, TransactionCount: 1, AdjustmentCount: 1,
	}}
	engine := gin.New()
	engine.POST("/admin/api/community-market/imports/fangjian", NewCommunityMarket(app).ImportFangjian)
	requestBody, _ := json.Marshal(map[string]any{
		"dataSourceId":   "11111111-1111-4111-8111-111111111111",
		"neighborhoodId": "22222222-2222-4222-8222-222222222222", "sourceRef": "fangjian-test",
		"bundle": map[string]any{
			"schemaVersion": "fangjian.bundle/v1", "collectedAt": "2026-07-17T00:00:00Z",
			"community": map[string]any{
				"sourceCommunityId": "source", "communityName": "测试花园", "cityCode": "120100", "cityName": "天津市",
				"districtCode": "120111", "districtName": "西青区", "blockCode": "block", "blockName": "大寺",
				"latitude": 39, "longitude": 117, "analysis": map[string]any{}, "surroundings": map[string]any{}, "cityContext": map[string]any{},
			},
			"listings": []any{}, "transactions": []any{}, "adjustments": []any{},
			"quality": map[string]any{"status": "complete", "warnings": []any{}},
		},
	})
	request := httptest.NewRequest(http.MethodPost, "/admin/api/community-market/imports/fangjian", bytes.NewReader(requestBody))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusCreated || app.fangjianCalls != 1 {
		t.Fatalf("status/calls/body = %d/%d/%s", recorder.Code, app.fangjianCalls, recorder.Body.String())
	}
	if app.fangjianCommand.Bundle.SchemaVersion != appcommunitymarket.FangjianBundleSchemaVersion || len(app.fangjianCommand.RawPayload) == 0 {
		t.Fatalf("command = %#v", app.fangjianCommand)
	}
}

func TestCommunityMarketListingHandlerParsesFilters(t *testing.T) {
	gin.SetMode(gin.TestMode)
	app := &stubCommunityMarketApplication{listingResult: appcommunitymarket.Page[appcommunitymarket.MarketListing]{Items: []appcommunitymarket.MarketListing{}, Page: 2, PageSize: 10}}
	engine := gin.New()
	engine.GET("/api/v1/neighborhoods/:id/market-listings", NewCommunityMarket(app).ListListings)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/neighborhoods/22222222-2222-4222-8222-222222222222/market-listings?layout=2%E5%AE%A41%E5%8E%85&floor=%E9%AB%98%E6%A5%BC%E5%B1%82&minPriceWan=50&maxPriceWan=100&sortBy=price&sortOrder=asc&page=2&pageSize=10", nil)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || app.listingQuery.Page != 2 || app.listingQuery.PageSize != 10 || app.listingQuery.SortBy != "price" || app.listingQuery.MinPriceWan == nil || *app.listingQuery.MinPriceWan != 50 {
		t.Fatalf("status/query/body = %d/%#v/%s", recorder.Code, app.listingQuery, recorder.Body.String())
	}
}

func TestCommunityMarketGetListingReturnsCollectionMetadata(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	app := &stubCommunityMarketApplication{listingDetail: appcommunitymarket.MarketListingDetail{
		MarketListing:  appcommunitymarket.MarketListing{RoomID: "room-1", Layout: "3室2厅", AreaSQM: 118, ListingTotalPriceWan: 500},
		NeighborhoodID: "22222222-2222-4222-8222-222222222222", NeighborhoodName: "海河花园",
		Status: "active", SnapshotID: "33333333-3333-4333-8333-333333333333",
		CollectionRunID: "44444444-4444-4444-8444-444444444444", CollectedAt: time.Date(2026, 7, 17, 8, 0, 0, 0, time.UTC),
		Source:        appcommunitymarket.MarketSource{DataSourceID: "55555555-5555-4555-8555-555555555555", DataSourceName: "房鉴", DataSourceType: "fangjian", SourceRef: "batch-1"},
		QualityStatus: "complete",
	}}
	engine := gin.New()
	engine.GET("/api/v1/neighborhoods/:id/market-listings/:roomId", NewCommunityMarket(app).GetListing)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/neighborhoods/22222222-2222-4222-8222-222222222222/market-listings/room-1", nil)
	engine.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || !bytes.Contains(recorder.Body.Bytes(), []byte(`"collectionRunId":"44444444-4444-4444-8444-444444444444"`)) ||
		!bytes.Contains(recorder.Body.Bytes(), []byte(`"dataSourceName":"房鉴"`)) {
		t.Fatalf("status/body = %d/%s", recorder.Code, recorder.Body.String())
	}
}

type stubCommunityMarketApplication struct {
	command          appcommunitymarket.ImportSnapshotCommand
	result           appcommunitymarket.ImportSnapshotResult
	importErr        error
	importCalls      int
	fangjianCommand  appcommunitymarket.ImportFangjianCommand
	fangjianResult   appcommunitymarket.ImportFangjianResult
	fangjianErr      error
	fangjianCalls    int
	listingQuery     appcommunitymarket.MarketListQuery
	listingResult    appcommunitymarket.Page[appcommunitymarket.MarketListing]
	listingDetail    appcommunitymarket.MarketListingDetail
	listingDetailErr error
}

func (s *stubCommunityMarketApplication) ImportSnapshot(_ context.Context, command appcommunitymarket.ImportSnapshotCommand) (appcommunitymarket.ImportSnapshotResult, error) {
	s.importCalls++
	s.command = command
	return s.result, s.importErr
}

func (*stubCommunityMarketApplication) LatestSnapshot(context.Context, appcommunitymarket.LatestSnapshotQuery) (appcommunitymarket.Snapshot, error) {
	return appcommunitymarket.Snapshot{}, appcommunitymarket.ErrSnapshotNotFound
}

func (s *stubCommunityMarketApplication) ImportFangjian(_ context.Context, command appcommunitymarket.ImportFangjianCommand) (appcommunitymarket.ImportFangjianResult, error) {
	s.fangjianCalls++
	s.fangjianCommand = command
	return s.fangjianResult, s.fangjianErr
}

func (s *stubCommunityMarketApplication) ListListings(_ context.Context, query appcommunitymarket.MarketListQuery) (appcommunitymarket.Page[appcommunitymarket.MarketListing], error) {
	s.listingQuery = query
	return s.listingResult, nil
}

func (s *stubCommunityMarketApplication) GetListing(context.Context, appcommunitymarket.GetListingQuery) (appcommunitymarket.MarketListingDetail, error) {
	return s.listingDetail, s.listingDetailErr
}

func (*stubCommunityMarketApplication) ListTransactions(context.Context, appcommunitymarket.MarketListQuery) (appcommunitymarket.Page[appcommunitymarket.MarketTransaction], error) {
	return appcommunitymarket.Page[appcommunitymarket.MarketTransaction]{}, nil
}

func (*stubCommunityMarketApplication) ListingAdjustments(context.Context, appcommunitymarket.ListingAdjustmentsQuery) ([]appcommunitymarket.ListingAdjustment, error) {
	return nil, nil
}

func (*stubCommunityMarketApplication) Compare(context.Context, appcommunitymarket.ComparisonQuery) (appcommunitymarket.Comparison, error) {
	return appcommunitymarket.Comparison{}, nil
}

func communityMarketImportResult(replay bool) appcommunitymarket.ImportSnapshotResult {
	return appcommunitymarket.ImportSnapshotResult{
		IdempotentReplay: replay,
		Snapshot: appcommunitymarket.Snapshot{
			ID:              "88888888-8888-4888-8888-888888888888",
			DataSourceID:    "11111111-1111-4111-8111-111111111111",
			NeighborhoodID:  "22222222-2222-4222-8222-222222222222",
			SourceRef:       "fangjian-test",
			CollectedAt:     time.Date(2026, 7, 16, 12, 58, 57, 0, time.UTC),
			ContentChecksum: strings.Repeat("a", 64),
			CreatedAt:       time.Date(2026, 7, 16, 13, 0, 0, 0, time.UTC),
			Data: domaincommunitymarket.SnapshotData{
				SourceCommunityID: "source-community",
				CommunityName:     "鸣泉花园",
				OnSaleRoomTypes:   []string{},
			},
		},
	}
}

func newCommunityMarketImportRequest(t *testing.T, raw []byte) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for name, value := range map[string]string{
		"dataSourceId":   "11111111-1111-4111-8111-111111111111",
		"neighborhoodId": "22222222-2222-4222-8222-222222222222",
		"sourceRef":      "fangjian-test",
	} {
		if err := writer.WriteField(name, value); err != nil {
			t.Fatal(err)
		}
	}
	part, err := writer.CreateFormFile("file", "fangjian.csv")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(raw); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/imports", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	return request
}

func communityMarketV1Row() []string {
	return []string{
		"2026-07-16T12:58:57Z", "a2d56505411446cfe70fd3960beb19c7", "富力津门湖鸣泉花园", "鸣泉花园",
		"120100", "天津市", "120111", "西青区", "BK2022112435579", "梅江南",
		"39.057089", "117.203624", "2026-06-29", "22741", "46", "5491.78", "272", "22884.52",
		"9", "270", "20425", "2026.06.12", "16461", "5", "639.37", "215", "16780", "17371",
		"1", "125", "3.25", "84-229", "149-479", "五室,四室,二室,三室",
	}
}

func communityMarketCSV(t *testing.T, rows ...[]string) []byte {
	t.Helper()
	var body bytes.Buffer
	writer := csv.NewWriter(&body)
	for _, row := range rows {
		if err := writer.Write(row); err != nil {
			t.Fatal(err)
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		t.Fatal(err)
	}
	return body.Bytes()
}
