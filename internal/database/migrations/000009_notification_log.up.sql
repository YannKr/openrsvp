CREATE TABLE notification_log (
    id          TEXT PRIMARY KEY,
    event_id    TEXT NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    attendee_id TEXT REFERENCES attendees(id) ON DELETE SET NULL,
    channel     TEXT NOT NULL CHECK(channel IN ('email','sms')),
    provider    TEXT NOT NULL,
    status      TEXT NOT NULL CHECK(status IN ('pending','sent','failed')),
    error       TEXT,
    sent_at     TEXT,
    created_at  TEXT NOT NULL
);

CREATE INDEX idx_notification_log_event_id ON notification_log(event_id);
CREATE INDEX idx_notification_log_attendee_id ON notification_log(attendee_id);
CREATE INDEX idx_notification_log_status ON notification_log(status);
