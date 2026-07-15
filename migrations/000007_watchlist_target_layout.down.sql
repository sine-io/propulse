DROP INDEX IF EXISTS idx_neighborhood_layouts_layout_neighborhood;
DROP INDEX IF EXISTS idx_neighborhoods_city_area_name_id;

ALTER TABLE neighborhoods
  ADD COLUMN target_layout TEXT NOT NULL DEFAULT '';

UPDATE neighborhoods n
SET target_layout = COALESCE(
  (
    SELECT wi.target_layout
    FROM watchlist_items wi
    WHERE wi.neighborhood_id = n.id
    ORDER BY wi.created_at ASC, wi.id ASC
    LIMIT 1
  ),
  (
    SELECT nl.layout
    FROM neighborhood_layouts nl
    WHERE nl.neighborhood_id = n.id
    ORDER BY nl.layout ASC
    LIMIT 1
  ),
  ''
);

ALTER TABLE neighborhood_metrics
  ADD COLUMN target_layout_supply INT NOT NULL DEFAULT 0;

UPDATE neighborhood_metrics nm
SET target_layout_supply = COALESCE(
  (nm.target_layout_supply_by_layout ->> n.target_layout)::int,
  0
)
FROM neighborhoods n
WHERE n.id = nm.neighborhood_id;

ALTER TABLE neighborhood_metrics
  DROP CONSTRAINT neighborhood_metrics_layout_supply_object_check,
  DROP COLUMN target_layout_supply_by_layout;

ALTER TABLE watchlist_items
  DROP CONSTRAINT watchlist_items_target_layout_fk,
  DROP CONSTRAINT watchlist_items_target_layout_check,
  DROP COLUMN target_layout;

DROP TABLE neighborhood_layouts;

ALTER TABLE neighborhoods
  DROP CONSTRAINT neighborhoods_city_check,
  DROP COLUMN city;
