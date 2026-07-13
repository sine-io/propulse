DROP TABLE IF EXISTS neighborhood_metrics;
DROP TABLE IF EXISTS transaction_observations;
DROP TABLE IF EXISTS listing_observations;
DROP TABLE IF EXISTS collection_runs;
DROP TABLE IF EXISTS data_sources;
DROP TABLE IF EXISTS watchlist_items;
-- These tables are absent on a fresh install, but may have been recreated while
-- rolling an upgraded database back through migrations 3 and 2.
DROP TABLE IF EXISTS listing_snapshots;
DROP TABLE IF EXISTS raw_collection_records;
DROP TABLE IF EXISTS neighborhoods;
DROP TABLE IF EXISTS capacity_calculations;
