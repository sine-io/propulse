-- name: AggregateListingSnapshots :one
SELECT
  COUNT(*)::int AS listed_homes,
  COUNT(*) FILTER (WHERE price_cut)::int AS price_cut_homes,
  COALESCE(AVG(days_on_market), 0)::numeric AS avg_days_on_market,
  COALESCE(MIN(listing_price), 0)::numeric AS listing_price_min,
  COALESCE(MAX(listing_price), 0)::numeric AS listing_price_max,
  COALESCE(MIN(transaction_price), 0)::numeric AS transaction_price_min,
  COALESCE(MAX(transaction_price), 0)::numeric AS transaction_price_max,
  COUNT(*) FILTER (WHERE layout = sqlc.arg(target_layout))::int AS target_layout_supply
FROM listing_snapshots
WHERE neighborhood_id = sqlc.arg(neighborhood_id)
  AND collection_run_id = (
    SELECT collection_run_id
    FROM listing_snapshots
    WHERE neighborhood_id = sqlc.arg(neighborhood_id)
      AND collection_run_id IS NOT NULL
    GROUP BY collection_run_id
    ORDER BY MAX(captured_at) DESC, collection_run_id DESC
    LIMIT 1
  );

-- name: InsertNeighborhoodMetric :one
INSERT INTO neighborhood_metrics (
  neighborhood_id,
  listed_homes,
  price_cut_homes,
  avg_days_on_market,
  listing_price_min,
  listing_price_max,
  transaction_price_min,
  transaction_price_max,
  transaction_momentum,
  target_layout_supply
) VALUES (
  $1,$2,$3,$4,$5,$6,$7,$8,$9,$10
)
RETURNING *;

-- name: LatestNeighborhoodMetric :one
SELECT *
FROM neighborhood_metrics
WHERE neighborhood_id = $1
ORDER BY calculated_at DESC
LIMIT 1;
