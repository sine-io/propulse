ALTER TABLE transaction_observations
  ADD COLUMN IF NOT EXISTS attributes JSONB NOT NULL DEFAULT '{}'::jsonb;

ALTER TABLE community_market_snapshots
  ADD COLUMN IF NOT EXISTS collection_run_id UUID,
  ADD COLUMN IF NOT EXISTS analysis JSONB NOT NULL DEFAULT '{}'::jsonb,
  ADD COLUMN IF NOT EXISTS surroundings JSONB NOT NULL DEFAULT '{}'::jsonb,
  ADD COLUMN IF NOT EXISTS city_context JSONB NOT NULL DEFAULT '{}'::jsonb,
  ADD COLUMN IF NOT EXISTS quality_status TEXT NOT NULL DEFAULT 'aggregate_only';

ALTER TABLE community_market_snapshots
  DROP CONSTRAINT IF EXISTS community_market_snapshots_quality_status_check,
  ADD CONSTRAINT community_market_snapshots_quality_status_check
    CHECK (quality_status IN ('complete', 'aggregate_only'));

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname = 'community_market_snapshots_collection_run_fk'
  ) THEN
    ALTER TABLE community_market_snapshots
      ADD CONSTRAINT community_market_snapshots_collection_run_fk
      FOREIGN KEY (collection_run_id, neighborhood_id)
      REFERENCES collection_runs(id, neighborhood_id) ON DELETE CASCADE;
  END IF;
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname = 'community_market_snapshots_collection_run_unique'
  ) THEN
    ALTER TABLE community_market_snapshots
      ADD CONSTRAINT community_market_snapshots_collection_run_unique
      UNIQUE (collection_run_id);
  END IF;
END
$$;

CREATE TABLE listing_adjustments (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  collection_run_id UUID NOT NULL,
  neighborhood_id UUID NOT NULL,
  room_id TEXT NOT NULL CHECK (char_length(room_id) BETWEEN 1 AND 128),
  adjusted_at DATE NOT NULL,
  price_before_wan NUMERIC(12,4) NOT NULL CHECK (price_before_wan > 0),
  price_after_wan NUMERIC(12,4) NOT NULL CHECK (price_after_wan > 0),
  amount_wan NUMERIC(12,4) NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  FOREIGN KEY (collection_run_id, neighborhood_id)
    REFERENCES collection_runs(id, neighborhood_id) ON DELETE CASCADE,
  UNIQUE (collection_run_id, room_id, adjusted_at, price_before_wan, price_after_wan)
);

CREATE INDEX idx_listing_adjustments_run_room_date
  ON listing_adjustments(collection_run_id, room_id, adjusted_at DESC);

CREATE INDEX idx_community_market_snapshots_complete_latest
  ON community_market_snapshots(neighborhood_id, collected_at DESC)
  WHERE collection_run_id IS NOT NULL;
