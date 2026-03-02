-- Revert CHECK constraint on reminders.status to remove 'processing'.
-- Any rows with status='processing' are moved to 'scheduled' before migration.

UPDATE reminders SET status = 'scheduled' WHERE status = 'processing';

CREATE TABLE reminders_old (
    id           TEXT PRIMARY KEY,
    event_id     TEXT NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    remind_at    TEXT NOT NULL,
    target_group TEXT NOT NULL DEFAULT 'all' CHECK(target_group IN ('all','attending','maybe','declined','pending')),
    message      TEXT NOT NULL DEFAULT '',
    status       TEXT NOT NULL DEFAULT 'scheduled' CHECK(status IN ('scheduled','sent','cancelled','failed')),
    created_at   TEXT NOT NULL,
    updated_at   TEXT NOT NULL
);

INSERT INTO reminders_old SELECT * FROM reminders;
DROP TABLE reminders;
ALTER TABLE reminders_old RENAME TO reminders;

CREATE INDEX idx_reminders_event_id ON reminders(event_id);
CREATE INDEX idx_reminders_status ON reminders(status);
CREATE INDEX idx_reminders_remind_at ON reminders(remind_at);
