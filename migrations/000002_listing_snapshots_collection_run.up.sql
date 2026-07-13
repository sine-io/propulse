DO $$
BEGIN
  IF to_regclass('public.listing_snapshots') IS NOT NULL THEN
    ALTER TABLE listing_snapshots
      ADD COLUMN IF NOT EXISTS collection_run_id UUID REFERENCES raw_collection_records(id) ON DELETE CASCADE;

    CREATE INDEX IF NOT EXISTS idx_listing_snapshots_neighborhood_run_captured_at
      ON listing_snapshots(neighborhood_id, collection_run_id, captured_at DESC);
  END IF;
END
$$;
