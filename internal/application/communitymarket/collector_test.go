package communitymarket

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

type collectorClientStub struct {
	post func(string, any) (json.RawMessage, error)
}

func (*collectorClientStub) Get(context.Context, string) (json.RawMessage, error) {
	return fangjianEnvelope(map[string]any{}), nil
}

func (s *collectorClientStub) Post(_ context.Context, path string, input any) (json.RawMessage, error) {
	return s.post(path, input)
}

type archiveStub struct{}

func (archiveStub) Write(context.Context, CollectedCommunity) (string, error) { return "archive", nil }

func TestCollectorSplitsImplicitHundredRowLimitAndDeduplicates(t *testing.T) {
	client := &collectorClientStub{post: func(path string, input any) (json.RawMessage, error) {
		if path != "/esf/listingRecord" {
			return nil, fmt.Errorf("unexpected path %s", path)
		}
		request := input.(map[string]any)
		if got := request["latestListingDate"]; got != "2026-06-01" {
			t.Fatalf("latestListingDate = %#v, want 2026-06-01", got)
		}
		roomType, _ := request["roomTypeFilter"].(string)
		var rows []map[string]any
		switch roomType {
		case "":
			rows = listingFixtureRows(0, 100, "1室")
		case "1室", "一室":
			rows = listingFixtureRows(0, 60, "1室")
		case "2室", "二室":
			rows = listingFixtureRows(60, 119, "2室")
		default:
			rows = []map[string]any{}
		}
		return fangjianEnvelope(rows), nil
	}}
	collector := NewCollector(client, archiveStub{}, nil)
	raw := map[string]json.RawMessage{}
	endpoints := []string{}
	_, result, err := collector.collectRecordRows(context.Background(), raw, &endpoints, "listing", DefaultFangjianCommunities[0], time.Date(2026, 7, 17, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("collectRecordRows() error = %v", err)
	}
	listings := result.([]MarketListing)
	if len(listings) != 119 {
		t.Fatalf("listing count = %d, want 119", len(listings))
	}
	if len(raw) < 10 || listings[0].Layout != "一室" || listings[0].DaysOnMarket != 16 {
		t.Fatalf("raw/listing normalization = %d/%#v", len(raw), listings[0])
	}
}

func TestCollectorStopsWhenLeafStillReturnsHundredRows(t *testing.T) {
	client := &collectorClientStub{post: func(_ string, input any) (json.RawMessage, error) {
		request := input.(map[string]any)
		roomType, _ := request["roomTypeFilter"].(string)
		floor, _ := request["currentFloor"].(string)
		if roomType == "" || roomType == "1室" || (roomType == "1室" && floor == "高楼层") {
			return fangjianEnvelope(listingFixtureRows(0, 100, "1室")), nil
		}
		return fangjianEnvelope([]map[string]any{}), nil
	}}
	collector := NewCollector(client, archiveStub{}, nil)
	_, _, err := collector.collectRecordRows(context.Background(), map[string]json.RawMessage{}, &[]string{}, "listing", DefaultFangjianCommunities[0], time.Now())
	if !errors.Is(err, ErrFangjianIncomplete) {
		t.Fatalf("collectRecordRows() error = %v, want incomplete", err)
	}
}

func TestFangjianDataRejectsBusinessFailureImmediately(t *testing.T) {
	body, _ := json.Marshal(map[string]any{"code": 401, "description": "expired", "data": nil})
	_, err := fangjianData(body)
	if !errors.Is(err, ErrFangjianResponse) {
		t.Fatalf("fangjianData() error = %v", err)
	}
	if !strings.Contains(err.Error(), "expired") {
		t.Fatalf("fangjianData() error = %v, want description", err)
	}
}

func TestTrimmedHARFixturesNormalizeDatesAmountsNullsAndAdjustments(t *testing.T) {
	listingBody, err := os.ReadFile("testdata/listing-record.json")
	if err != nil {
		t.Fatalf("ReadFile(listing) error = %v", err)
	}
	listingData, err := fangjianData(listingBody)
	if err != nil {
		t.Fatalf("fangjianData(listing) error = %v", err)
	}
	listingRows, err := decodeObjectRows(listingData)
	if err != nil {
		t.Fatalf("decodeObjectRows(listing) error = %v", err)
	}
	listings, err := normalizeListingRows(listingRows, time.Date(2026, 7, 17, 0, 0, 0, 0, time.UTC))
	if err != nil || len(listings) != 1 || listings[0].Layout != "二室" || listings[0].ListingTotalPriceWan != 63 || listings[0].FollowCount != 0 || listings[0].DaysOnMarket != 19 {
		t.Fatalf("normalizeListingRows() = %#v, %v", listings, err)
	}

	tradeBody, _ := os.ReadFile("testdata/trade-record.json")
	tradeData, _ := fangjianData(tradeBody)
	tradeRows, _ := decodeObjectRows(tradeData)
	transactions, err := normalizeTransactionRows(tradeRows)
	if err != nil || len(transactions) != 1 || transactions[0].TradeTotalPriceWan != 45 || transactions[0].NegotiationPercent != 15.09 || transactions[0].TradeDate.Format(time.DateOnly) != "2026-06-14" {
		t.Fatalf("normalizeTransactionRows() = %#v, %v", transactions, err)
	}

	adjustBody, _ := os.ReadFile("testdata/adjust-record.json")
	adjustData, _ := fangjianData(adjustBody)
	adjustRows, _ := decodeObjectRows(adjustData)
	adjustedAt, _ := sourceDate(adjustRows[0]["adjustDate"])
	adjustments := deduplicateAdjustments([]ListingAdjustment{
		{RoomID: "101138410213", AdjustedAt: adjustedAt, PriceBeforeWan: sourceFloat(adjustRows[0]["listingPriceAdjustBefore"]), PriceAfterWan: sourceFloat(adjustRows[0]["listingPriceAdjustAfter"]), AmountWan: sourceFloat(adjustRows[0]["adjustAmount"])},
		{RoomID: "101138410213", AdjustedAt: adjustedAt, PriceBeforeWan: 65, PriceAfterWan: 63, AmountWan: -2},
	})
	if len(adjustments) != 1 || adjustments[0].AmountWan != -2 {
		t.Fatalf("deduplicateAdjustments() = %#v", adjustments)
	}
}

func listingFixtureRows(start, end int, roomType string) []map[string]any {
	rows := make([]map[string]any, 0, end-start)
	for index := start; index < end; index++ {
		rows = append(rows, map[string]any{
			"roomId": fmt.Sprintf("room-%03d", index), "roomType": roomType + "1厅", "roomTypeFilter": roomType,
			"listingArea": 70, "listingTotalPrice": 60, "listingUnitPrice": 8571,
			"autualListingDate": "2026.07.01", "currentFloor": "高楼层", "onFloor": "高楼层(共18层)",
			"orientation": "南", "adjustNum": 0, "followAll": 0, "takeLookThirty": 0,
		})
	}
	return rows
}

func fangjianEnvelope(data any) json.RawMessage {
	body, _ := json.Marshal(map[string]any{"code": 200, "description": "success", "data": data})
	return body
}
