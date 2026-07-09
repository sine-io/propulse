DROP INDEX IF EXISTS idx_listing_snapshots_neighborhood_run_captured_at;

ALTER TABLE listing_snapshots
  DROP COLUMN IF EXISTS collection_run_id;
