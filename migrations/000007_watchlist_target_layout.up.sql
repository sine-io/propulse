ALTER TABLE neighborhoods
  ADD COLUMN city TEXT,
  ADD CONSTRAINT neighborhoods_city_check
    CHECK (city IS NULL OR (char_length(btrim(city)) BETWEEN 1 AND 128));

WITH trusted_city AS (
  SELECT
    cr.neighborhood_id,
    min(btrim(ds.city)) AS city
  FROM collection_runs cr
  JOIN data_sources ds ON ds.id = cr.data_source_id
  WHERE btrim(ds.city) <> ''
  GROUP BY cr.neighborhood_id
  HAVING count(DISTINCT btrim(ds.city)) = 1
)
UPDATE neighborhoods n
SET city = trusted_city.city
FROM trusted_city
WHERE trusted_city.neighborhood_id = n.id;

CREATE TABLE neighborhood_layouts (
  neighborhood_id UUID NOT NULL REFERENCES neighborhoods(id) ON DELETE CASCADE,
  layout TEXT NOT NULL CHECK (char_length(btrim(layout)) BETWEEN 1 AND 64),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (neighborhood_id, layout)
);

INSERT INTO neighborhood_layouts (neighborhood_id, layout)
SELECT neighborhood_id, layout
FROM (
  SELECT id AS neighborhood_id, btrim(target_layout) AS layout
  FROM neighborhoods
  WHERE btrim(target_layout) <> ''
  UNION
  SELECT neighborhood_id, btrim(layout)
  FROM listing_observations
  WHERE btrim(layout) <> ''
  UNION
  SELECT neighborhood_id, btrim(layout)
  FROM transaction_observations
  WHERE btrim(layout) <> ''
) catalog
ON CONFLICT (neighborhood_id, layout) DO NOTHING;

ALTER TABLE watchlist_items
  ADD COLUMN target_layout TEXT;

UPDATE watchlist_items wi
SET target_layout = btrim(n.target_layout)
FROM neighborhoods n
WHERE n.id = wi.neighborhood_id
  AND btrim(n.target_layout) <> '';

ALTER TABLE watchlist_items
  ALTER COLUMN target_layout SET NOT NULL,
  ADD CONSTRAINT watchlist_items_target_layout_check
    CHECK (char_length(btrim(target_layout)) BETWEEN 1 AND 64),
  ADD CONSTRAINT watchlist_items_target_layout_fk
    FOREIGN KEY (neighborhood_id, target_layout)
    REFERENCES neighborhood_layouts(neighborhood_id, layout) ON DELETE RESTRICT;

ALTER TABLE neighborhood_metrics
  ADD COLUMN target_layout_supply_by_layout JSONB NOT NULL DEFAULT '{}'::jsonb,
  ADD CONSTRAINT neighborhood_metrics_layout_supply_object_check
    CHECK (jsonb_typeof(target_layout_supply_by_layout) = 'object');

UPDATE neighborhood_metrics nm
SET target_layout_supply_by_layout = jsonb_build_object(
  btrim(n.target_layout),
  nm.target_layout_supply
)
FROM neighborhoods n
WHERE n.id = nm.neighborhood_id
  AND btrim(n.target_layout) <> '';

ALTER TABLE neighborhood_metrics
  DROP COLUMN target_layout_supply;

ALTER TABLE neighborhoods
  DROP COLUMN target_layout;

CREATE INDEX idx_neighborhoods_city_area_name_id
  ON neighborhoods(city, area, name, id);

CREATE INDEX idx_neighborhood_layouts_layout_neighborhood
  ON neighborhood_layouts(layout, neighborhood_id);
