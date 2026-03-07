CREATE TABLE event_comments (
    id          TEXT PRIMARY KEY,
    event_id    TEXT NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    attendee_id TEXT NOT NULL REFERENCES attendees(id) ON DELETE CASCADE,
    author_name TEXT NOT NULL,
    body        TEXT NOT NULL,
    created_at  TEXT NOT NULL
);

CREATE INDEX idx_event_comments_event_id ON event_comments(event_id);
CREATE INDEX idx_event_comments_attendee_id ON event_comments(attendee_id);
CREATE INDEX idx_event_comments_created_at ON event_comments(event_id, created_at);
