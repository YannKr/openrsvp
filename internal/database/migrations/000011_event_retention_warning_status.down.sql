-- Revert to original CHECK constraint without retention_warning.

CREATE TABLE events_old (
    id             TEXT PRIMARY KEY,
    organizer_id   TEXT NOT NULL REFERENCES organizers(id),
    title          TEXT NOT NULL,
    description    TEXT NOT NULL DEFAULT '',
    event_date     TEXT NOT NULL,
    end_date       TEXT,
    location       TEXT NOT NULL DEFAULT '',
    timezone       TEXT NOT NULL DEFAULT 'America/New_York',
    retention_days INTEGER NOT NULL DEFAULT 30,
    status         TEXT NOT NULL DEFAULT 'draft' CHECK(status IN ('draft','published','cancelled','archived')),
    share_token    TEXT NOT NULL UNIQUE,
    created_at     TEXT NOT NULL,
    updated_at     TEXT NOT NULL
);

INSERT INTO events_old SELECT * FROM events WHERE status != 'retention_warning';
UPDATE events SET status = 'published' WHERE status = 'retention_warning';
INSERT INTO events_old SELECT * FROM events WHERE status = 'published' AND id NOT IN (SELECT id FROM events_old);
DROP TABLE events;
ALTER TABLE events_old RENAME TO events;

CREATE INDEX idx_events_organizer_id ON events(organizer_id);
CREATE INDEX idx_events_share_token ON events(share_token);
CREATE INDEX idx_events_status ON events(status);
CREATE INDEX idx_events_event_date ON events(event_date);
