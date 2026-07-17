ALTER TABLE community_market_snapshots
  ADD COLUMN province_code TEXT CHECK (province_code IS NULL OR char_length(btrim(province_code)) BETWEEN 1 AND 32),
  ADD COLUMN province_name TEXT CHECK (province_name IS NULL OR char_length(btrim(province_name)) BETWEEN 1 AND 128),
  ADD COLUMN property_type TEXT CHECK (property_type IS NULL OR char_length(btrim(property_type)) BETWEEN 1 AND 128),
  ADD COLUMN property_tags JSONB CHECK (property_tags IS NULL OR (jsonb_typeof(property_tags) = 'array' AND jsonb_array_length(property_tags) <= 20)),
  ADD COLUMN building_count INT CHECK (building_count IS NULL OR building_count >= 0),
  ADD COLUMN building_type TEXT CHECK (building_type IS NULL OR char_length(btrim(building_type)) BETWEEN 1 AND 128),
  ADD COLUMN building_year INT CHECK (building_year IS NULL OR building_year BETWEEN 1800 AND 3000),
  ADD COLUMN developer TEXT CHECK (developer IS NULL OR char_length(btrim(developer)) BETWEEN 1 AND 256),
  ADD COLUMN household_count INT CHECK (household_count IS NULL OR household_count >= 0),
  ADD COLUMN closed_management TEXT CHECK (closed_management IS NULL OR closed_management IN ('是', '否')),
  ADD COLUMN plot_ratio NUMERIC(8,4) CHECK (plot_ratio IS NULL OR plot_ratio BETWEEN 0 AND 100),
  ADD COLUMN green_area_sqm NUMERIC(14,2) CHECK (green_area_sqm IS NULL OR green_area_sqm >= 0),
  ADD COLUMN greening_rate_percent NUMERIC(7,4) CHECK (greening_rate_percent IS NULL OR greening_rate_percent BETWEEN 0 AND 100),
  ADD COLUMN property_management_company TEXT CHECK (property_management_company IS NULL OR char_length(btrim(property_management_company)) BETWEEN 1 AND 256),
  ADD COLUMN property_fee TEXT CHECK (property_fee IS NULL OR char_length(btrim(property_fee)) BETWEEN 1 AND 128),
  ADD COLUMN fixed_parking_spaces INT CHECK (fixed_parking_spaces IS NULL OR fixed_parking_spaces >= 0),
  ADD COLUMN parking_ratio TEXT CHECK (parking_ratio IS NULL OR char_length(btrim(parking_ratio)) BETWEEN 1 AND 64),
  ADD COLUMN parking_fee TEXT CHECK (parking_fee IS NULL OR char_length(btrim(parking_fee)) BETWEEN 1 AND 128),
  ADD COLUMN heating_type TEXT CHECK (heating_type IS NULL OR char_length(btrim(heating_type)) BETWEEN 1 AND 128),
  ADD COLUMN water_type TEXT CHECK (water_type IS NULL OR char_length(btrim(water_type)) BETWEEN 1 AND 128),
  ADD COLUMN electricity_type TEXT CHECK (electricity_type IS NULL OR char_length(btrim(electricity_type)) BETWEEN 1 AND 128),
  ADD COLUMN gas_cost TEXT CHECK (gas_cost IS NULL OR char_length(btrim(gas_cost)) BETWEEN 1 AND 128),
  ADD COLUMN man_car_separation TEXT CHECK (man_car_separation IS NULL OR man_car_separation IN ('是', '否'));

DROP INDEX idx_community_market_snapshots_neighborhood_collected_at;

CREATE INDEX idx_community_market_snapshots_neighborhood_collected_at
  ON community_market_snapshots(neighborhood_id, collected_at DESC, created_at DESC, id DESC);
