CREATE TABLE user_property_assets (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id TEXT NOT NULL,
  name TEXT NOT NULL CHECK (char_length(btrim(name)) BETWEEN 1 AND 128),
  neighborhood_id UUID NOT NULL REFERENCES neighborhoods(id) ON DELETE RESTRICT,
  neighborhood_name TEXT NOT NULL CHECK (char_length(btrim(neighborhood_name)) BETWEEN 1 AND 256),
  city TEXT NOT NULL CHECK (char_length(btrim(city)) BETWEEN 1 AND 128),
  district TEXT NOT NULL CHECK (char_length(btrim(district)) BETWEEN 1 AND 128),
  layout TEXT NOT NULL CHECK (char_length(btrim(layout)) BETWEEN 1 AND 64),
  area_sqm NUMERIC(8,2) NOT NULL CHECK (area_sqm > 0 AND area_sqm <= 10000),
  floor_band TEXT NOT NULL DEFAULT '' CHECK (char_length(floor_band) <= 64),
  floor_description TEXT NOT NULL DEFAULT '' CHECK (char_length(floor_description) <= 128),
  orientation TEXT NOT NULL DEFAULT '' CHECK (char_length(orientation) <= 128),
  current_listing_price_wan NUMERIC(12,2) CHECK (current_listing_price_wan > 0),
  original_purchase_price_wan NUMERIC(12,2) NOT NULL CHECK (original_purchase_price_wan > 0),
  purchased_on DATE NOT NULL CHECK (purchased_on >= DATE '1900-01-01'),
  current_loan_balance_wan NUMERIC(12,2) NOT NULL CHECK (current_loan_balance_wan >= 0),
  source_kind TEXT NOT NULL CHECK (source_kind IN ('manual', 'market_listing')),
  source_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ,
  CHECK (
    (source_kind = 'manual' AND source_snapshot = '{}'::jsonb) OR
    (source_kind = 'market_listing' AND current_listing_price_wan IS NOT NULL AND source_snapshot <> '{}'::jsonb)
  )
);

CREATE INDEX idx_user_property_assets_user_updated
  ON user_property_assets(user_id, updated_at DESC, id DESC)
  WHERE deleted_at IS NULL;

CREATE INDEX idx_user_property_assets_neighborhood
  ON user_property_assets(neighborhood_id)
  WHERE deleted_at IS NULL;

ALTER TABLE capacity_calculations
  ADD COLUMN selection_context JSONB;
