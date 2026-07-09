CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE capacity_calculations (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id TEXT NOT NULL DEFAULT 'demo-user',
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
  user_id TEXT NOT NULL DEFAULT 'demo-user',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (user_id, neighborhood_id)
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
  avg_days_on_market NUMERIC(8,2) NOT NULL,
  listing_price_min NUMERIC(12,2) NOT NULL,
  listing_price_max NUMERIC(12,2) NOT NULL,
  transaction_price_min NUMERIC(12,2) NOT NULL,
  transaction_price_max NUMERIC(12,2) NOT NULL,
  transaction_momentum TEXT NOT NULL,
  target_layout_supply INT NOT NULL,
  calculated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_listing_snapshots_neighborhood_captured_at
  ON listing_snapshots(neighborhood_id, captured_at DESC);

CREATE INDEX idx_neighborhood_metrics_neighborhood_calculated_at
  ON neighborhood_metrics(neighborhood_id, calculated_at DESC);
