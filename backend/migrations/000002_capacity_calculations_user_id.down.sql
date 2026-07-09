DROP INDEX IF EXISTS idx_capacity_calculations_user_created_at;

ALTER TABLE capacity_calculations
  DROP COLUMN IF EXISTS user_id;
