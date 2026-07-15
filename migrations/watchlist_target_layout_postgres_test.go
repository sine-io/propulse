package migrations

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestWatchlistTargetLayoutMigrationBackfillsOnlyTrustedCatalogData(t *testing.T) {
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

	schema := sanitizeTestSchema("watchlist_target_" + uuid.NewString())
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

	trustedNeighborhoodID := uuid.NewString()
	conflictingNeighborhoodID := uuid.NewString()
	unknownNeighborhoodID := uuid.NewString()
	insertLegacyNeighborhood(t, ctx, db, trustedNeighborhoodID, "可信花园", "滨江", "三房")
	insertLegacyNeighborhood(t, ctx, db, conflictingNeighborhoodID, "冲突花园", "城西", "四房")
	insertLegacyNeighborhood(t, ctx, db, unknownNeighborhoodID, "未知花园", "城北", "两房")

	trustedSourceID := insertMigrationSource(t, ctx, db, "可信来源", "杭州")
	conflictingSourceAID := insertMigrationSource(t, ctx, db, "冲突来源甲", "上海")
	conflictingSourceBID := insertMigrationSource(t, ctx, db, "冲突来源乙", "苏州")
	trustedRunID := insertMigrationRun(t, ctx, db, trustedSourceID, trustedNeighborhoodID, "trusted-run", "a")
	insertMigrationRun(t, ctx, db, conflictingSourceAID, conflictingNeighborhoodID, "conflict-a", "b")
	insertMigrationRun(t, ctx, db, conflictingSourceBID, conflictingNeighborhoodID, "conflict-b", "c")

	if _, err := db.ExecContext(ctx, `
INSERT INTO listing_observations (
  collection_run_id, neighborhood_id, source_listing_id, source_row, layout,
  area_sqm, listing_price, days_on_market, status, captured_at
) VALUES ($1, $2, 'listing-1', 1, '四房', 120, 600, 10, 'active', '2026-07-14T00:00:00Z')`, trustedRunID, trustedNeighborhoodID); err != nil {
		t.Fatalf("insert listing observation: %v", err)
	}
	if _, err := db.ExecContext(ctx, `
INSERT INTO transaction_observations (
  collection_run_id, neighborhood_id, source_record_id, source_row, layout,
  area_sqm, transaction_price, transaction_date, captured_at
) VALUES ($1, $2, 'transaction-1', 2, '五房', 150, 720, '2026-07-01', '2026-07-14T00:00:00Z')`, trustedRunID, trustedNeighborhoodID); err != nil {
		t.Fatalf("insert transaction observation: %v", err)
	}
	if _, err := db.ExecContext(ctx,
		"INSERT INTO watchlist_items (user_id, neighborhood_id) VALUES ('migration-user', $1)",
		trustedNeighborhoodID,
	); err != nil {
		t.Fatalf("insert watchlist item: %v", err)
	}
	metricID := uuid.NewString()
	if _, err := db.ExecContext(ctx, `
INSERT INTO neighborhood_metrics (
  id, neighborhood_id, listed_homes, price_cut_homes, transaction_momentum,
  target_layout_supply, collection_run_id, source_ids, listing_sample_count,
  transaction_sample_count, coverage, freshness, quality_state, latest_observed_at
) VALUES ($1, $2, 9, 1, 'stable', 7, $3, '[]'::jsonb, 9, 3,
  'full', 'current', 'sufficient', '2026-07-14T00:00:00Z')`, metricID, trustedNeighborhoodID, trustedRunID); err != nil {
		t.Fatalf("insert legacy metric: %v", err)
	}

	execEmbeddedMigration(t, ctx, db, "000007_watchlist_target_layout.up.sql")

	assertMigratedCity(t, ctx, db, trustedNeighborhoodID, "杭州", true)
	assertMigratedCity(t, ctx, db, conflictingNeighborhoodID, "", false)
	assertMigratedCity(t, ctx, db, unknownNeighborhoodID, "", false)

	rows, err := db.QueryContext(ctx,
		"SELECT layout FROM neighborhood_layouts WHERE neighborhood_id = $1 ORDER BY layout", trustedNeighborhoodID)
	if err != nil {
		t.Fatalf("query layout catalog: %v", err)
	}
	defer rows.Close()
	var layouts []string
	for rows.Next() {
		var layout string
		if err := rows.Scan(&layout); err != nil {
			t.Fatalf("scan layout: %v", err)
		}
		layouts = append(layouts, layout)
	}
	if !reflect.DeepEqual(layouts, []string{"三房", "五房", "四房"}) {
		t.Fatalf("layout catalog = %#v", layouts)
	}

	var targetLayout string
	if err := db.QueryRowContext(ctx,
		"SELECT target_layout FROM watchlist_items WHERE user_id = 'migration-user' AND neighborhood_id = $1",
		trustedNeighborhoodID,
	).Scan(&targetLayout); err != nil {
		t.Fatalf("read migrated watchlist target: %v", err)
	}
	if targetLayout != "三房" {
		t.Fatalf("watchlist target = %q, want 三房", targetLayout)
	}

	var rawSupply []byte
	if err := db.QueryRowContext(ctx,
		"SELECT target_layout_supply_by_layout FROM neighborhood_metrics WHERE id = $1", metricID,
	).Scan(&rawSupply); err != nil {
		t.Fatalf("read migrated layout supply: %v", err)
	}
	var supply map[string]int
	if err := json.Unmarshal(rawSupply, &supply); err != nil {
		t.Fatalf("decode migrated layout supply: %v", err)
	}
	if !reflect.DeepEqual(supply, map[string]int{"三房": 7}) {
		t.Fatalf("layout supply = %#v", supply)
	}

	if _, err := db.ExecContext(ctx,
		"INSERT INTO watchlist_items (user_id, neighborhood_id, target_layout) VALUES ('migration-user', $1, '四房')",
		trustedNeighborhoodID,
	); err == nil {
		t.Fatal("duplicate user/neighborhood watchlist insert succeeded")
	}
	if _, err := db.ExecContext(ctx,
		"INSERT INTO watchlist_items (user_id, neighborhood_id, target_layout) VALUES ('other-user', $1, '不存在户型')",
		trustedNeighborhoodID,
	); err == nil {
		t.Fatal("watchlist insert outside the layout catalog succeeded")
	}
}

func insertLegacyNeighborhood(t *testing.T, ctx context.Context, db *sql.DB, id, name, area, layout string) {
	t.Helper()
	if _, err := db.ExecContext(ctx,
		"INSERT INTO neighborhoods (id, name, area, target_layout) VALUES ($1, $2, $3, $4)",
		id, name, area, layout,
	); err != nil {
		t.Fatalf("insert neighborhood %q: %v", name, err)
	}
}

func insertMigrationSource(t *testing.T, ctx context.Context, db *sql.DB, name, city string) string {
	t.Helper()
	id := uuid.NewString()
	if _, err := db.ExecContext(ctx,
		"INSERT INTO data_sources (id, name, source_type, city) VALUES ($1, $2, 'manual', $3)",
		id, name, city,
	); err != nil {
		t.Fatalf("insert data source %q: %v", name, err)
	}
	return id
}

func insertMigrationRun(t *testing.T, ctx context.Context, db *sql.DB, sourceID, neighborhoodID, sourceRef, checksumCharacter string) string {
	t.Helper()
	id := uuid.NewString()
	if _, err := db.ExecContext(ctx, `
INSERT INTO collection_runs (
  id, data_source_id, neighborhood_id, source_ref, collected_at, coverage,
  import_format, content_checksum, raw_payload, raw_content_type, validation_summary
) VALUES ($1, $2, $3, $4, '2026-07-14T00:00:00Z', 'full', 'json',
  repeat($5, 64), '{}'::bytea, 'application/json', '{}'::jsonb)`, id, sourceID, neighborhoodID, sourceRef, checksumCharacter); err != nil {
		t.Fatalf("insert collection run %q: %v", sourceRef, err)
	}
	return id
}

func assertMigratedCity(t *testing.T, ctx context.Context, db *sql.DB, neighborhoodID, want string, wantValid bool) {
	t.Helper()
	var city sql.NullString
	if err := db.QueryRowContext(ctx, "SELECT city FROM neighborhoods WHERE id = $1", neighborhoodID).Scan(&city); err != nil {
		t.Fatalf("read migrated city: %v", err)
	}
	if city.Valid != wantValid || city.String != want {
		t.Fatalf("city = %#v, want value %q valid %v", city, want, wantValid)
	}
}
