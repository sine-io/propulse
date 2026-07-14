DROP INDEX IF EXISTS idx_neighborhood_metrics_neighborhood_algorithm_calculated_at;

ALTER TABLE neighborhood_metrics
  DROP CONSTRAINT IF EXISTS neighborhood_metrics_collection_run_algorithm_unique,
  DROP CONSTRAINT IF EXISTS neighborhood_metrics_versioned_evidence_check,
  DROP CONSTRAINT IF EXISTS neighborhood_metrics_algorithm_version_check;

WITH ranked_metrics AS (
  SELECT
    id,
    row_number() OVER (
      PARTITION BY collection_run_id
      ORDER BY calculated_at DESC, algorithm_version DESC, id DESC
    ) AS rank
  FROM neighborhood_metrics
)
DELETE FROM neighborhood_metrics
WHERE id IN (SELECT id FROM ranked_metrics WHERE rank > 1);

ALTER TABLE neighborhood_metrics
  DROP COLUMN preceding_60_day_monthly_frequency,
  DROP COLUMN recent_30_day_monthly_frequency,
  DROP COLUMN preceding_60_day_transaction_count,
  DROP COLUMN recent_30_day_transaction_count,
  DROP COLUMN transaction_window_end,
  DROP COLUMN transaction_window_start,
  DROP COLUMN algorithm_version,
  ADD CONSTRAINT neighborhood_metrics_collection_run_unique UNIQUE (collection_run_id);
