CREATE TABLE invite_cards (
    id              TEXT PRIMARY KEY,
    event_id        TEXT NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    template_id     TEXT NOT NULL DEFAULT 'balloon-party',
    heading         TEXT NOT NULL DEFAULT '',
    body            TEXT NOT NULL DEFAULT '',
    footer          TEXT NOT NULL DEFAULT '',
    primary_color   TEXT NOT NULL DEFAULT '#6366f1',
    secondary_color TEXT NOT NULL DEFAULT '#f0abfc',
    font            TEXT NOT NULL DEFAULT 'Inter',
    custom_data     TEXT NOT NULL DEFAULT '{}',
    created_at      TEXT NOT NULL,
    updated_at      TEXT NOT NULL,
    UNIQUE(event_id)
);

CREATE INDEX idx_invite_cards_event_id ON invite_cards(event_id);
