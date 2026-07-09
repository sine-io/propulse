ALTER TABLE capacity_calculations
  ADD COLUMN IF NOT EXISTS user_id TEXT NOT NULL DEFAULT 'demo-user';

CREATE INDEX IF NOT EXISTS idx_capacity_calculations_user_created_at
  ON capacity_calculations(user_id, created_at DESC);
