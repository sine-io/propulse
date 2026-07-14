package migrations

import (
	"io/fs"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func TestEmbeddedMigrationSetIsCompleteAndOrdered(t *testing.T) {
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
		"000002_listing_snapshots_collection_run.down.sql",
		"000002_listing_snapshots_collection_run.up.sql",
		"000003_trusted_market_data.down.sql",
		"000003_trusted_market_data.up.sql",
		"000004_review_notes.down.sql",
		"000004_review_notes.up.sql",
		"000005_remove_legacy_demo_neighborhoods.down.sql",
		"000005_remove_legacy_demo_neighborhoods.up.sql",
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
	for _, forbidden := range []string{"raw_collection_records", "listing_snapshots"} {
		if strings.Contains(string(body), forbidden) {
			t.Fatalf("initial schema still contains legacy table %q", forbidden)
		}
	}

	const stableUserDefault = "user_id TEXT NOT NULL DEFAULT 'propulse-user'"
	if count := strings.Count(string(body), stableUserDefault); count != 2 {
		t.Fatalf("initial schema has %d %q defaults, want 2", count, stableUserDefault)
	}
}

func TestLegacyDemoCleanupMigrationUsesStrictIdentificationAndSafetyGuards(t *testing.T) {
	body, err := fs.ReadFile(FS, "000005_remove_legacy_demo_neighborhoods.up.sql")
	if err != nil {
		t.Fatalf("ReadFile(version 5) error = %v", err)
	}

	for _, required := range []string{
		"n.name = '青枫花园'",
		"n.area = '滨江核心'",
		"n.target_layout = '三房'",
		"n.name = '云澜府'",
		"n.area = '城东新区'",
		"n.target_layout = '四房'",
		"w.user_id = 'propulse-user'",
		"w.user_id = 'demo-user'",
		"NOT EXISTS",
		"collection_runs",
	} {
		if !strings.Contains(string(body), required) {
			t.Fatalf("legacy demo cleanup migration is missing %q", required)
		}
	}
}

func TestUpgradeMigrationsPreservePublishedVersionAndContractLegacyMarketData(t *testing.T) {
	versionTwo, err := fs.ReadFile(FS, "000002_listing_snapshots_collection_run.up.sql")
	if err != nil {
		t.Fatalf("ReadFile(version 2) error = %v", err)
	}
	for _, required := range []string{
		"to_regclass('public.listing_snapshots')",
		"ADD COLUMN IF NOT EXISTS collection_run_id",
	} {
		if !strings.Contains(string(versionTwo), required) {
			t.Fatalf("compatibility migration 2 is missing %q", required)
		}
	}

	versionThree, err := fs.ReadFile(FS, "000003_trusted_market_data.up.sql")
	if err != nil {
		t.Fatalf("ReadFile(version 3) error = %v", err)
	}
	for _, required := range []string{
		"CREATE TABLE IF NOT EXISTS data_sources",
		"DELETE FROM neighborhood_metrics",
		"ALTER COLUMN collection_run_id SET NOT NULL",
		"DROP TABLE IF EXISTS listing_snapshots",
		"DROP TABLE IF EXISTS raw_collection_records",
	} {
		if !strings.Contains(string(versionThree), required) {
			t.Fatalf("trusted-data migration 3 is missing %q", required)
		}
	}
}
