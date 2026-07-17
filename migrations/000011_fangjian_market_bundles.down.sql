DROP INDEX IF EXISTS idx_community_market_snapshots_complete_latest;
DROP TABLE IF EXISTS listing_adjustments;

ALTER TABLE community_market_snapshots
  DROP CONSTRAINT IF EXISTS community_market_snapshots_collection_run_unique,
  DROP CONSTRAINT IF EXISTS community_market_snapshots_collection_run_fk,
  DROP CONSTRAINT IF EXISTS community_market_snapshots_quality_status_check,
  DROP COLUMN IF EXISTS quality_status,
  DROP COLUMN IF EXISTS city_context,
  DROP COLUMN IF EXISTS surroundings,
  DROP COLUMN IF EXISTS analysis,
  DROP COLUMN IF EXISTS collection_run_id;

ALTER TABLE transaction_observations
  DROP COLUMN IF EXISTS attributes;
