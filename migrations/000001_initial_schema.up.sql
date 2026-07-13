CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE capacity_calculations (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id TEXT NOT NULL DEFAULT 'propulse-user',
  input JSONB NOT NULL,
  result JSONB NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_capacity_calculations_user_created_at
  ON capacity_calculations(user_id, created_at DESC);

CREATE TABLE neighborhoods (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name TEXT NOT NULL,
  area TEXT NOT NULL DEFAULT '',
  target_layout TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE watchlist_items (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  neighborhood_id UUID NOT NULL REFERENCES neighborhoods(id) ON DELETE CASCADE,
  user_id TEXT NOT NULL DEFAULT 'propulse-user',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (user_id, neighborhood_id)
);

CREATE TABLE data_sources (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name TEXT NOT NULL CHECK (char_length(name) BETWEEN 1 AND 128),
  source_type TEXT NOT NULL CHECK (source_type ~ '^[a-z][a-z0-9_]{0,63}$'),
  city TEXT NOT NULL CHECK (char_length(city) BETWEEN 1 AND 128),
  notes TEXT NOT NULL DEFAULT '' CHECK (char_length(notes) <= 2048),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (name, city)
);

CREATE TABLE collection_runs (
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
  metric_status TEXT NOT NULL DEFAULT 'pending'
    CHECK (metric_status IN ('pending', 'completed', 'failed')),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (data_source_id, source_ref, content_checksum),
  UNIQUE (id, neighborhood_id)
);

CREATE TABLE listing_observations (
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
  FOREIGN KEY (collection_run_id, neighborhood_id)
    REFERENCES collection_runs(id, neighborhood_id) ON DELETE CASCADE,
  UNIQUE (collection_run_id, source_listing_id)
);

CREATE TABLE transaction_observations (
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
  FOREIGN KEY (collection_run_id, neighborhood_id)
    REFERENCES collection_runs(id, neighborhood_id) ON DELETE CASCADE,
  UNIQUE (collection_run_id, source_record_id)
);

CREATE TABLE raw_collection_records (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  source_type TEXT NOT NULL,
  source_ref TEXT NOT NULL,
  payload JSONB NOT NULL,
  collected_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE listing_snapshots (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  collection_run_id UUID NOT NULL REFERENCES raw_collection_records(id) ON DELETE CASCADE,
  neighborhood_id UUID NOT NULL REFERENCES neighborhoods(id) ON DELETE CASCADE,
  listing_price NUMERIC(12,2) NOT NULL,
  transaction_price NUMERIC(12,2),
  price_cut BOOLEAN NOT NULL DEFAULT false,
  days_on_market INT NOT NULL DEFAULT 0,
  layout TEXT NOT NULL DEFAULT '',
  captured_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE neighborhood_metrics (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  neighborhood_id UUID NOT NULL REFERENCES neighborhoods(id) ON DELETE CASCADE,
  listed_homes INT NOT NULL,
  price_cut_homes INT NOT NULL,
  avg_days_on_market NUMERIC(8,2),
  listing_price_min NUMERIC(12,2),
  listing_price_max NUMERIC(12,2),
  transaction_price_min NUMERIC(12,2),
  transaction_price_max NUMERIC(12,2),
  transaction_momentum TEXT NOT NULL,
  target_layout_supply INT NOT NULL,
  calculated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  collection_run_id UUID,
  inventory_collection_run_id UUID,
  source_ids JSONB NOT NULL DEFAULT '[]'::jsonb,
  listing_sample_count INT NOT NULL DEFAULT 0 CHECK (listing_sample_count >= 0),
  transaction_sample_count INT NOT NULL DEFAULT 0 CHECK (transaction_sample_count >= 0),
  listed_homes_change_pct NUMERIC(8,2),
  coverage TEXT CHECK (coverage IN ('full', 'partial')),
  freshness TEXT CHECK (freshness IN ('unknown', 'current', 'stale', 'expired')),
  quality_state TEXT CHECK (quality_state IN ('sufficient', 'low_confidence', 'insufficient_data')),
  latest_observed_at TIMESTAMPTZ,
  inventory_collected_at TIMESTAMPTZ,
  quality_warnings JSONB NOT NULL DEFAULT '[]'::jsonb,
  FOREIGN KEY (collection_run_id, neighborhood_id)
    REFERENCES collection_runs(id, neighborhood_id) ON DELETE CASCADE,
  FOREIGN KEY (inventory_collection_run_id, neighborhood_id)
    REFERENCES collection_runs(id, neighborhood_id)
    ON DELETE SET NULL (inventory_collection_run_id),
  UNIQUE (collection_run_id)
);

CREATE INDEX idx_listing_snapshots_neighborhood_captured_at
  ON listing_snapshots(neighborhood_id, captured_at DESC);

CREATE INDEX idx_listing_snapshots_neighborhood_run_captured_at
  ON listing_snapshots(neighborhood_id, collection_run_id, captured_at DESC);

CREATE INDEX idx_collection_runs_neighborhood_collected_at
  ON collection_runs(neighborhood_id, collected_at DESC);

CREATE INDEX idx_listing_observations_source_history
  ON listing_observations(collection_run_id, source_listing_id, captured_at DESC);

CREATE INDEX idx_transaction_observations_neighborhood_date
  ON transaction_observations(neighborhood_id, transaction_date DESC);

CREATE INDEX idx_neighborhood_metrics_neighborhood_calculated_at
  ON neighborhood_metrics(neighborhood_id, calculated_at DESC);
