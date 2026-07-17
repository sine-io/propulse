package migrations_test

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	migraterunner "github.com/sine-io/propulse/internal/infrastructure/migrate"
)

func TestFangjianMarketMigrationUpAndDown(t *testing.T) {
	databaseURL := os.Getenv("PROPULSE_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("PROPULSE_TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	if err := migraterunner.Run(ctx, databaseURL, "up"); err != nil {
		t.Fatalf("migrate up error = %v", err)
	}
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("pgxpool.New() error = %v", err)
	}
	defer pool.Close()
	var adjustmentTable *string
	if err := pool.QueryRow(ctx, `SELECT to_regclass('public.listing_adjustments')::text`).Scan(&adjustmentTable); err != nil || adjustmentTable == nil || *adjustmentTable != "listing_adjustments" {
		t.Fatalf("listing_adjustments after up = %#v, %v", adjustmentTable, err)
	}
	var columns int
	if err := pool.QueryRow(ctx, `
		SELECT count(*) FROM information_schema.columns
		WHERE table_name = 'community_market_snapshots'
		  AND column_name IN ('collection_run_id', 'analysis', 'surroundings', 'city_context', 'quality_status')
	`).Scan(&columns); err != nil || columns != 5 {
		t.Fatalf("Fangjian snapshot columns after up = %d, %v", columns, err)
	}

	if err := migraterunner.Run(ctx, databaseURL, "down"); err != nil {
		t.Fatalf("migrate down error = %v", err)
	}
	adjustmentTable = nil
	if err := pool.QueryRow(ctx, `SELECT to_regclass('public.listing_adjustments')::text`).Scan(&adjustmentTable); err != nil || adjustmentTable != nil {
		t.Fatalf("listing_adjustments after down = %#v, %v", adjustmentTable, err)
	}
}
