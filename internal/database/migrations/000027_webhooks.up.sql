CREATE TABLE webhooks (
    id          TEXT PRIMARY KEY,
    event_id    TEXT NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    url         TEXT NOT NULL,
    secret      TEXT NOT NULL,
    event_types TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    enabled     INTEGER NOT NULL DEFAULT 1,
    created_at  TEXT NOT NULL,
    updated_at  TEXT NOT NULL
);

CREATE INDEX idx_webhooks_event_id ON webhooks(event_id);

CREATE TABLE webhook_deliveries (
    id              TEXT PRIMARY KEY,
    webhook_id      TEXT NOT NULL REFERENCES webhooks(id) ON DELETE CASCADE,
    event_type      TEXT NOT NULL,
    payload         TEXT NOT NULL,
    response_status INTEGER,
    response_body   TEXT,
    error           TEXT,
    attempt         INTEGER NOT NULL DEFAULT 1,
    delivered_at    TEXT,
    created_at      TEXT NOT NULL
);

CREATE INDEX idx_webhook_deliveries_webhook_id ON webhook_deliveries(webhook_id);
CREATE INDEX idx_webhook_deliveries_created_at ON webhook_deliveries(webhook_id, created_at);
