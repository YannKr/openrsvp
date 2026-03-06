DROP INDEX IF EXISTS idx_events_series_id;

ALTER TABLE events DROP COLUMN series_id;
ALTER TABLE events DROP COLUMN series_index;
ALTER TABLE events DROP COLUMN series_override;
