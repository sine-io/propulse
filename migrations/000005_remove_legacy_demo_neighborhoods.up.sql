-- Remove only the two records created by the retired runtime demo seeder.
-- Exact identity, a known historical seeder owner, and absence of trusted imports
-- are all required so similarly named real neighborhoods remain untouched.
WITH legacy_demo_neighborhoods AS (
  SELECT n.id
  FROM neighborhoods AS n
  WHERE (
      (n.name = '青枫花园' AND n.area = '滨江核心' AND n.target_layout = '三房')
      OR
      (n.name = '云澜府' AND n.area = '城东新区' AND n.target_layout = '四房')
    )
    AND EXISTS (
      SELECT 1
      FROM watchlist_items AS w
      WHERE w.neighborhood_id = n.id
        AND (w.user_id = 'propulse-user' OR w.user_id = 'demo-user')
    )
    AND NOT EXISTS (
      SELECT 1
      FROM collection_runs AS cr
      WHERE cr.neighborhood_id = n.id
    )
)
DELETE FROM neighborhoods AS n
USING legacy_demo_neighborhoods AS legacy
WHERE n.id = legacy.id;
