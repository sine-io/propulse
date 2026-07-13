CREATE TABLE IF NOT EXISTS data_sources (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name TEXT NOT NULL CHECK (char_length(name) BETWEEN 1 AND 128),
  source_type TEXT NOT NULL CHECK (source_type ~ '^[a-z][a-z0-9_]{0,63}$'),
  city TEXT NOT NULL CHECK (char_length(city) BETWEEN 1 AND 128),
  notes TEXT NOT NULL DEFAULT '' CHECK (char_length(notes) <= 2048),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (name, city)
);

CREATE TABLE IF NOT EXISTS collection_runs (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  data_source_id UUID NOT NULL REFERENCES data_sources(id) ON DELETE RESTRICT,
  neighborhood_id UUID NOT NULL REFERENCES neighborhoods(id) ON DELETE CASCADE,
  source_ref TEXT NOT NULL CHECK (char_length(source_ref) BETWEEN 1 AND 256),
  collected_at TIMESTAMPTZ NOT NULL,
  coverage TEXT NOT NULL CHECK (coverage IN ('full', 'partial')),
  import_format TEXT NOT NULL CHECK (import_format IN ('json', 'csv')),
  content_checksum TEXT NOT NULL CHECK (content_checksum ~ '^[0-9a-f]{64}$'),
  raw_payload BYTEA NOT NULL,
  raw_content_type TEXT NOT NULL,
  validation_summary JSONB NOT NULL,
  status TEXT NOT NULL DEFAULT 'completed' CHECK (status = 'completed'),
  metric_status TEXT NOT NULL DEFAULT 'pending' CHECK (metric_status IN ('pending', 'completed', 'failed')),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (data_source_id, source_ref, content_checksum),
  UNIQUE (id, neighborhood_id)
);

CREATE TABLE IF NOT EXISTS listing_observations (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  collection_run_id UUID NOT NULL,
  neighborhood_id UUID NOT NULL,
  source_listing_id TEXT NOT NULL CHECK (char_length(source_listing_id) BETWEEN 1 AND 128),
  source_row INT NOT NULL CHECK (source_row >= 1),
  layout TEXT NOT NULL CHECK (char_length(layout) BETWEEN 1 AND 64),
  area_sqm NUMERIC(8,2) NOT NULL CHECK (area_sqm > 0 AND area_sqm <= 10000),
  listing_price NUMERIC(12,2) NOT NULL CHECK (listing_price > 0),
  days_on_market INT NOT NULL CHECK (days_on_market BETWEEN 0 AND 36500),
  status TEXT NOT NULL CHECK (status IN ('active', 'pending', 'withdrawn', 'sold')),
  captured_at TIMESTAMPTZ NOT NULL,
  attributes JSONB NOT NULL DEFAULT '{}'::jsonb,
  FOREIGN KEY (collection_run_id, neighborhood_id) REFERENCES collection_runs(id, neighborhood_id) ON DELETE CASCADE,
  UNIQUE (collection_run_id, source_listing_id)
);

CREATE TABLE IF NOT EXISTS transaction_observations (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  collection_run_id UUID NOT NULL,
  neighborhood_id UUID NOT NULL,
  source_record_id TEXT NOT NULL CHECK (char_length(source_record_id) BETWEEN 1 AND 128),
  source_row INT NOT NULL CHECK (source_row >= 1),
  layout TEXT NOT NULL CHECK (char_length(layout) BETWEEN 1 AND 64),
  area_sqm NUMERIC(8,2) NOT NULL CHECK (area_sqm > 0 AND area_sqm <= 10000),
  transaction_price NUMERIC(12,2) NOT NULL CHECK (transaction_price > 0),
  transaction_date DATE NOT NULL,
  original_listing_ref TEXT,
  captured_at TIMESTAMPTZ NOT NULL,
  FOREIGN KEY (collection_run_id, neighborhood_id) REFERENCES collection_runs(id, neighborhood_id) ON DELETE CASCADE,
  UNIQUE (collection_run_id, source_record_id)
);

ALTER TABLE neighborhood_metrics
  ADD COLUMN IF NOT EXISTS collection_run_id UUID,
  ADD COLUMN IF NOT EXISTS inventory_collection_run_id UUID,
  ADD COLUMN IF NOT EXISTS source_ids JSONB NOT NULL DEFAULT '[]'::jsonb,
  ADD COLUMN IF NOT EXISTS listing_sample_count INT NOT NULL DEFAULT 0 CHECK (listing_sample_count >= 0),
  ADD COLUMN IF NOT EXISTS transaction_sample_count INT NOT NULL DEFAULT 0 CHECK (transaction_sample_count >= 0),
  ADD COLUMN IF NOT EXISTS listed_homes_change_pct NUMERIC(8,2),
  ADD COLUMN IF NOT EXISTS coverage TEXT,
  ADD COLUMN IF NOT EXISTS freshness TEXT,
  ADD COLUMN IF NOT EXISTS quality_state TEXT,
  ADD COLUMN IF NOT EXISTS latest_observed_at TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS inventory_collected_at TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS quality_warnings JSONB NOT NULL DEFAULT '[]'::jsonb;

-- Rows without complete provenance cannot be represented as trusted metrics.
-- Already compliant rows from an intermediate deployment remain intact.
DELETE FROM neighborhood_metrics
WHERE collection_run_id IS NULL
   OR coverage IS NULL
   OR freshness IS NULL
   OR quality_state IS NULL
   OR latest_observed_at IS NULL;

ALTER TABLE neighborhood_metrics
  ALTER COLUMN avg_days_on_market DROP NOT NULL,
  ALTER COLUMN listing_price_min DROP NOT NULL,
  ALTER COLUMN listing_price_max DROP NOT NULL,
  ALTER COLUMN transaction_price_min DROP NOT NULL,
  ALTER COLUMN transaction_price_max DROP NOT NULL,
  ALTER COLUMN collection_run_id SET NOT NULL,
  ALTER COLUMN coverage SET NOT NULL,
  ALTER COLUMN freshness SET NOT NULL,
  ALTER COLUMN quality_state SET NOT NULL,
  ALTER COLUMN latest_observed_at SET NOT NULL;

ALTER TABLE neighborhood_metrics
  DROP CONSTRAINT IF EXISTS neighborhood_metrics_coverage_check,
  ADD CONSTRAINT neighborhood_metrics_coverage_check CHECK (coverage IN ('full', 'partial')),
  DROP CONSTRAINT IF EXISTS neighborhood_metrics_freshness_check,
  ADD CONSTRAINT neighborhood_metrics_freshness_check CHECK (freshness IN ('unknown', 'current', 'stale', 'expired')),
  DROP CONSTRAINT IF EXISTS neighborhood_metrics_quality_state_check,
  ADD CONSTRAINT neighborhood_metrics_quality_state_check CHECK (quality_state IN ('sufficient', 'low_confidence', 'insufficient_data'));

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'neighborhood_metrics_collection_run_fk') THEN
    ALTER TABLE neighborhood_metrics ADD CONSTRAINT neighborhood_metrics_collection_run_fk
      FOREIGN KEY (collection_run_id, neighborhood_id) REFERENCES collection_runs(id, neighborhood_id) ON DELETE CASCADE;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'neighborhood_metrics_inventory_run_fk') THEN
    ALTER TABLE neighborhood_metrics ADD CONSTRAINT neighborhood_metrics_inventory_run_fk
      FOREIGN KEY (inventory_collection_run_id, neighborhood_id) REFERENCES collection_runs(id, neighborhood_id)
      ON DELETE SET NULL (inventory_collection_run_id);
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'neighborhood_metrics_collection_run_unique') THEN
    ALTER TABLE neighborhood_metrics ADD CONSTRAINT neighborhood_metrics_collection_run_unique UNIQUE (collection_run_id);
  END IF;
END
$$;

CREATE INDEX IF NOT EXISTS idx_collection_runs_neighborhood_collected_at
  ON collection_runs(neighborhood_id, collected_at DESC);
CREATE INDEX IF NOT EXISTS idx_listing_observations_source_history
  ON listing_observations(collection_run_id, source_listing_id, captured_at DESC);
CREATE INDEX IF NOT EXISTS idx_transaction_observations_neighborhood_date
  ON transaction_observations(neighborhood_id, transaction_date DESC);

DROP TABLE IF EXISTS listing_snapshots;
DROP TABLE IF EXISTS raw_collection_records;
