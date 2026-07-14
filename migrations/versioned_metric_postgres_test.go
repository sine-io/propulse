package migrations

import (
	"context"
	"database/sql"
	"os"
	"testing"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestVersionedMetricMigrationPreservesLegacyRowsAndAllowsNewVersions(t *testing.T) {
	databaseURL := os.Getenv("PROPULSE_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("PROPULSE_TEST_DATABASE_URL is not set")
	}

	ctx := context.Background()
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.ExecContext(ctx, "CREATE EXTENSION IF NOT EXISTS pgcrypto WITH SCHEMA public"); err != nil {
		t.Fatalf("ensure pgcrypto extension error = %v", err)
	}

	schema := sanitizeTestSchema("metric_version_" + uuid.NewString())
	if _, err := db.ExecContext(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatalf("create test schema error = %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
	})
	if _, err := db.ExecContext(ctx, "SET search_path TO "+schema+", public"); err != nil {
		t.Fatalf("set search_path error = %v", err)
	}

	execEmbeddedMigration(t, ctx, db, "000001_initial_schema.up.sql")
	neighborhoodID := uuid.NewString()
	sourceID := uuid.NewString()
	runID := uuid.NewString()
	legacyMetricID := uuid.NewString()
	if _, err := db.ExecContext(ctx, "INSERT INTO neighborhoods (id, name) VALUES ($1, 'legacy metric neighborhood')", neighborhoodID); err != nil {
		t.Fatalf("insert neighborhood: %v", err)
	}
	if _, err := db.ExecContext(ctx, "INSERT INTO data_sources (id, name, source_type, city) VALUES ($1, 'legacy metric source', 'manual', '测试市')", sourceID); err != nil {
		t.Fatalf("insert source: %v", err)
	}
	if _, err := db.ExecContext(ctx, `
INSERT INTO collection_runs (
  id, data_source_id, neighborhood_id, source_ref, collected_at, coverage,
  import_format, content_checksum, raw_payload, raw_content_type, validation_summary
) VALUES ($1, $2, $3, 'legacy-run', '2026-07-14T00:00:00Z', 'full', 'json',
  repeat('a', 64), '{}'::bytea, 'application/json', '{}'::jsonb)`, runID, sourceID, neighborhoodID); err != nil {
		t.Fatalf("insert collection run: %v", err)
	}
	if _, err := db.ExecContext(ctx, `
INSERT INTO neighborhood_metrics (
  id, neighborhood_id, listed_homes, price_cut_homes, transaction_momentum,
  target_layout_supply, collection_run_id, source_ids, listing_sample_count,
  transaction_sample_count, coverage, freshness, quality_state, latest_observed_at
) VALUES ($1, $2, 6, 1, 'stable', 2, $3, '[]'::jsonb, 6, 3,
  'full', 'current', 'sufficient', '2026-07-14T00:00:00Z')`, legacyMetricID, neighborhoodID, runID); err != nil {
		t.Fatalf("insert legacy metric: %v", err)
	}

	execEmbeddedMigration(t, ctx, db, "000006_versioned_metric_evidence.up.sql")
	var algorithmVersion string
	var windowStart, recentCount, recentFrequency any
	if err := db.QueryRowContext(ctx, `
SELECT algorithm_version, transaction_window_start, recent_30_day_transaction_count,
       recent_30_day_monthly_frequency
FROM neighborhood_metrics WHERE id = $1`, legacyMetricID).Scan(
		&algorithmVersion, &windowStart, &recentCount, &recentFrequency,
	); err != nil {
		t.Fatalf("read migrated legacy metric: %v", err)
	}
	if algorithmVersion != "legacy_unversioned" || windowStart != nil || recentCount != nil || recentFrequency != nil {
		t.Fatalf("legacy metric was inferred: version=%q window=%#v count=%#v frequency=%#v", algorithmVersion, windowStart, recentCount, recentFrequency)
	}

	const insertVersionedMetric = `
INSERT INTO neighborhood_metrics (
  neighborhood_id, listed_homes, price_cut_homes, transaction_momentum,
  target_layout_supply, collection_run_id, source_ids, listing_sample_count,
  transaction_sample_count, coverage, freshness, quality_state, latest_observed_at,
  algorithm_version, transaction_window_start, transaction_window_end,
  recent_30_day_transaction_count, preceding_60_day_transaction_count,
  recent_30_day_monthly_frequency, preceding_60_day_monthly_frequency
) VALUES ($1, 6, 1, 'stable', 2, $2, '[]'::jsonb, 6, 3, 'full', 'current',
  'sufficient', '2026-07-14T00:00:00Z', 'market-metrics/test.1',
  '2026-04-15', '2026-07-14', 1, 2, 1, 1)`
	if _, err := db.ExecContext(ctx, insertVersionedMetric, neighborhoodID, runID); err != nil {
		t.Fatalf("insert versioned metric beside legacy row: %v", err)
	}
	if _, err := db.ExecContext(ctx, insertVersionedMetric, neighborhoodID, runID); err == nil {
		t.Fatal("duplicate run/algorithm metric insert succeeded")
	}

	var count int
	if err := db.QueryRowContext(ctx, "SELECT count(*) FROM neighborhood_metrics WHERE collection_run_id = $1", runID).Scan(&count); err != nil {
		t.Fatalf("count metric versions: %v", err)
	}
	if count != 2 {
		t.Fatalf("metric version count = %d, want legacy plus current", count)
	}
}
