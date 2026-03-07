ALTER TABLE notification_log ADD COLUMN recipient TEXT NOT NULL DEFAULT '';
ALTER TABLE notification_log ADD COLUMN subject TEXT NOT NULL DEFAULT '';
ALTER TABLE notification_log ADD COLUMN message_id TEXT;
ALTER TABLE notification_log ADD COLUMN delivery_status TEXT NOT NULL DEFAULT 'unknown';
ALTER TABLE notification_log ADD COLUMN delivered_at TEXT;
ALTER TABLE notification_log ADD COLUMN opened_at TEXT;
ALTER TABLE notification_log ADD COLUMN clicked_at TEXT;
ALTER TABLE notification_log ADD COLUMN bounced_at TEXT;
ALTER TABLE notification_log ADD COLUMN bounce_type TEXT;
ALTER TABLE notification_log ADD COLUMN complaint_at TEXT;

CREATE INDEX idx_notification_log_message_id ON notification_log(message_id);
CREATE INDEX idx_notification_log_recipient ON notification_log(recipient);
