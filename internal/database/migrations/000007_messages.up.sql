CREATE TABLE messages (
    id             TEXT PRIMARY KEY,
    event_id       TEXT NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    sender_type    TEXT NOT NULL CHECK(sender_type IN ('organizer','attendee')),
    sender_id      TEXT NOT NULL,
    recipient_type TEXT NOT NULL CHECK(recipient_type IN ('organizer','attendee','group')),
    recipient_id   TEXT NOT NULL,
    subject        TEXT NOT NULL DEFAULT '',
    body           TEXT NOT NULL,
    read_at        TEXT,
    created_at     TEXT NOT NULL
);

CREATE INDEX idx_messages_event_id ON messages(event_id);
CREATE INDEX idx_messages_sender_id ON messages(sender_id);
CREATE INDEX idx_messages_recipient_id ON messages(recipient_id);
