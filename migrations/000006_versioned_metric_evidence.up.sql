ALTER TABLE neighborhood_metrics
  DROP CONSTRAINT neighborhood_metrics_collection_run_unique,
  ADD COLUMN algorithm_version TEXT,
  ADD COLUMN transaction_window_start DATE,
  ADD COLUMN transaction_window_end DATE,
  ADD COLUMN recent_30_day_transaction_count INT,
  ADD COLUMN preceding_60_day_transaction_count INT,
  ADD COLUMN recent_30_day_monthly_frequency NUMERIC(10,4),
  ADD COLUMN preceding_60_day_monthly_frequency NUMERIC(10,4);

UPDATE neighborhood_metrics
SET algorithm_version = 'legacy_unversioned';

ALTER TABLE neighborhood_metrics
  ALTER COLUMN algorithm_version SET NOT NULL;

ALTER TABLE neighborhood_metrics
  ADD CONSTRAINT neighborhood_metrics_algorithm_version_check
    CHECK (char_length(algorithm_version) BETWEEN 1 AND 128),
  ADD CONSTRAINT neighborhood_metrics_versioned_evidence_check
    CHECK (
      algorithm_version = 'legacy_unversioned'
      OR (
        transaction_window_start IS NOT NULL
        AND transaction_window_end IS NOT NULL
        AND transaction_window_end - transaction_window_start = 90
        AND recent_30_day_transaction_count IS NOT NULL
        AND recent_30_day_transaction_count >= 0
        AND preceding_60_day_transaction_count IS NOT NULL
        AND preceding_60_day_transaction_count >= 0
        AND transaction_sample_count = recent_30_day_transaction_count + preceding_60_day_transaction_count
        AND recent_30_day_monthly_frequency = recent_30_day_transaction_count::numeric
        AND preceding_60_day_monthly_frequency = preceding_60_day_transaction_count::numeric / 2
      )
    ),
  ADD CONSTRAINT neighborhood_metrics_collection_run_algorithm_unique
    UNIQUE (collection_run_id, algorithm_version);

CREATE INDEX idx_neighborhood_metrics_neighborhood_algorithm_calculated_at
  ON neighborhood_metrics(neighborhood_id, algorithm_version, calculated_at DESC);
