CREATE TABLE event_cohosts (
    id           TEXT PRIMARY KEY,
    event_id     TEXT NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    organizer_id TEXT NOT NULL REFERENCES organizers(id) ON DELETE CASCADE,
    role         TEXT NOT NULL DEFAULT 'cohost' CHECK(role IN ('cohost')),
    added_by     TEXT NOT NULL REFERENCES organizers(id),
    created_at   TEXT NOT NULL
);

CREATE UNIQUE INDEX idx_event_cohosts_event_organizer ON event_cohosts(event_id, organizer_id);
CREATE INDEX idx_event_cohosts_organizer_id ON event_cohosts(organizer_id);
