ALTER TABLE events ADD COLUMN series_id TEXT REFERENCES event_series(id) ON DELETE SET NULL;
ALTER TABLE events ADD COLUMN series_index INTEGER;
ALTER TABLE events ADD COLUMN series_override INTEGER NOT NULL DEFAULT 0;

CREATE INDEX idx_events_series_id ON events(series_id);
