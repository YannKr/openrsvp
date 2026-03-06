CREATE TABLE event_series (
    id               TEXT PRIMARY KEY,
    organizer_id     TEXT NOT NULL REFERENCES organizers(id),
    title            TEXT NOT NULL,
    description      TEXT NOT NULL DEFAULT '',
    location         TEXT NOT NULL DEFAULT '',
    timezone         TEXT NOT NULL DEFAULT 'America/New_York',
    event_time       TEXT NOT NULL,
    duration_minutes INTEGER,
    recurrence_rule  TEXT NOT NULL CHECK(recurrence_rule IN ('weekly','biweekly','monthly')),
    recurrence_end   TEXT,
    max_occurrences  INTEGER,
    series_status    TEXT NOT NULL DEFAULT 'active' CHECK(series_status IN ('active','stopped')),
    retention_days   INTEGER NOT NULL DEFAULT 30,
    contact_requirement TEXT NOT NULL DEFAULT 'email',
    show_headcount   INTEGER NOT NULL DEFAULT 0,
    show_guest_list  INTEGER NOT NULL DEFAULT 0,
    rsvp_deadline_offset_hours INTEGER,
    max_capacity     INTEGER,
    created_at       TEXT NOT NULL,
    updated_at       TEXT NOT NULL
);

CREATE INDEX idx_event_series_organizer_id ON event_series(organizer_id);
CREATE INDEX idx_event_series_status ON event_series(series_status);
