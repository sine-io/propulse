DROP INDEX idx_community_market_snapshots_neighborhood_collected_at;

CREATE INDEX idx_community_market_snapshots_neighborhood_collected_at
  ON community_market_snapshots(neighborhood_id, collected_at DESC, id DESC);

ALTER TABLE community_market_snapshots
  DROP COLUMN man_car_separation,
  DROP COLUMN gas_cost,
  DROP COLUMN electricity_type,
  DROP COLUMN water_type,
  DROP COLUMN heating_type,
  DROP COLUMN parking_fee,
  DROP COLUMN parking_ratio,
  DROP COLUMN fixed_parking_spaces,
  DROP COLUMN property_fee,
  DROP COLUMN property_management_company,
  DROP COLUMN greening_rate_percent,
  DROP COLUMN green_area_sqm,
  DROP COLUMN plot_ratio,
  DROP COLUMN closed_management,
  DROP COLUMN household_count,
  DROP COLUMN developer,
  DROP COLUMN building_year,
  DROP COLUMN building_type,
  DROP COLUMN building_count,
  DROP COLUMN property_tags,
  DROP COLUMN property_type,
  DROP COLUMN province_name,
  DROP COLUMN province_code;
