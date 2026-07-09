package collection

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestImportManualListingsStoresRawRunAndSnapshots(t *testing.T) {
	now := time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC)
	ids := []string{"collection_run_1", "snapshot_1", "snapshot_2"}
	transactionPrice := 495.0
	repo := &fakeRepository{neighborhoods: map[string]bool{"neighborhood_1": true}}
	service := NewService(repo, func() time.Time { return now }, func() string {
		id := ids[0]
		ids = ids[1:]
		return id
	})

	result, err := service.ImportManualListings(context.Background(), ImportManualListingsCommand{
		SourceType:     "manual_json",
		SourceRef:      "demo-weekly-import",
		NeighborhoodID: "neighborhood_1",
		Records: []ManualListingRecord{
			{ListingPrice: 520, TransactionPrice: &transactionPrice, PriceCut: true, DaysOnMarket: 78, Layout: "三房"},
			{ListingPrice: 610, PriceCut: false, DaysOnMarket: 14, Layout: "三房"},
		},
	})
	if err != nil {
		t.Fatalf("ImportManualListings() error = %v", err)
	}

	if result.CollectionRunID != "collection_run_1" {
		t.Fatalf("CollectionRunID = %q, want collection_run_1", result.CollectionRunID)
	}
	if result.ImportedSnapshotCount != 2 {
		t.Fatalf("ImportedSnapshotCount = %d, want 2", result.ImportedSnapshotCount)
	}
	if len(repo.rawRecords) != 1 {
		t.Fatalf("raw records = %d, want 1", len(repo.rawRecords))
	}
	raw := repo.rawRecords[0]
	if raw.ID != "collection_run_1" || raw.SourceType != "manual_json" || raw.SourceRef != "demo-weekly-import" {
		t.Fatalf("raw record = %#v", raw)
	}
	if len(raw.Payload) == 0 {
		t.Fatal("raw payload is empty")
	}
	if len(repo.snapshots) != 2 {
		t.Fatalf("snapshots = %d, want 2", len(repo.snapshots))
	}
	first := repo.snapshots[0]
	if first.ID != "snapshot_1" || first.NeighborhoodID != "neighborhood_1" || first.ListingPrice != 520 || first.TransactionPrice == nil || *first.TransactionPrice != 495 || !first.PriceCut || first.DaysOnMarket != 78 || first.Layout != "三房" {
		t.Fatalf("first snapshot = %#v", first)
	}
	if !first.CapturedAt.Equal(now) {
		t.Fatalf("CapturedAt = %v, want %v", first.CapturedAt, now)
	}
}

func TestImportManualListingsPreservesOmittedTransactionPrice(t *testing.T) {
	repo := &fakeRepository{neighborhoods: map[string]bool{"neighborhood_1": true}}
	service := NewService(repo, time.Now, func() string { return "id" })

	_, err := service.ImportManualListings(context.Background(), ImportManualListingsCommand{
		SourceType:     "manual_json",
		SourceRef:      "demo-weekly-import",
		NeighborhoodID: "neighborhood_1",
		Records: []ManualListingRecord{
			{ListingPrice: 610, PriceCut: false, DaysOnMarket: 14, Layout: "三房"},
		},
	})
	if err != nil {
		t.Fatalf("ImportManualListings() error = %v", err)
	}
	if len(repo.snapshots) != 1 {
		t.Fatalf("snapshots = %d, want 1", len(repo.snapshots))
	}
	if repo.snapshots[0].TransactionPrice != nil {
		t.Fatalf("TransactionPrice = %v, want nil for omitted transactionPrice", *repo.snapshots[0].TransactionPrice)
	}
}

func TestImportManualListingsValidatesRequest(t *testing.T) {
	valid := ImportManualListingsCommand{
		SourceType:     "manual_json",
		SourceRef:      "demo-weekly-import",
		NeighborhoodID: "neighborhood_1",
		Records:        []ManualListingRecord{{ListingPrice: 520, DaysOnMarket: 0}},
	}

	tests := []struct {
		name    string
		mutate  func(*ImportManualListingsCommand)
		wantErr error
	}{
		{name: "source type", mutate: func(c *ImportManualListingsCommand) { c.SourceType = "crawler" }, wantErr: ErrInvalidRequest},
		{name: "missing source ref", mutate: func(c *ImportManualListingsCommand) { c.SourceRef = "" }, wantErr: ErrInvalidRequest},
		{name: "blank source ref", mutate: func(c *ImportManualListingsCommand) { c.SourceRef = " \t\n " }, wantErr: ErrInvalidRequest},
		{name: "neighborhood id", mutate: func(c *ImportManualListingsCommand) { c.NeighborhoodID = "" }, wantErr: ErrInvalidRequest},
		{name: "missing records", mutate: func(c *ImportManualListingsCommand) { c.Records = nil }, wantErr: ErrInvalidRequest},
		{name: "too many records", mutate: func(c *ImportManualListingsCommand) {
			c.Records = make([]ManualListingRecord, 501)
			for i := range c.Records {
				c.Records[i].ListingPrice = 1
			}
		}, wantErr: ErrInvalidRequest},
		{name: "listing price", mutate: func(c *ImportManualListingsCommand) { c.Records[0].ListingPrice = 0 }, wantErr: ErrInvalidRequest},
		{name: "days on market", mutate: func(c *ImportManualListingsCommand) { c.Records[0].DaysOnMarket = -1 }, wantErr: ErrInvalidRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			command := valid
			command.Records = append([]ManualListingRecord(nil), valid.Records...)
			tt.mutate(&command)

			repo := &fakeRepository{neighborhoods: map[string]bool{"neighborhood_1": true}}
			service := NewService(repo, time.Now, func() string { return "id" })
			_, err := service.ImportManualListings(context.Background(), command)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("error = %v, want %v", err, tt.wantErr)
			}
			if len(repo.rawRecords) != 0 || len(repo.snapshots) != 0 {
				t.Fatalf("repository was called for invalid request")
			}
		})
	}
}

func TestImportManualListingsTrimsSourceRefBeforeSaving(t *testing.T) {
	repo := &fakeRepository{neighborhoods: map[string]bool{"neighborhood_1": true}}
	service := NewService(repo, time.Now, func() string { return "id" })

	_, err := service.ImportManualListings(context.Background(), ImportManualListingsCommand{
		SourceType:     "manual_json",
		SourceRef:      "  demo-weekly-import  ",
		NeighborhoodID: "neighborhood_1",
		Records:        []ManualListingRecord{{ListingPrice: 520, DaysOnMarket: 0}},
	})
	if err != nil {
		t.Fatalf("ImportManualListings() error = %v", err)
	}
	if len(repo.rawRecords) != 1 {
		t.Fatalf("raw records = %d, want 1", len(repo.rawRecords))
	}
	if repo.rawRecords[0].SourceRef != "demo-weekly-import" {
		t.Fatalf("SourceRef = %q, want demo-weekly-import", repo.rawRecords[0].SourceRef)
	}
}

func TestImportManualListingsReturnsNeighborhoodNotFound(t *testing.T) {
	service := NewService(&fakeRepository{}, time.Now, func() string { return "id" })

	_, err := service.ImportManualListings(context.Background(), ImportManualListingsCommand{
		SourceType:     "manual_json",
		SourceRef:      "demo-weekly-import",
		NeighborhoodID: "missing",
		Records:        []ManualListingRecord{{ListingPrice: 520, DaysOnMarket: 0}},
	})
	if !errors.Is(err, ErrNeighborhoodNotFound) {
		t.Fatalf("error = %v, want %v", err, ErrNeighborhoodNotFound)
	}
}

func TestImportManualListingsWrapsSaveFailure(t *testing.T) {
	service := NewService(&fakeRepository{
		neighborhoods: map[string]bool{"neighborhood_1": true},
		saveErr:       errors.New("database unavailable"),
	}, time.Now, func() string { return "id" })

	_, err := service.ImportManualListings(context.Background(), ImportManualListingsCommand{
		SourceType:     "manual_json",
		SourceRef:      "demo-weekly-import",
		NeighborhoodID: "neighborhood_1",
		Records:        []ManualListingRecord{{ListingPrice: 520, DaysOnMarket: 0}},
	})
	if !errors.Is(err, ErrImportFailed) {
		t.Fatalf("error = %v, want %v", err, ErrImportFailed)
	}
}

type fakeRepository struct {
	neighborhoods map[string]bool
	rawRecords    []RawCollectionRecord
	snapshots     []ListingSnapshot
	saveErr       error
}

func (r *fakeRepository) NeighborhoodExists(_ context.Context, id string) (bool, error) {
	return r.neighborhoods[id], nil
}

func (r *fakeRepository) SaveImport(_ context.Context, raw RawCollectionRecord, snapshots []ListingSnapshot) error {
	if r.saveErr != nil {
		return r.saveErr
	}
	r.rawRecords = append(r.rawRecords, raw)
	r.snapshots = append(r.snapshots, snapshots...)
	return nil
}
