DROP INDEX IF EXISTS idx_transaction_observations_neighborhood_date;
DROP INDEX IF EXISTS idx_listing_observations_source_history;
DROP INDEX IF EXISTS idx_collection_runs_neighborhood_collected_at;

ALTER TABLE neighborhood_metrics
  DROP CONSTRAINT IF EXISTS neighborhood_metrics_collection_run_unique,
  DROP CONSTRAINT IF EXISTS neighborhood_metrics_inventory_run_fk,
  DROP CONSTRAINT IF EXISTS neighborhood_metrics_collection_run_fk,
  DROP CONSTRAINT IF EXISTS neighborhood_metrics_coverage_check,
  DROP CONSTRAINT IF EXISTS neighborhood_metrics_freshness_check,
  DROP CONSTRAINT IF EXISTS neighborhood_metrics_quality_state_check;

TRUNCATE TABLE neighborhood_metrics;

ALTER TABLE neighborhood_metrics
  DROP COLUMN IF EXISTS collection_run_id,
  DROP COLUMN IF EXISTS inventory_collection_run_id,
  DROP COLUMN IF EXISTS source_ids,
  DROP COLUMN IF EXISTS listing_sample_count,
  DROP COLUMN IF EXISTS transaction_sample_count,
  DROP COLUMN IF EXISTS listed_homes_change_pct,
  DROP COLUMN IF EXISTS coverage,
  DROP COLUMN IF EXISTS freshness,
  DROP COLUMN IF EXISTS quality_state,
  DROP COLUMN IF EXISTS latest_observed_at,
  DROP COLUMN IF EXISTS inventory_collected_at,
  DROP COLUMN IF EXISTS quality_warnings;

ALTER TABLE neighborhood_metrics
  ALTER COLUMN avg_days_on_market SET NOT NULL,
  ALTER COLUMN listing_price_min SET NOT NULL,
  ALTER COLUMN listing_price_max SET NOT NULL,
  ALTER COLUMN transaction_price_min SET NOT NULL,
  ALTER COLUMN transaction_price_max SET NOT NULL;

DROP TABLE IF EXISTS transaction_observations;
DROP TABLE IF EXISTS listing_observations;
DROP TABLE IF EXISTS collection_runs;
DROP TABLE IF EXISTS data_sources;

CREATE TABLE IF NOT EXISTS raw_collection_records (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  source_type TEXT NOT NULL,
  source_ref TEXT NOT NULL,
  payload JSONB NOT NULL,
  collected_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS listing_snapshots (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  collection_run_id UUID REFERENCES raw_collection_records(id) ON DELETE CASCADE,
  neighborhood_id UUID NOT NULL REFERENCES neighborhoods(id) ON DELETE CASCADE,
  listing_price NUMERIC(12,2) NOT NULL,
  transaction_price NUMERIC(12,2),
  price_cut BOOLEAN NOT NULL DEFAULT false,
  days_on_market INT NOT NULL DEFAULT 0,
  layout TEXT NOT NULL DEFAULT '',
  captured_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_listing_snapshots_neighborhood_captured_at
  ON listing_snapshots(neighborhood_id, captured_at DESC);
CREATE INDEX IF NOT EXISTS idx_listing_snapshots_neighborhood_run_captured_at
  ON listing_snapshots(neighborhood_id, collection_run_id, captured_at DESC);
