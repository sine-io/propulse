package migrations

import (
	"io/fs"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func TestEmbeddedMigrationSetIsSingleCoherentInitialSchema(t *testing.T) {
	entries, err := fs.ReadDir(FS, ".")
	if err != nil {
		t.Fatalf("ReadDir() error = %v", err)
	}

	var names []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".sql") {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	want := []string{
		"000001_initial_schema.down.sql",
		"000001_initial_schema.up.sql",
	}
	if !reflect.DeepEqual(names, want) {
		t.Fatalf("embedded migrations = %#v, want %#v", names, want)
	}

	body, err := fs.ReadFile(FS, "000001_initial_schema.up.sql")
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	for _, required := range []string{
		"collection_run_id UUID NOT NULL",
		"idx_listing_snapshots_neighborhood_run_captured_at",
		"CREATE TABLE data_sources",
		"CREATE TABLE collection_runs",
		"CREATE TABLE listing_observations",
		"CREATE TABLE transaction_observations",
		"UNIQUE (data_source_id, source_ref, content_checksum)",
		"UNIQUE (collection_run_id, source_listing_id)",
		"UNIQUE (collection_run_id, source_record_id)",
		"metric_status TEXT NOT NULL",
		"inventory_collection_run_id UUID",
		"FOREIGN KEY (collection_run_id, neighborhood_id)",
		"ON DELETE SET NULL (inventory_collection_run_id)",
		"avg_days_on_market NUMERIC(8,2),",
		"listing_price_min NUMERIC(12,2),",
		"listing_price_max NUMERIC(12,2),",
		"transaction_price_min NUMERIC(12,2),",
		"transaction_price_max NUMERIC(12,2),",
		"idx_collection_runs_neighborhood_collected_at",
		"idx_listing_observations_source_history",
		"idx_transaction_observations_neighborhood_date",
	} {
		if !strings.Contains(string(body), required) {
			t.Fatalf("expanded initial schema is missing %q", required)
		}
	}

	const stableUserDefault = "user_id TEXT NOT NULL DEFAULT 'propulse-user'"
	if count := strings.Count(string(body), stableUserDefault); count != 2 {
		t.Fatalf("initial schema has %d %q defaults, want 2", count, stableUserDefault)
	}
}
