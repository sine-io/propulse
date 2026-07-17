package asset

import (
	"testing"
	"time"
)

func TestNewNormalizesAndValidatesManualAsset(t *testing.T) {
	now := time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC)
	price := 320.0
	got, err := New(Draft{
		ID: "11111111-1111-4111-8111-111111111111", UserID: " user-1 ", Name: " 海河花园 两室 ",
		Property: PropertySnapshot{
			NeighborhoodID: "22222222-2222-4222-8222-222222222222", NeighborhoodName: " 海河花园 ",
			City: " 天津 ", District: " 河西区 ", Layout: " 2室1厅 ", AreaSQM: 82.5,
			CurrentListingPriceWan: &price,
		},
		OriginalPurchasePriceWan: 180, PurchasedOn: time.Date(2020, 8, 20, 16, 0, 0, 0, time.Local),
		CurrentLoanBalanceWan: 60, SourceKind: SourceManual,
	}, now)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if got.UserID != "user-1" || got.Name != "海河花园 两室" || got.Property.Layout != "2室1厅" {
		t.Fatalf("normalized asset = %#v", got)
	}
	if got.PurchasedOn.Format(time.DateOnly) != "2020-08-20" {
		t.Fatalf("PurchasedOn = %v", got.PurchasedOn)
	}
}

func TestNewRejectsInvalidMoneyDateAndForgedListingSource(t *testing.T) {
	now := time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC)
	valid := Draft{
		ID: "11111111-1111-4111-8111-111111111111", UserID: "user-1", Name: "海河花园 两室",
		Property:                 PropertySnapshot{NeighborhoodID: "22222222-2222-4222-8222-222222222222", NeighborhoodName: "海河花园", City: "天津", District: "河西区", Layout: "2室1厅", AreaSQM: 82.5},
		OriginalPurchasePriceWan: 180, PurchasedOn: time.Date(2020, 8, 20, 0, 0, 0, 0, time.UTC), SourceKind: SourceManual,
	}
	tests := []struct {
		name string
		edit func(*Draft)
	}{
		{name: "zero purchase price", edit: func(d *Draft) { d.OriginalPurchasePriceWan = 0 }},
		{name: "future date", edit: func(d *Draft) { d.PurchasedOn = now.AddDate(0, 0, 1) }},
		{name: "listing without authoritative source", edit: func(d *Draft) { d.SourceKind = SourceMarketListing }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			draft := valid
			test.edit(&draft)
			if _, err := New(draft, now); err != ErrInvalidAsset {
				t.Fatalf("New() error = %v, want ErrInvalidAsset", err)
			}
		})
	}
}

func TestHoldingYearsUsesCompletedAnniversaries(t *testing.T) {
	asset := Asset{PurchasedOn: time.Date(2020, 8, 20, 0, 0, 0, 0, time.UTC)}
	if got := asset.HoldingYears(time.Date(2026, 7, 17, 0, 0, 0, 0, time.UTC)); got != 5 {
		t.Fatalf("HoldingYears() = %d, want 5", got)
	}
	if got := asset.HoldingYears(time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)); got != 6 {
		t.Fatalf("HoldingYears() = %d, want 6", got)
	}
}
