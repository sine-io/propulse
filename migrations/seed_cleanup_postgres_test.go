package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestLegacyDemoCleanupPreservesTrustedDataAndReviewNotes(t *testing.T) {
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

	schema := "seed_cleanup_" + uuid.NewString()
	schema = sanitizeTestSchema(schema)
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
	execEmbeddedMigration(t, ctx, db, "000004_review_notes.up.sql")

	legacyQingfengID := uuid.NewString()
	legacyYunlanID := uuid.NewString()
	trustedQingfengID := uuid.NewString()
	otherUserYunlanID := uuid.NewString()
	nearMatchID := uuid.NewString()

	insertNeighborhood := func(id, name, area, layout string) {
		t.Helper()
		if _, err := db.ExecContext(ctx,
			"INSERT INTO neighborhoods (id, name, area, target_layout) VALUES ($1, $2, $3, $4)",
			id, name, area, layout,
		); err != nil {
			t.Fatalf("insert neighborhood %q error = %v", name, err)
		}
	}
	insertWatchlist := func(userID, neighborhoodID string) {
		t.Helper()
		if _, err := db.ExecContext(ctx,
			"INSERT INTO watchlist_items (user_id, neighborhood_id) VALUES ($1, $2)",
			userID, neighborhoodID,
		); err != nil {
			t.Fatalf("insert watchlist item error = %v", err)
		}
	}

	insertNeighborhood(legacyQingfengID, "青枫花园", "滨江核心", "三房")
	insertNeighborhood(legacyYunlanID, "云澜府", "城东新区", "四房")
	insertNeighborhood(trustedQingfengID, "青枫花园", "滨江核心", "三房")
	insertNeighborhood(otherUserYunlanID, "云澜府", "城东新区", "四房")
	insertNeighborhood(nearMatchID, "青枫花园", "滨江核心", "四房")

	insertWatchlist("propulse-user", legacyQingfengID)
	insertWatchlist("demo-user", legacyYunlanID)
	insertWatchlist("propulse-user", trustedQingfengID)
	insertWatchlist("another-user", otherUserYunlanID)
	insertWatchlist("propulse-user", nearMatchID)

	noteID := uuid.NewString()
	if _, err := db.ExecContext(ctx,
		"INSERT INTO review_notes (id, user_id, neighborhood_id, kind, content) VALUES ($1, $2, $3, $4, $5)",
		noteID, "propulse-user", legacyQingfengID, "viewing_note", "应在小区清理后保留",
	); err != nil {
		t.Fatalf("insert review note error = %v", err)
	}

	dataSourceID := uuid.NewString()
	if _, err := db.ExecContext(ctx,
		"INSERT INTO data_sources (id, name, source_type, city) VALUES ($1, $2, $3, $4)",
		dataSourceID, "可信人工导入", "manual", "测试城市",
	); err != nil {
		t.Fatalf("insert data source error = %v", err)
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO collection_runs (
			id, data_source_id, neighborhood_id, source_ref, collected_at,
			coverage, import_format, content_checksum, raw_payload,
			raw_content_type, validation_summary
		) VALUES ($1, $2, $3, $4, now(), $5, $6, $7, $8, $9, $10)`,
		uuid.NewString(), dataSourceID, trustedQingfengID, "trusted-run-1",
		"full", "json", fmt.Sprintf("%064d", 1), []byte(`{}`),
		"application/json", `{}`,
	); err != nil {
		t.Fatalf("insert collection run error = %v", err)
	}

	execEmbeddedMigration(t, ctx, db, "000005_remove_legacy_demo_neighborhoods.up.sql")

	assertNeighborhoodExists(t, ctx, db, legacyQingfengID, false)
	assertNeighborhoodExists(t, ctx, db, legacyYunlanID, false)
	assertNeighborhoodExists(t, ctx, db, trustedQingfengID, true)
	assertNeighborhoodExists(t, ctx, db, otherUserYunlanID, true)
	assertNeighborhoodExists(t, ctx, db, nearMatchID, true)

	var legacyWatchlistCount int
	if err := db.QueryRowContext(ctx,
		"SELECT count(*) FROM watchlist_items WHERE neighborhood_id IN ($1, $2)",
		legacyQingfengID, legacyYunlanID,
	).Scan(&legacyWatchlistCount); err != nil {
		t.Fatalf("count legacy watchlist items error = %v", err)
	}
	if legacyWatchlistCount != 0 {
		t.Fatalf("legacy watchlist item count = %d, want 0", legacyWatchlistCount)
	}

	var (
		noteNeighborhoodIsNull bool
		noteContent            string
	)
	if err := db.QueryRowContext(ctx,
		"SELECT neighborhood_id IS NULL, content FROM review_notes WHERE id = $1",
		noteID,
	).Scan(&noteNeighborhoodIsNull, &noteContent); err != nil {
		t.Fatalf("load preserved review note error = %v", err)
	}
	if !noteNeighborhoodIsNull || noteContent != "应在小区清理后保留" {
		t.Fatalf("review note = {neighborhood null: %v, content: %q}, want preserved with null neighborhood", noteNeighborhoodIsNull, noteContent)
	}
}

func execEmbeddedMigration(t *testing.T, ctx context.Context, db *sql.DB, name string) {
	t.Helper()
	body, err := FS.ReadFile(name)
	if err != nil {
		t.Fatalf("read migration %s error = %v", name, err)
	}
	if _, err := db.ExecContext(ctx, string(body)); err != nil {
		t.Fatalf("execute migration %s error = %v", name, err)
	}
}

func assertNeighborhoodExists(t *testing.T, ctx context.Context, db *sql.DB, id string, want bool) {
	t.Helper()
	var exists bool
	if err := db.QueryRowContext(ctx,
		"SELECT EXISTS (SELECT 1 FROM neighborhoods WHERE id = $1)", id,
	).Scan(&exists); err != nil {
		t.Fatalf("check neighborhood %s error = %v", id, err)
	}
	if exists != want {
		t.Fatalf("neighborhood %s exists = %v, want %v", id, exists, want)
	}
}

func sanitizeTestSchema(value string) string {
	result := make([]byte, 0, len(value))
	for i := 0; i < len(value); i++ {
		if value[i] == '-' {
			result = append(result, '_')
			continue
		}
		result = append(result, value[i])
	}
	return string(result)
}
